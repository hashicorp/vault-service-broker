package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/vault/api"
	"github.com/pivotal-cf/brokerapi"
)

const (
	// DefaultListenAddr is the default address unless we get
	// an override via PORT
	DefaultListenAddr = ":8000"

	// DefaultUUID is the default UUID of the services
	DefaultUUID = "0654695e-0760-a1d4-1cad-5dd87b75ed99"
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
	guid := os.Getenv("BROKER_GUID")
	if guid == "" {
		guid = DefaultUUID
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
		guid:   guid,
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

	// Parse the listen address
	addr := DefaultListenAddr
	if v := os.Getenv("PORT"); v != "" {
		if v[0] != ':' {
			v = ":" + v
		}
		addr = v
	}

	// Listen to incoming connection
	serverCh := make(chan struct{}, 1)
	go func() {
		log.Printf("[INFO] starting server on %s", addr)
		if err := http.ListenAndServe(addr, handler); err != nil {
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
