package main

import (
	"code.cloudfoundry.org/lager/lagertest"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var logger = lagertest.NewTestLogger("vault-broker-test")

func TestNormalizeAddr(t *testing.T) {
	cases := []struct {
		name string
		i    string
		e    string
	}{
		{
			"empty",
			"",
			"",
		},
		{
			"scheme",
			"www.example.com",
			"https://www.example.com/",
		},
		{
			"trailing-slash",
			"https://www.example.com/foo",
			"https://www.example.com/foo/",
		},
		{
			"trailing-slash-many",
			"https://www.example.com/foo///////",
			"https://www.example.com/foo/",
		},
		{
			"no-overwrite-scheme",
			"ftp://foo.com/",
			"ftp://foo.com/",
		},
		{
			"port",
			"www.example.com:8200",
			"https://www.example.com:8200/",
		},
		{
			"port-scheme",
			"http://www.example.com:8200",
			"http://www.example.com:8200/",
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			r := normalizeAddr(tc.i)
			if r != tc.e {
				t.Errorf("expected %q to be %q", r, tc.e)
			}
		})
	}
}

func TestParseConfigDefaults(t *testing.T) {
	os.Clearenv()

	os.Setenv("SECURITY_USER_NAME", "fizz")
	os.Setenv("SECURITY_USER_PASSWORD", "buzz")
	os.Setenv("VAULT_TOKEN", "bang")

	config, err := parseConfig(logger)
	if err != nil {
		t.Fatal(err)
	}
	if config.SecurityUserName != "fizz" {
		t.Fatalf("expected %s but received %s", `"fizz"`, config.SecurityUserName)
	}
	if config.SecurityUserPassword != "buzz" {
		t.Fatalf("expected %s but received %s", `"buzz"`, config.SecurityUserPassword)
	}
	if config.VaultToken != "bang" {
		t.Fatalf("expected %s but received %s", `"bang"`, config.VaultToken)
	}
	if config.CredhubURL != "" {
		t.Fatalf("expected %s but received %s", `""`, config.CredhubURL)
	}
	if config.Port != ":8000" {
		t.Fatalf("expected %s but received %s", `":8000"`, config.Port)
	}
	if config.ServiceID != "0654695e-0760-a1d4-1cad-5dd87b75ed99" {
		t.Fatalf("expected %s but received %s", `"0654695e-0760-a1d4-1cad-5dd87b75ed99"`, config.ServiceID)
	}
	if config.VaultAddr != "https://127.0.0.1:8200/" {
		t.Fatalf("expected %s but received %s", `"https://127.0.0.1:8200/"`, config.VaultAddr)
	}
	if config.VaultAdvertiseAddr != "https://127.0.0.1:8200/" {
		t.Fatalf("expected %s but received %s", `"https://127.0.0.1:8200/"`, config.VaultAdvertiseAddr)
	}
	if config.ServiceName != "hashicorp-vault" {
		t.Fatalf("expected %s but received %s", `"hashicorp-vault"`, config.ServiceName)
	}
	if config.ServiceDescription != "HashiCorp Vault Service Broker" {
		t.Fatalf("expected %s but received %s", `"HashiCorp Vault Service Broker"`, config.ServiceDescription)
	}
	if config.PlanName != "shared" {
		t.Fatalf("expected %s but received %s", `"shared"`, config.PlanName)
	}
	if config.PlanDescription != "Secure access to Vault's storage and transit backends" {
		t.Fatalf("expected %s but received %s", `"Secure access to Vault's storage and transit backends"`, config.PlanDescription)
	}
	if len(config.ServiceTags) != 0 {
		t.Fatalf("expected %d but received %d: %s", 0, len(config.ServiceTags), config.ServiceTags)
	}
	if config.VaultRenew != true {
		t.Fatal("expected true but received false")
	}
}

