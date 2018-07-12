package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/vault/api"
	"github.com/pivotal-cf/brokerapi"
)

const (
	// DefaultListenAddr is the default address unless we get
	// an override via PORT
	DefaultListenAddr = ":8000"

	// DefaultServiceID is the default UUID of the services
	DefaultServiceID = "0654695e-0760-a1d4-1cad-5dd87b75ed99"

	// DefaultVaultAddr is the default address to the Vault cluster.
	DefaultVaultAddr = "https://127.0.0.1:8200"

	// DefaultServiceName is the name of the service in the marketplace
	DefaultServiceName = "hashicorp-vault"

	// DefaultServiceDescription is the default service description.
	DefaultServiceDescription = "HashiCorp Vault Service Broker"

	// DefaultPlanName is the name of our plan, only one supported
	DefaultPlanName = "shared"

	// DefaultPlanDescription is the default description.
	DefaultPlanDescription = "Secure access to Vault's storage and transit backends"

	//DefaultShareable is the default configuration for the service metadata.
	DefaultShareable = true
)

func main() {
	// Setup the logger - intentionally do not log date or time because it will
	// be prefixed in the log output by CF.
	log := log.New(os.Stdout, "", 0)

	// Ensure username and password are present
	username := os.Getenv("SECURITY_USER_NAME")
	if username == "" {
		log.Fatal("[ERR] missing SECURITY_USER_NAME")
	}
	password := os.Getenv("SECURITY_USER_PASSWORD")
	if password == "" {
		log.Fatal("[ERR] missing SECURITY_USER_PASSWORD")
	}

	// Get a custom Shareable
	shareable, parseErr := strconv.ParseBool(os.Getenv("SHAREABLE"))
	if parseErr != nil {
		shareable = DefaultShareable
	}
	// Get a custom GUID
	serviceID := os.Getenv("SERVICE_ID")
	if serviceID == "" {
		serviceID = DefaultServiceID
	}

	// Get the service name
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = DefaultServiceName
	}

	// Get the service description
	serviceDescription := os.Getenv("SERVICE_DESCRIPTION")
	if serviceDescription == "" {
		serviceDescription = DefaultServiceDescription
	}

	// Get the service tags
	serviceTags := strings.Split(os.Getenv("SERVICE_TAGS"), ",")

	// Get the plan name
	planName := os.Getenv("PLAN_NAME")
	if planName == "" {
		planName = DefaultPlanName
	}

	// Get the plan description
	planDescription := os.Getenv("PLAN_DESCRIPTION")
	if planDescription == "" {
		planDescription = DefaultPlanDescription
	}

	// Parse the port
	port := os.Getenv("PORT")
	if port == "" {
		port = DefaultListenAddr
	} else {
		if port[0] != ':' {
			port = ":" + port
		}
	}

	// Check for vault address
	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		vaultAddr = "https://127.0.0.1:8200"
	}
	os.Setenv("VAULT_ADDR", normalizeAddr(vaultAddr))

	// Get the vault advertise addr
	vaultAdvertiseAddr := os.Getenv("VAULT_ADVERTISE_ADDR")
	if vaultAdvertiseAddr == "" {
		vaultAdvertiseAddr = normalizeAddr(vaultAddr)
	}

	// Check if renewal is enabled
	renew := true
	if s := os.Getenv("VAULT_RENEW"); s != "" {
		b, err := strconv.ParseBool(s)
		if err != nil {
			log.Fatalf("[ERR] failed to parse VAULT_RENEW: %s", err)
		}
		renew = b
	}

	// Check for vault token
	if v := os.Getenv("VAULT_TOKEN"); v == "" {
		log.Fatal("[ERR] missing VAULT_TOKEN")
	}

	// Setup the vault client
	client, err := api.NewClient(nil)
	if err != nil {
		log.Fatal("[ERR] failed to create api client", err)
	}

	// Setup the broker
	broker := &Broker{
		log:    log,
		client: client,

		serviceID:          serviceID,
		serviceName:        serviceName,
		serviceDescription: serviceDescription,
		serviceTags:        serviceTags,

		planName:        planName,
		planDescription: planDescription,

		vaultAdvertiseAddr: vaultAdvertiseAddr,
		vaultRenewToken:    renew,

		Metadata: &brokerapi.ServiceMetadata{Shareable: &shareable},
	}
	if err := broker.Start(); err != nil {
		log.Fatalf("[ERR] failed to start broker: %s", err)
	}

	// Parse the broker credentials
	creds := brokerapi.BrokerCredentials{
		Username: username,
		Password: password,
	}

	// Setup the HTTP handler
	handler := brokerapi.New(broker, lager.NewLogger("vault-broker"), creds)

	// Listen to incoming connection
	serverCh := make(chan struct{}, 1)
	go func() {
		log.Printf("[INFO] starting server on %s", port)
		if err := http.ListenAndServe(port, handler); err != nil {
			log.Fatalf("[ERR] server exited with: %s", err)
		}
		close(serverCh)
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case <-serverCh:
	case s := <-signalCh:
		log.Printf("[INFO] received signal %s", s)
	}

	if err := broker.Stop(); err != nil {
		log.Fatalf("[ERR] faild to stop broker: %s", err)
	}

	os.Exit(0)
}

// normalizeAddr takes a string that represents a URL and ensures it has a
// scheme (defaulting to https), and ensures the path ends in a trailing slash.
func normalizeAddr(s string) string {
	if s == "" {
		return s
	}

	u, err := url.Parse(s)
	if err != nil {
		return s
	}

	if u.Scheme == "" {
		u.Scheme = "https"
	}

	if strings.Contains(u.Scheme, ".") {
		u.Host = u.Scheme
		if u.Opaque != "" {
			u.Host = u.Host + ":" + u.Opaque
			u.Opaque = ""
		}
		u.Scheme = "https"
	}

	if u.Host == "" {
		split := strings.SplitN(u.Path, "/", 2)
		switch len(split) {
		case 0:
		case 1:
			u.Host = split[0]
			u.Path = "/"
		case 2:
			u.Host = split[0]
			u.Path = split[1]
		}
	}

	u.Path = strings.TrimRight(u.Path, "/") + "/"

	return u.String()
}
