package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"code.cloudfoundry.org/lager"
)

func main() {
	logLevel, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		log.Fatal(err)
	}

	log := lager.NewLogger("vault-broker")
	log.RegisterSink(lager.NewWriterSink(os.Stderr, logLevel))
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