func TestParseConfigFromEnv(t *testing.T) {
	os.Clearenv()

	os.Setenv("SECURITY_USER_NAME", "fizz")
	os.Setenv("SECURITY_USER_PASSWORD", "buzz")
	os.Setenv("VAULT_TOKEN", "bang")

	os.Setenv("PORT", "8080")
	os.Setenv("SERVICE_ID", "1234")
	os.Setenv("VAULT_ADDR", "http://localhost:8200")
	os.Setenv("VAULT_ADVERTISE_ADDR", "https://some-domain.com")
	os.Setenv("SERVICE_NAME", "vault")
	os.Setenv("SERVICE_DESCRIPTION", "Vault, by Hashicorp")
	os.Setenv("PLAN_NAME", "free")
	os.Setenv("PLAN_DESCRIPTION", "Can you believe it's opensource?")
	os.Setenv("SERVICE_TAGS", "hello,world")
	os.Setenv("VAULT_RENEW", "false")

	config, err := parseConfig(logger)
	if err != nil {
		t.Fatal(err)
	}
	if config.SecurityUserName != "fizz" {
		t.Fatalf("expected %s but received %s", `"fizz"`, config.SecurityUserName)
	}
	if config.SecurityUserPassword != "buzz" {
		t.Fatalf("expected %s but received %s", `"buzz"`, config.SecurityUserPassword)
	}
	if config.VaultToken != "bang" {
		t.Fatalf("expected %s but received %s", `"bang"`, config.VaultToken)
	}
	if config.CredhubURL != "" {
		t.Fatalf("expected %s but received %s", `""`, config.CredhubURL)
	}
	if config.Port != ":8080" {
		t.Fatalf("expected %s but received %s", `":8080"`, config.Port)
	}
	if config.ServiceID != "1234" {
		t.Fatalf("expected %s but received %s", `"1234"`, config.ServiceID)
	}
	if config.VaultAddr != "http://localhost:8200/" {
		t.Fatalf("expected %s but received %s", `"http://localhost:8200/"`, config.VaultAddr)
	}
	if config.VaultAdvertiseAddr != "https://some-domain.com/" {
		t.Fatalf("expected %s but received %s", `"https://some-domain.com/"`, config.VaultAdvertiseAddr)
	}
	if config.ServiceName != "vault" {
		t.Fatalf("expected %s but received %s", `"vault"`, config.ServiceName)
	}
	if config.ServiceDescription != "Vault, by Hashicorp" {
		t.Fatalf("expected %s but received %s", `"Vault, by Hashicorp"`, config.ServiceDescription)
	}
	if config.PlanName != "free" {
		t.Fatalf("expected %s but received %s", `"free"`, config.PlanName)
	}
	if config.PlanDescription != "Can you believe it's opensource?" {
		t.Fatalf("expected %s but received %s", `"Can you believe it's opensource?"`, config.PlanDescription)
	}
	if len(config.ServiceTags) != 2 {
		t.Fatalf("expected %d but received %d: %s", 2, len(config.ServiceTags), config.ServiceTags)
	}
	if config.VaultRenew != false {
		t.Fatal("expected false but received true")
	}
}

func TestParseConfigFromCredhub(t *testing.T) {
	os.Clearenv()

	credhubTs := testCredhubServer()
	defer credhubTs.Close()
	os.Setenv("CREDHUB_URL", credhubTs.URL)

	uaaTs := uaaTestServer()
	defer uaaTs.Close()
	os.Setenv("UAA_ENDPOINT", uaaTs.URL)
	os.Setenv("UAA_CLIENT_NAME", "client-name")
	os.Setenv("UAA_CLIENT_SECRET", "client-secret")

	config, err := parseConfig(logger)
	if err != nil {
		t.Fatal(err)
	}
	if config.SecurityUserName != "securityUserName" {
		t.Fatalf("expected %s but received %s", `"securityUserName"`, config.SecurityUserName)
	}
	if config.SecurityUserPassword != "securityUserPassword" {
		t.Fatalf("expected %s but received %s", `"securityUserPassword"`, config.SecurityUserPassword)
	}
	if config.VaultToken != "vaultToken" {
		t.Fatalf("expected %s but received %s", `"vaultToken"`, config.VaultToken)
	}
	if config.CredhubURL != credhubTs.URL {
		t.Fatalf("expected %s but received %s", credhubTs.URL, config.CredhubURL)
	}
	if config.UAAEndpoint != uaaTs.URL {
		t.Fatalf("expected %s but received %s", uaaTs.URL, config.UAAEndpoint)
	}
	if config.UAAClientName != "client-name" {
		t.Fatalf("expected %s but received %s", "client-name", config.UAAClientName)
	}
	if config.UAAClientSecret != "client-secret" {
		t.Fatalf("expected %s but received %s", "client-secret", config.UAAClientSecret)
	}
	if config.Port != ":8080" {
		t.Fatalf("expected %s but received %s", `":8080"`, config.Port)
	}
	if config.ServiceID != "serviceID" {
		t.Fatalf("expected %s but received %s", `"serviceID"`, config.ServiceID)
	}
	if config.VaultAddr != "https://vaultAddr/" {
		t.Fatalf("expected %s but received %s", `"http:s//vaultAddr/"`, config.VaultAddr)
	}
	if config.VaultAdvertiseAddr != "https://vaultAdvertiseAddr/" {
		t.Fatalf("expected %s but received %s", `"https://vaultAdvertiseAddr/"`, config.VaultAdvertiseAddr)
	}
	if config.ServiceName != "serviceName" {
		t.Fatalf("expected %s but received %s", `"serviceName"`, config.ServiceName)
	}
	if config.ServiceDescription != "serviceDescription" {
		t.Fatalf("expected %s but received %s", `"serviceDescription"`, config.ServiceDescription)
	}
	if config.PlanName != "planName" {
		t.Fatalf("expected %s but received %s", `"planName"`, config.PlanName)
	}
	if config.PlanDescription != "planDescription" {
		t.Fatalf("expected %s but received %s", `"planDescription"`, config.PlanDescription)
	}
	if len(config.ServiceTags) != 2 {
		t.Fatalf("expected %d but received %d: %s", 2, len(config.ServiceTags), config.ServiceTags)
	}
	if config.VaultRenew != false {
		t.Fatal("expected false but received true")
	}
}

