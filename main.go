package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/vault/api"
	"github.com/pivotal-cf/brokerapi"
)

const (
	// DefaultLogLevel is the default log level unless we
	// get an override via LOG_LEVEL
	DefaultLogLevel = "INFO"

	// DefaultListenAddr is the default address unless we get
	// an override via PORT
	DefaultListenAddr = ":8000"
)

func main() {
	// Parse our log level
	rawLog := DefaultLogLevel
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		rawLog = v
	}
	logLevel, err := parseLogLevel(rawLog)
	if err != nil {
		log.Fatal(err)
	}

	// Setup the logger
	log := lager.NewLogger("vault-broker")
	log.RegisterSink(lager.NewWriterSink(os.Stderr, logLevel))

	// Setup the vault client
	client, err := api.NewClient(nil)
	if err != nil {
		log.Error("main: failed to setup Vault client", err)
		os.Exit(1)
	}

	// Setup the broker
	broker := &Broker{
		log:    log,
		client: client,
	}
	log.Info("main: starting broker")
	if err := broker.Start(); err != nil {
		log.Error("main: failed to start broker", err)
		os.Exit(1)
	}

	// Parse the broker credentials
	creds := brokerapi.BrokerCredentials{
		Username: os.Getenv("SECURITY_USER_NAME"),
		Password: os.Getenv("SECURITY_USER_PASSWORD"),
	}

	// Setup the HTTP handler
	handler := brokerapi.New(broker, log, creds)

	// Parse the listen address
	addr := DefaultListenAddr
	if v := os.Getenv("PORT"); v != "" {
		if v[0] != ':' {
			v = ":" + v
		}
		addr = v
	}

	// Listen to incoming connection
	log.Info(fmt.Sprintf("main: starting http listener on %s", addr))
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Error("main: failed to start http listener", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// parseLogLevel takes a string and returns an associated lager log level or
// an error if one does not exist.
func parseLogLevel(s string) (lager.LogLevel, error) {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return lager.DEBUG, nil
	case "INFO":
		return lager.INFO, nil
	case "ERROR":
		return lager.ERROR, nil
	case "FATAL":
		return lager.FATAL, nil
	default:
		return 0, fmt.Errorf("invalid log level %q", s)
	}
}
