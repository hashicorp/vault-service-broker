package main

import (
	"html/template"
	"io"
)

const (
	// ServicePolicyTemplate is the template used to generate a Vault policy on
	// service create.
	ServicePolicyTemplate string = `
path "cf/{{ .InstanceID }}" {
  capabilities = ["list"]
}

path "cf/{{ .InstanceID }}/*" {
	capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/{{ .SpaceID }}" {
  capabilities = ["list"]
}

path "cf/{{ .SpaceID }}/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/{{ .OrgID }}" {
  capabilities = ["list"]
}

path "cf/{{ .OrgID }}/*" {
  capabilities = ["read", "list"]
}
{{ if ne .ApplicationID "" }}
path "cf/{{ .ApplicationID }}" {
  capabilities = ["list"]
}

path "cf/{{ .ApplicationID }}/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
{{ end -}}
`
)

// ServicePolicyTemplateInput is used as input to the ServicePolicyTemplate.
type ServicePolicyTemplateInput struct {
	// InstanceID is the unique ID of the service instance.
	InstanceID string

	// SpaceID is the unique ID of the space.
	SpaceID string

	// OrgID is the unique ID of the space.
	OrgID string

	// ApplicationID is the unique ID of the service.
	ApplicationID string
}

// GeneratePolicy takes an io.Writer object and template input and renders the
// resulting template into the writer.
func GeneratePolicy(w io.Writer, info *ServicePolicyTemplateInput) error {
	tmpl, err := template.New("service").Parse(ServicePolicyTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, info)
}