func TestCredhubConfigOverridesEnvConfig(t *testing.T) {
	os.Clearenv()

	os.Setenv("SECURITY_USER_NAME", "fizz")
	os.Setenv("SECURITY_USER_PASSWORD", "buzz")
	os.Setenv("VAULT_TOKEN", "bang")

	os.Setenv("PORT", "8080")
	os.Setenv("SERVICE_ID", "1234")
	os.Setenv("VAULT_ADDR", "http://localhost:8200")
	os.Setenv("VAULT_ADVERTISE_ADDR", "https://some-domain.com")
	os.Setenv("SERVICE_NAME", "vault")
	os.Setenv("SERVICE_DESCRIPTION", "Vault, by Hashicorp")
	os.Setenv("PLAN_NAME", "free")
	os.Setenv("PLAN_DESCRIPTION", "Can you believe it's opensource?")
	os.Setenv("SERVICE_TAGS", "hello,world")
	os.Setenv("VAULT_RENEW", "false")

	credhubTs := testCredhubServer()
	defer credhubTs.Close()
	os.Setenv("CREDHUB_URL", credhubTs.URL)

	uaaTs := uaaTestServer()
	defer uaaTs.Close()
	os.Setenv("UAA_ENDPOINT", uaaTs.URL)
	os.Setenv("UAA_CLIENT_NAME", "client-name")
	os.Setenv("UAA_CLIENT_SECRET", "client-secret")

	config, err := parseConfig(logger)
	if err != nil {
		t.Fatal(err)
	}
	if config.SecurityUserName != "securityUserName" {
		t.Fatalf("expected %s but received %s", `"securityUserName"`, config.SecurityUserName)
	}
	if config.SecurityUserPassword != "securityUserPassword" {
		t.Fatalf("expected %s but received %s", `"securityUserPassword"`, config.SecurityUserPassword)
	}
	if config.VaultToken != "vaultToken" {
		t.Fatalf("expected %s but received %s", `"vaultToken"`, config.VaultToken)
	}
	if config.CredhubURL != credhubTs.URL {
		t.Fatalf("expected %s but received %s", credhubTs.URL, config.CredhubURL)
	}
	if config.UAAEndpoint != uaaTs.URL {
		t.Fatalf("expected %s but received %s", uaaTs.URL, config.UAAEndpoint)
	}
	if config.UAAClientName != "client-name" {
		t.Fatalf("expected %s but received %s", "client-name", config.UAAClientName)
	}
	if config.UAAClientSecret != "client-secret" {
		t.Fatalf("expected %s but received %s", "client-secret", config.UAAClientSecret)
	}
	if config.Port != ":8080" {
		t.Fatalf("expected %s but received %s", `":8080"`, config.Port)
	}
	if config.ServiceID != "serviceID" {
		t.Fatalf("expected %s but received %s", `"serviceID"`, config.ServiceID)
	}
	if config.VaultAddr != "https://vaultAddr/" {
		t.Fatalf("expected %s but received %s", `"http:s//vaultAddr/"`, config.VaultAddr)
	}
	if config.VaultAdvertiseAddr != "https://vaultAdvertiseAddr/" {
		t.Fatalf("expected %s but received %s", `"https://vaultAdvertiseAddr/"`, config.VaultAdvertiseAddr)
	}
	if config.ServiceName != "serviceName" {
		t.Fatalf("expected %s but received %s", `"serviceName"`, config.ServiceName)
	}
	if config.ServiceDescription != "serviceDescription" {
		t.Fatalf("expected %s but received %s", `"serviceDescription"`, config.ServiceDescription)
	}
	if config.PlanName != "planName" {
		t.Fatalf("expected %s but received %s", `"planName"`, config.PlanName)
	}
	if config.PlanDescription != "planDescription" {
		t.Fatalf("expected %s but received %s", `"planDescription"`, config.PlanDescription)
	}
	if len(config.ServiceTags) != 2 {
		t.Fatalf("expected %d but received %d: %s", 2, len(config.ServiceTags), config.ServiceTags)
	}
	if config.VaultRenew != false {
		t.Fatal("expected false but received true")
	}
}

func testCredhubServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/data" {
			writer.WriteHeader(400)
			writer.Write([]byte(fmt.Sprintf("unrecognized path: %s", request.URL.Path)))
			return
		}
		respVal := ""
		switch request.URL.Query().Get("name") {
		case "VAULT_SERVICE_BROKER_SECURITY_USER_NAME":
			respVal = "securityUserName"
		case "VAULT_SERVICE_BROKER_SECURITY_USER_PASSWORD":
			respVal = "securityUserPassword"
		case "VAULT_SERVICE_BROKER_VAULT_TOKEN":
			respVal = "vaultToken"
		case "VAULT_SERVICE_BROKER_PORT":
			respVal = "8080"
		case "VAULT_SERVICE_BROKER_SERVICE_ID":
			respVal = "serviceID"
		case "VAULT_SERVICE_BROKER_VAULT_ADDR":
			respVal = "vaultAddr"
		case "VAULT_SERVICE_BROKER_VAULT_ADVERTISE_ADDR":
			respVal = "vaultAdvertiseAddr"
		case "VAULT_SERVICE_BROKER_SERVICE_NAME":
			respVal = "serviceName"
		case "VAULT_SERVICE_BROKER_SERVICE_DESCRIPTION":
			respVal = "serviceDescription"
		case "VAULT_SERVICE_BROKER_PLAN_NAME":
			respVal = "planName"
		case "VAULT_SERVICE_BROKER_PLAN_DESCRIPTION":
			respVal = "planDescription"
		case "VAULT_SERVICE_BROKER_SERVICE_TAGS":
			respVal = "service,tags"
		case "VAULT_SERVICE_BROKER_VAULT_RENEW":
			respVal = "false"
		default:
			writer.WriteHeader(400)
		}
		respBody := fmt.Sprintf(`{
			"data": [{
				"type": "password",
				"version_created_at": "2017-01-05T01:01:01Z",
				"id": "2993f622-cb1e-4e00-a267-4b23c273bf3d",
				"name": "/example-password",
				"value": "%s"
			}]
		}`, respVal)

		writer.WriteHeader(200)
		writer.Write([]byte(respBody))
	}))
}

func uaaTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			fmt.Fprintf(w, `{
			  "access_token" : "8d952f1311c041d19253fc01c2145144",
			  "token_type" : "bearer",
			  "id_token" : "eyJhbGciOiJIUzI1NiIsImprdSI6Imh0dHBzOi8vbG9jYWxob3N0OjgwODAvdWFhL3Rva2VuX2tleXMiLCJraWQiOiJsZWdhY3ktdG9rZW4ta2V5IiwidHlwIjoiSldUIn0.eyJzdWIiOiJjMWJhZTk2OC1hMjFlLTQ5ODItOGQwYi03ODJjMjQwNGI3OWYiLCJhdWQiOlsibG9naW4iXSwiaXNzIjoiaHR0cDovL2xvY2FsaG9zdDo4MDgwL3VhYS9vYXV0aC90b2tlbiIsImV4cCI6MTU0NTQ3NjcwNSwiaWF0IjoxNTQ1NDMzNTA1LCJhenAiOiJsb2dpbiIsInNjb3BlIjpbIm9wZW5pZCJdLCJlbWFpbCI6IkQ3a1J6RkB0ZXN0Lm9yZyIsInppZCI6InVhYSIsIm9yaWdpbiI6InVhYSIsImp0aSI6IjhkOTUyZjEzMTFjMDQxZDE5MjUzZmMwMWMyMTQ1MTQ0IiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImNsaWVudF9pZCI6ImxvZ2luIiwiY2lkIjoibG9naW4iLCJncmFudF90eXBlIjoiYXV0aG9yaXphdGlvbl9jb2RlIiwidXNlcl9uYW1lIjoiRDdrUnpGQHRlc3Qub3JnIiwicmV2X3NpZyI6IjRkOWQ4ZjY5IiwidXNlcl9pZCI6ImMxYmFlOTY4LWEyMWUtNDk4Mi04ZDBiLTc4MmMyNDA0Yjc5ZiIsImF1dGhfdGltZSI6MTU0NTQzMzUwNX0.DDqZtEIaTgtIhT0iaRyEoNvDpsGvHuUMyxOS9Zo5fhI",
			  "refresh_token" : "331e025fe0384bf588fae5bba0b7f784-r",
			  "expires_in" : 43199,
			  "scope" : "openid oauth.approvals",
			  "jti" : "8d952f1311c041d19253fc01c2145144"
			}`)
		}
	}))
}
