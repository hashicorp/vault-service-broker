package main

import (
	"os"
	"testing"
	"time"

	"github.com/hashicorp/vault/command"
)

const testToken = "root"

func TestIntegration(t *testing.T) {
	if err := setup(); err != nil {
		t.Fatal(err)
	}

	t.Run("TestBroker_Services", TestBroker_Services)
	t.Run("TestBroker_Provision", TestBroker_Provision)
	t.Run("TestBroker_Deprovision", TestBroker_Deprovision)
	t.Run("TestBroker_Bind", TestBroker_Bind)
	t.Run("TestBroker_Unbind", TestBroker_Unbind)
	t.Run("TestBroker_Update", TestBroker_Update)
	t.Run("TestBroker_LastOperation", TestBroker_LastOperation)
}

func setup() error {

	// Start Vault.
	go command.Run([]string{"server", "-dev", "-dev-root-token-id="+testToken})

	// Wait a few seconds for Vault to get started.
	<- time.After(time.Second * 5)

	// Set the minimum number of env variables needed to run our tests.
	os.Setenv("SECURITY_USER_NAME", "security-user-name")
	os.Setenv("SECURITY_USER_PASSWORD", "security-user-password")
	os.Setenv("VAULT_ADDR", "http://localhost:8200")
	os.Setenv("VAULT_TOKEN", testToken)
	os.Setenv("AUTH_USERNAME", "auth-username")
	os.Setenv("AUTH_PASSWORD", "auth-password")

	// Start the broker and its API locally.
	go main()

	// Wait a few seconds for the broker to get started.
	<- time.After(time.Second * 5)

	return nil
}

func TestBroker_Services(t *testing.T) {
	t.Skip("pending")
}

func TestBroker_Provision(t *testing.T) {
	t.Skip("pending")
}

func TestBroker_Deprovision(t *testing.T) {
	t.Skip("pending")
}

func TestBroker_Bind(t *testing.T) {
	t.Skip("pending")
}

func TestBroker_Unbind(t *testing.T) {
	t.Skip("pending")
}

func TestBroker_Update(t *testing.T) {
	t.Skip("pending")
}

func TestBroker_LastOperation(t *testing.T) {
	t.Skip("pending")
}