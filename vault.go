package main

import (
	"html/template"
	"io"
)

const (
	// ServicePolicyTemplate is the template used to generate a Vault policy on
	// service create.
	ServicePolicyTemplate string = `
path "cf/{{ .ServiceInstanceGUID }}" {
  capabilities = ["list"]
}

path "cf/{{ .ServiceInstanceGUID }}/*" {
	capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/{{ .SpaceGUID }}" {
  capabilities = ["list"]
}

path "cf/{{ .SpaceGUID }}/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/{{ .OrganizationGUID }}" {
  capabilities = ["list"]
}

path "cf/{{ .OrganizationGUID }}/*" {
  capabilities = ["read", "list"]
}
`

	ApplicationPolicyTemplate = `
path "cf/{{ .ApplicationGUID }}" {
  capabilities = ["list"]
}

path "cf/{{ .ApplicationGUID }}/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
`
)

// GeneratePolicy takes an io.Writer object and template input and renders the
// resulting template into the writer.
func GeneratePolicy(w io.Writer, info *instanceInfo) error {
	policies := ServicePolicyTemplate
	if info.ApplicationGUID != "" {
		policies += ApplicationPolicyTemplate
	}
	tmpl, err := template.New("service").Parse(policies)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, info)
}
