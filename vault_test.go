package main

import (
	"bytes"
	"testing"
)

func TestGeneratePolicy(t *testing.T) {
	w := new(bytes.Buffer)
	info := &ServicePolicyTemplateInput{
		OrgID:      "org-id",
		SpaceID:    "space-id",
		InstanceID: "service-instance-id",
	}
	if err := GeneratePolicy(w, info); err != nil {
		t.Fatal(err)
	}
	result := w.String()
	if result != expectedWithoutAppID {
		t.Fatalf("received unexpected policy of %s", result)
	}

	w = new(bytes.Buffer)
	info.ApplicationID = "application-id"
	if err := GeneratePolicy(w, info); err != nil {
		t.Fatal(err)
	}
	result = w.String()
	if result != expectedWithAppID {
		t.Fatalf("received unexpected policy of %s", result)
	}
}

var expectedWithoutAppID = `
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
`

var expectedWithAppID = `
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
