package main

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/vault/api"
	"github.com/kelseyhightower/envconfig"
	"github.com/pivotal-cf/brokerapi"
)

func main() {
	// Setup the logger - intentionally do not log date or time because it will
	// be prefixed in the log output by CF.
	logger := log.New(os.Stdout, "", 0)

	config, err := parseConfig()
	if err != nil {
		logger.Fatal("[ERR] failed to read configuration", err)
	}

	// Setup the vault client
	client, err := api.NewClient(nil)
	if err != nil {
		logger.Fatal("[ERR] failed to create api client", err)
	}

	// Setup the broker
	broker := &Broker{
		log:    logger,
		client: client,

		serviceID:          config.ServiceID,
		serviceName:        config.ServiceName,
		serviceDescription: config.ServiceDescription,
		serviceTags:        config.ServiceTags,

		planName:        config.PlanName,
		planDescription: config.PlanDescription,

		vaultAdvertiseAddr: config.VaultAdvertiseAddr,
		vaultRenewToken:    config.VaultRenew,
	}
	if err := broker.Start(); err != nil {
		logger.Fatalf("[ERR] failed to start broker: %s", err)
	}

	// Parse the broker credentials
	creds := brokerapi.BrokerCredentials{
		Username: config.SecurityUserName,
		Password: config.SecurityUserPassword,
	}

	// Setup the HTTP handler
	handler := brokerapi.New(broker, lager.NewLogger("vault-broker"), creds)

	// Listen to incoming connection
	serverCh := make(chan struct{}, 1)
	go func() {
		logger.Printf("[INFO] starting server on %s", config.Port)
		if err := http.ListenAndServe(config.Port, handler); err != nil {
			logger.Fatalf("[ERR] server exited with: %s", err)
		}
		close(serverCh)
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case <-serverCh:
	case s := <-signalCh:
		logger.Printf("[INFO] received signal %s", s)
	}

	if err := broker.Stop(); err != nil {
		logger.Fatalf("[ERR] faild to stop broker: %s", err)
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

func parseConfig() (*Configuration, error) {
	config := &Configuration{}
	if err := envconfig.Process("", config); err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return config, nil
}

type Configuration struct {
	// Required
	SecurityUserName     string `envconfig:"security_user_name"`
	SecurityUserPassword string `envconfig:"security_user_password"`
	VaultToken           string `envconfig:"vault_token"`

	// Optional
	CredhubURL         string   `envconfig:"credhub_url"`
	Port               string   `envconfig:"port" default:":8000"`
	ServiceID          string   `envconfig:"service_id" default:"0654695e-0760-a1d4-1cad-5dd87b75ed99"`
	VaultAddr          string   `envconfig:"vault_addr" default:"https://127.0.0.1:8200"`
	VaultAdvertiseAddr string   `envconfig:"vault_advertise_addr"`
	ServiceName        string   `envconfig:"service_name" default:"hashicorp-vault"`
	ServiceDescription string   `envconfig:"service_description" default:"HashiCorp Vault Service Broker"`
	PlanName           string   `envconfig:"plan_name" default:"shared"`
	PlanDescription    string   `envconfig:"plan_description" default:"Secure access to Vault's storage and transit backends"`
	ServiceTags        []string `envconfig:"service_tags"`
	VaultRenew         bool     `envconfig:"vault_renew" default:"true"`
}

func (c *Configuration) Validate() error {
	// Ensure required parameters were provided
	if c.SecurityUserName == "" {
		return errors.New("missing SECURITY_USER_NAME")
	}
	if c.SecurityUserPassword == "" {
		return errors.New("missing SECURITY_USER_PASSWORD")
	}
	if c.VaultToken == "" {
		return errors.New("missing VAULT_TOKEN")
	}

	// If these values aren't perfect, we can fix them
	if !strings.HasPrefix(c.Port, ":") {
		c.Port = ":" + c.Port
	}
	if c.VaultAdvertiseAddr == "" {
		c.VaultAdvertiseAddr = c.VaultAddr
	}
	c.VaultAddr = normalizeAddr(c.VaultAddr)
	c.VaultAdvertiseAddr = normalizeAddr(c.VaultAdvertiseAddr)
	return nil
}
