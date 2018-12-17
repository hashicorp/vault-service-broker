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

	// DefaultPlanName is the name of our plan, only one is currently supported
	DefaultPlanName = "shared"

	// DefaultPlanDescription is the default description.
	DefaultPlanDescription = "Secure access to Vault's storage and transit backends"

	//DefaultDisplayName is the default name of the service to be displayed in graphical clients.
	DefaultDisplayName = "Vault for PCF"

	//DefaultImageUrl is the default URL to an image.
	DefaultImageUrl = "data:image/gif;base64,iVBORw0KGgoAAAANSUhEUgAAAQAAAAEABAMAAACuXLVVAAAAJ1BMVEVHcEwVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUVFRUPAUIJAAAADHRSTlMAYOgXdQi7SS72ldNTKM7gAAAE00lEQVR42u3dvUscQRQA8JFzuSvlIJVpDBIhXGFlcZUYDFx3gmkskyIWV0iKpNjmqmvSpolsJwEPbEyjxTU5grD6/qgUfu3u7M7XvvcmgffKBZ0fem92dvbmPaUkJCQkJP7HSOovb7ON/67++psxEyC9qb8+2OAZfwZNALjiGH+YNQPyj/TjHwygGQD5PvX43QWYALBcox2/NwEzAG6mlON3RmADwC3ldNAHOwC+jwkT0AVAl4xDcAPAMc34h5krAH6SJaAjYLlPlYCOALg7QU/AOfgA8KeDPfAD5Ke4yZiCJwAA9d68A/4A+IQ3/lEWAoAzrPFXqr/ZEYB1b+4tIAwAv3ES8AKiApIRxAWsQ1zADOIChllcwOEAogK6C4gKKN6BYwCSOSABemEL5T5gAVaDFsop4AFgKyABc0yA/0L5MANUgO+9eWUAyIDlLkoChgO8Fso1d+D2AI+FcrIHFAA43W6fgK0ArgvlGVAB4Dr8DlyK41CAy3RgTkDjgt8B8GM/9A5ciMb9BweAdROrM7GOvzluA7AkY90SuBKGXHICmDex+tbxTT/uBjD8CV0S0PQHdAQ0f4iG1vHN87kroCkZO9YEtHyEnQF5/f+xYx3fksTOAAgD5LY1BTXgXMUF2KdxWoDDBjApYGMcF+D0ZEEIcHsJQQdwXE6SAVwX1FSAO20C7rIC9Am4+4sToE/AvcmSE3Be8+aAE3Bct2/CCLiqXbXxAfQJOAVOgD4B368auQD6Cvxh1coF2G16c8IFWGvauI0EeH5sjQMoPLZGART3rWIAesV9qwiA0qvjCIDKvhk/oPLmih3wBeICdiAy4KUABCAAAQhAAAIQgAA0wPva4AO4hgAEIAABCEAAAhCAAAQgAAEIQAACEIAA6PaIvtaGbNMJQAACEIAABCAAAfx7gOvIgKcTnpEAz99KjgMofCs5CuB2qqICSsdSIgDKx1L4AcsXKipg+VbFBVSPpXADtGMhzADtYLZezoMUcKmNr1cToATop4o/AydAPxbyDTgBxQn4PmoPU5MB9HOB9dUMqAD6ucCGciZEgFe71StN1RSIAPq5wAWwAqrRXE6FB2Co5sACaK6nxAMwlnPhABirSTAAzOVk6AGWahbkAFs1C2rAURYXsDqAqICVBYQBtKVDGKA7sY6/5Zi7QYDSucD6aK6mUJk9QwCduXV8UzWF8v0rAGCtp2WrZlC6g/sDknXr+LaKSMWajP4Aaz0vh1rKhVWkN8BeTih3qIo1CwbYywm51dNO0bbptDAUYyp+kkZUANfacI+18bAB7tXxHpIRGeBTH/D+gQIX4Fe9+ChDB3jWiNzBBlwrz0hxAf41vJM+JuDPtjdAdeZ4gJuA8ZXqTbEAbSvZtwVUNm75Aa27GbQEtO/n0A6A0NGiFQCjp0cbAEpXkxYAnEYO4QCkzjbBAKz+AcFFMNA6KKRhALyWMkk/BIDZRaPyyOn76hYhyk+tLgDsTiqlp1YHAH4vmeJTqx1A0U1n6AM4wx9fjWfuAJqOSs/JaANcKpp42kKyAOi6aj0moxlA2VfsIRmNgLupIoyDgQ1A3VtumJkB+ZkijpkZwNBfMDUBODosJqNmwKbiiM5FE4C0sV9xOvhQf/31lGd8lTTUqj1REhISEhISAfEXumiA5AUel8MAAAAASUVORK5CYII="

	//DefaultImageUrl is the default Long description.
	DefaultLongDescription = "The official HashiCorp Vault broker integration to the Open Service Broker API. This service broker provides support for secure secret storage and encryption-as-a-service to HashiCorp Vault."

	//DefaultProviderDisplayName is the default name of the upstream entity providing the actual service.
	DefaultProviderDisplayName = "HashiCorp"

	//DefaultDocumentationUrl is the default link to documentation page for the service.
	DefaultDocumentationUrl = "https://www.vaultproject.io/"

	//DefaultSupportUrl is the default link to support page for the service.
	DefaultSupportUrl = "https://support.hashicorp.com/"

	//DefaultSupportUrl is the default link to support page for the service.
	DefaultPlanMetadataName = "Architecture and Assumptions"

	//DefaultPlanBulletsString are the default bullets to add to the plan
	DefaultPlanBulletsString = "The Vault server is already running and is accessible by the broker;" +
		"The Vault server may be used by other applications (it is not exclusively tied to Cloud Foundry).;" +
		"All instances of an application will share a token. This goes against the recommended Vault usage, but this is a limitation of the Cloud Foundry service broker model.;" +
		"Any Vault operations performed outside of Cloud Foundry will require users to rebind their instances."
)

