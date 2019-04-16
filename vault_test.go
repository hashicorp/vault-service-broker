package main

import (
	"bytes"
	"testing"
)

func TestGeneratePolicy(t *testing.T) {
	w := new(bytes.Buffer)
	info := &instanceInfo{
		OrganizationGUID: "org-id",
		SpaceGUID: "space-id",
		ServiceInstanceGUID: "service-instance-id",
		ApplicationGUID: "application-id",
	}
	if err := GeneratePolicy(w, info); err != nil {
		t.Fatal(err)
	}
	result := w.String()
	if result != expected {
		t.Fatalf("received unexpected policy of %s", result)
	}
}

var expected = `
path "cf/service-instance-id" {
  capabilities = ["list"]
}

path "cf/service-instance-id/*" {
	capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/space-id" {
  capabilities = ["list"]
}

path "cf/space-id/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/org-id" {
  capabilities = ["list"]
}

path "cf/org-id/*" {
  capabilities = ["read", "list"]
}

path "cf/application-id" {
  capabilities = ["list"]
}

path "cf/application-id/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
`