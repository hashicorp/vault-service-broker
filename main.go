package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	uaa "code.cloudfoundry.org/uaa-go-client"
	uaaconf "code.cloudfoundry.org/uaa-go-client/config"
	credhub "github.com/cloudfoundry-community/go-credhub"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/vault/api"
	"github.com/kelseyhightower/envconfig"
	"github.com/pivotal-cf/brokerapi"
)

func main() {
	// Setup the logger - intentionally do not log date or time because it will
	// be prefixed in the log output by CF.
	logger := log.New(os.Stdout, "", 0)
	cfLogger := lager.NewLogger("vault-broker")

	config, err := parseConfig(cfLogger)
	if err != nil {
		logger.Fatal("[ERR] failed to read configuration", err)
	}

	// Setup the vault client
	vaultClientConfig := api.DefaultConfig()
	vaultClientConfig.HttpClient = cleanhttp.DefaultClient()

	vaultClient, err := api.NewClient(vaultClientConfig)
	if err != nil {
		logger.Fatal("[ERR] failed to create vault api client", err)
	}

	vaultClient.SetAddress(config.VaultAddr)
	vaultClient.SetToken(config.VaultToken)
	if config.VaultNamespace != "" {
		vaultClient.SetNamespace(config.VaultNamespace)
	}

	// Setup the broker
	broker := &Broker{
		log:         logger,
		vaultClient: vaultClient,

		serviceID:          config.ServiceID,
		serviceName:        config.ServiceName,
		serviceDescription: config.ServiceDescription,
		serviceTags:        config.ServiceTags,

		planName:         config.PlanName,
		planDescription:  config.PlanDescription,
		planMetadataName: config.PlanMetadataName,
		planBullets:      config.PlanBullets,

		displayName:         config.DisplayName,
		imageUrl:            config.ImageUrl.String(),
		longDescription:     config.LongDescription,
		providerDisplayName: config.ProviderDisplayName,
		documentationUrl:    config.DocumentationUrl,
		supportUrl:          config.SupportUrl,

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
	handler := brokerapi.New(broker, cfLogger, creds)

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

func parseConfig(cfLogger lager.Logger) (*Configuration, error) {
	config := &Configuration{}
	if err := envconfig.Process("", config); err != nil {
		return nil, err
	}
	if config.CredhubURL != "" {
		if err := credhubProcess(cfLogger, "VAULT_SERVICE_BROKER_", config); err != nil {
			return nil, err
		}
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

	// Optional, for using CredHub
	CredhubURL                       string `envconfig:"credhub_url"`
	UAAEndpoint                      string `envconfig:"uaa_endpoint"`
	UAAClientName                    string `envconfig:"uaa_client_name"`
	UAAClientSecret                  string `envconfig:"uaa_client_secret"`
	UAACACerts                       string `envconfig:"uaa_ca_certs"`
	UAASkipVerification              bool   `envconfig:"uaa_skip_verification"`
	UAAInsecureAllowAnySigningMethod bool   `envconfig:"uaa_insecure_allow_any_signing_method"`

	// Also optional
	Port               string `envconfig:"port" default:":8000"`
	ServiceID          string `envconfig:"service_id" default:"0654695e-0760-a1d4-1cad-5dd87b75ed99"`
	VaultAddr          string `envconfig:"vault_addr" default:"https://127.0.0.1:8200"`
	VaultAdvertiseAddr string `envconfig:"vault_advertise_addr"`
	VaultNamespace     string `envconfig:"vault_namespace"`
	ServiceName        string `envconfig:"service_name" default:"hashicorp-vault"`
	ServiceDescription string `envconfig:"service_description" default:"HashiCorp Vault Service Broker"`
	PlanName           string `envconfig:"plan_name" default:"shared"`
	PlanDescription    string `envconfig:"plan_description" default:"Secure access to Vault's storage and transit backends"`

	PlanMetadataName    string          `envconfig:"plan_metadata_name" default:"Architecture and Assumptions"`
	PlanBullets         []string        `envconfig:"plan_bullets" default:"The Vault server is already running and is accessible by the broker.,The Vault server may be used by other applications (it is not exclusively tied to Cloud Foundry).,All instances of an application will share a token. This goes against the recommended Vault usage. This is a limitation of the Cloud Foundry service broker model.,Any Vault operations performed outside of Cloud Foundry will require users to rebind their instances."`
	DisplayName         string          `envconfig:"display_name" default:"Vault for PCF"`
	ImageUrl            *ImageDefaulter `envconfig:"image_url" default:"unset"`
	LongDescription     string          `envconfig:"long_description" default:"The official HashiCorp Vault broker integration to the Open Service Broker API. This service broker provides support for secure secret storage and encryption-as-a-service to HashiCorp Vault."`
	ProviderDisplayName string          `envconfig:"provider_display_name" default:"HashiCorp"`
	DocumentationUrl    string          `envconfig:"documentation_url" default:"https://www.vaultproject.io/"`
	SupportUrl          string          `envconfig:"support_url" default:"https://support.hashicorp.com/"`

	ServiceTags []string `envconfig:"service_tags"`
	VaultRenew  bool     `envconfig:"vault_renew" default:"true"`
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

// credhubProcess iterates over the names of variables as set in the `envconfig` tag
// on the Configuration. It prepends them with "prefix" and then looks
// in Credhub to see if they exist. If they do and they have a value, the Configuration
// is updated with that value for that field.
func credhubProcess(cfLogger lager.Logger, prefix string, config *Configuration) error {
	uaaConf := &uaaconf.Config{
		ClientName:                    config.UAAClientName,
		ClientSecret:                  config.UAAClientSecret,
		UaaEndpoint:                   config.UAAEndpoint,
		SkipVerification:              config.UAASkipVerification,
		CACerts:                       config.UAACACerts,
		InsecureAllowAnySigningMethod: config.UAAInsecureAllowAnySigningMethod,
	}
	uaaClient, err := uaa.NewClient(cfLogger, uaaConf, clock.NewClock())
	if err != nil {
		return err
	}
	credhubClient := credhub.New(config.CredhubURL, credhub.NewUAAAuthClient(cleanhttp.DefaultClient(), uaaClient))

	// Pull the "envconfig" field name from each field and look for it in Credhub
	configTypeInfo := reflect.TypeOf(*config)
	settableConfig := reflect.ValueOf(config).Elem()

	for i := 0; i < configTypeInfo.NumField(); i++ {
		fieldTypeInfo := configTypeInfo.Field(i)
		credhubName := prefix + strings.ToUpper(fieldTypeInfo.Tag.Get("envconfig"))

		latest, err := credhubClient.GetLatestByName(credhubName)
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
			return err
		}
		if latest == nil {
			// This key doesn't exist in Credhub
			continue
		}
		settingValue, ok := latest.Value.(string)
		if !ok {
			return fmt.Errorf("we only support credhub values as bash-like string values, but received %s as a %s", credhubName, reflect.TypeOf(latest.Value))
		}
		if settingValue == "" {
			// The value for this key isn't set in Credhub
			continue
		}

		// Update the value for this field with Credhub's value
		settableField := settableConfig.Field(i)
		switch fieldTypeInfo.Type.Kind() {
		case reflect.Bool:
			asBool, err := strconv.ParseBool(settingValue)
			if err != nil {
				return fmt.Errorf("error parsing bool %s: %s", credhubName, err)
			}
			settableField.SetBool(asBool)
		case reflect.String:
			settableField.SetString(settingValue)
		case reflect.Slice:
			settableField.Set(reflect.ValueOf(strings.Split(settingValue, ",")))
		default:
			return fmt.Errorf("unsupported type of %s for %s", fieldTypeInfo.Type.Kind(), credhubName)
		}
	}
	return nil
}

type ImageDefaulter struct {
	Image string
}

func (d *ImageDefaulter) Decode(image string) error {
	d.Image = image
	if d.Image == "unset" {
		def, err := ioutil.ReadFile("dflt_img_url.txt")
		if err != nil {
			return err
		}
		d.Image = fmt.Sprintf("%s", def)
	}
	return nil
}

func (d *ImageDefaulter) String() string {
	return d.Image
}