func main() {
	// Setup the logger - intentionally do not log date or time because it will
	// be prefixed in the log output by CF.
	logger := log.New(os.Stdout, "", 0)

	// Ensure username and password are present
	username := os.Getenv("SECURITY_USER_NAME")
	if username == "" {
		logger.Fatal("[ERR] missing SECURITY_USER_NAME")
	}
	password := os.Getenv("SECURITY_USER_PASSWORD")
	if password == "" {
		logger.Fatal("[ERR] missing SECURITY_USER_PASSWORD")
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

	// Get the display name
	displayName := os.Getenv("DISPLAY_NAME")
	if displayName == "" {
		displayName = DefaultDisplayName
	}

	// Get the image url
	imageUrl := os.Getenv("IMAGE_URL")
	if imageUrl == "" {
		imageUrl = DefaultImageUrl
	}

	// Get the image url
	longDescription := os.Getenv("LONG_DESCRIPTION")
	if longDescription == "" {
		longDescription = DefaultLongDescription
	}

	// Get the provider display name
	providerDisplayName := os.Getenv("PROVIDER_DISPLAY_NAME")
	if providerDisplayName == "" {
		providerDisplayName = DefaultProviderDisplayName
	}

	// Get the documentation url
	documentationUrl := os.Getenv("DOCUMENTATION_URL")
	if documentationUrl == "" {
		documentationUrl = DefaultDocumentationUrl
	}

	// Get the support url
	supportUrl := os.Getenv("SUPPORT_URL")
	if supportUrl == "" {
		supportUrl = DefaultSupportUrl
	}

	// Get the plan name metadata
	planMetadataName := os.Getenv("PLAN_METADATA_NAME")
	if planMetadataName == "" {
		planMetadataName = DefaultPlanMetadataName
	}

	// Get the service bullets
	planBulletsString := os.Getenv("PLAN_BULLETS")
	if planBulletsString == "" {
		planBulletsString = DefaultPlanBulletsString
	}
	planBullets := strings.Split(planBulletsString, ";")

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
		vaultAddr = DefaultVaultAddr
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
			logger.Fatalf("[ERR] failed to parse VAULT_RENEW: %s", err)
		}
		renew = b
	}

	// Check for vault token
	if v := os.Getenv("VAULT_TOKEN"); v == "" {
		logger.Fatal("[ERR] missing VAULT_TOKEN")
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

		serviceID:          serviceID,
		serviceName:        serviceName,
		serviceDescription: serviceDescription,
		serviceTags:        serviceTags,

		planName:         planName,
		planDescription:  planDescription,
		planMetadataName: planMetadataName,
		planBullets:      planBullets,

		displayName:         displayName,
		imageUrl:            imageUrl,
		longDescription:     longDescription,
		providerDisplayName: providerDisplayName,
		documentationUrl:    documentationUrl,
		supportUrl:          supportUrl,

		vaultAdvertiseAddr: vaultAdvertiseAddr,
		vaultRenewToken:    renew,
	}
	if err := broker.Start(); err != nil {
		logger.Fatalf("[ERR] failed to start broker: %s", err)
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
		logger.Printf("[INFO] starting server on %s", port)
		if err := http.ListenAndServe(port, handler); err != nil {
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
