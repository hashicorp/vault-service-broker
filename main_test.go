package main

import (
	"fmt"
	"os"
	"testing"
)

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
	ensureEnvVarsAreUnset()

	os.Setenv("SECURITY_USER_NAME", "fizz")
	os.Setenv("SECURITY_USER_PASSWORD", "buzz")
	os.Setenv("VAULT_TOKEN", "bang")

	config, err := parseConfig()
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
	ensureEnvVarsAreUnset()

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

	config, err := parseConfig()
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

func ensureEnvVarsAreUnset() {
	os.Unsetenv("SECURITY_USER_NAME")
	os.Unsetenv("SECURITY_USER_PASSWORD")
	os.Unsetenv("VAULT_TOKEN")
	os.Unsetenv("CREDHUB_URL")
	os.Unsetenv("PORT")
	os.Unsetenv("SERVICE_ID")
	os.Unsetenv("VAULT_ADDR")
	os.Unsetenv("VAULT_ADVERTISE_ADDR")
	os.Unsetenv("SERVICE_NAME")
	os.Unsetenv("SERVICE_DESCRIPTION")
	os.Unsetenv("PLAN_NAME")
	os.Unsetenv("PLAN_DESCRIPTION")
	os.Unsetenv("SERVICE_TAGS")
	os.Unsetenv("VAULT_RENEW")
}
