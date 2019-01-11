package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/pivotal-cf/brokerapi"
)

func TestBroker_Start_Stop(t *testing.T) {
	env, closer := defaultEnvironment(t)
	defer closer()

	if err := env.Broker.Start(); err != nil {
		t.Fatal(err)
	}
	if err := env.Broker.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestBroker_Services(t *testing.T) {
	env, closer := defaultEnvironment(t)
	defer closer()

	services := env.Broker.Services(env.Context)
	if len(services) != 1 {
		t.Fatalf("expected 1 service but received %d", len(services))
	}
}

func TestBroker_Provision_Deprovision(t *testing.T) {
	env, closer := defaultEnvironment(t)
	defer closer()

	details := brokerapi.ProvisionDetails{
		SpaceGUID:        env.SpaceGUID,
		OrganizationGUID: env.OrganizationGUID,
	}
	provSpec, err := env.Broker.Provision(env.Context, env.InstanceID, details, env.Async)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(provSpec, brokerapi.ProvisionedServiceSpec{}) {
		t.Fatalf("%+v differs from %+v", provSpec, brokerapi.ProvisionedServiceSpec{})
	}

	deProvSpec, err := env.Broker.Deprovision(env.Context, env.InstanceID, brokerapi.DeprovisionDetails{}, env.Async)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(deProvSpec, brokerapi.DeprovisionServiceSpec{}) {
		t.Fatalf("%+v differs from %+v", deProvSpec, brokerapi.DeprovisionServiceSpec{})
	}
}

func TestBroker_Bind_Unbind(t *testing.T) {
	env, closer := defaultEnvironment(t)
	defer closer()

	// Seed the broker with the results of provisioning an instance
	// so binding can succeed.
	env.Broker.instances["instance-id"] = &instanceInfo{
		SpaceGUID:        "space-guid",
		OrganizationGUID: "organization-guid",
	}

	binding, err := env.Broker.Bind(env.Context, env.InstanceID, env.BindingID, brokerapi.BindDetails{})
	if err != nil {
		t.Fatal(err)
	}
	if binding.SyslogDrainURL != "" {
		t.Fatalf("expected empty SyslogDrainURL but received %s", binding.SyslogDrainURL)
	}
	if binding.RouteServiceURL != "" {
		t.Fatalf("expected empty RouteServiceURL but received %s", binding.RouteServiceURL)
	}
	if len(binding.VolumeMounts) != 0 {
		t.Fatalf("expected no VolumeMounts but received %+v", binding.VolumeMounts)
	}
	credMap, ok := binding.Credentials.(map[string]interface{})
	if !ok {
		t.Fatalf("expected a credential map but received %+v", binding.Credentials)
	}
	shared, ok := credMap["backends_shared"]
	if !ok {
		t.Fatalf("expected backends_shared but they're not in %+v", credMap)
	}
	sharedMap, ok := shared.(map[string]interface{})
	if !ok {
		t.Fatalf("expected a backends_shared map but received %+v", shared)
	}
	if sharedMap["organization"] != "cf/organization-guid/secret" {
		t.Fatalf("expected cf/space-guid/secret but received %s", sharedMap["organization"])
	}
	if sharedMap["space"] != "cf/space-guid/secret" {
		t.Fatalf("expected cf/space-guid/secret but received %s", sharedMap["space"])
	}

	if err := env.Broker.Unbind(env.Context, env.InstanceID, env.BindingID, brokerapi.UnbindDetails{}); err != nil {
		t.Fatal(err)
	}
}

func TestBroker_Update(t *testing.T) {
	env, closer := defaultEnvironment(t)
	defer closer()

	spec, err := env.Broker.Update(env.Context, env.InstanceID, brokerapi.UpdateDetails{}, env.Async)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec, brokerapi.UpdateServiceSpec{}) {
		t.Fatalf("%+v differs from %+v", spec, brokerapi.UpdateServiceSpec{})
	}
}

func TestBroker_LastOperation(t *testing.T) {
	env, closer := defaultEnvironment(t)
	defer closer()

	lastOperation, err := env.Broker.LastOperation(env.Context, env.InstanceID, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lastOperation, brokerapi.LastOperation{}) {
		t.Fatalf("%+v differs from %+v", lastOperation, brokerapi.LastOperation{})
	}
}

type Environment struct {
	Context          context.Context
	Broker           *Broker
	InstanceID       string
	BindingID        string
	SpaceGUID        string
	OrganizationGUID string
	Async            bool
}

func defaultEnvironment(t *testing.T) (*Environment, func()) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		reqURL := r.URL.String()

		switch {

		// The following auth calls are all for the token auth engine.
		case reqURL == "/v1/auth/token/renew-self" && r.Method == "PUT":
			w.WriteHeader(200)
			w.Write([]byte(`{
				"auth": {
					"client_token": "ABCD",
					"policies": [
						"web",
						"stage"
					],
					"metadata": {
						"user": "armon"
					},
					"lease_duration": 3600,
					"renewable": true
				}
			}`))
			return

		case reqURL == "/v1/auth/token/revoke-accessor" && r.Method == "POST":
			w.WriteHeader(204)
			return

		case reqURL == "/v1/auth/token/create/cf-instance-id" && r.Method == "POST":
			w.WriteHeader(200)
			w.Write([]byte(`{
				"auth": {
					"client_token": "ABCD",
					"policies": [
						"web",
						"stage"
					],
					"metadata": {
						"user": "armon"
					},
					"lease_duration": 3600,
					"renewable": true
				}
			}`))
			return

		case reqURL == "/v1/auth/token/roles/cf-instance-id" && r.Method == "PUT":
			w.WriteHeader(200)
			w.Write([]byte(`{
				"auth": null,
				"data": {
					"keys": ["foo", "foo/"]
				},
				"lease_duration": 2764800,
				"lease_id": "",
				"renewable": false
			}`))
			return

		case reqURL == "/v1/auth/token/roles/cf-instance-id" && r.Method == "DELETE":
			w.WriteHeader(204)
			return

		// The following calls to cf/broker are all for the generic KV store (v1).
		case reqURL == "/v1/cf/broker?list=true" && r.Method == "GET":
			w.WriteHeader(200)
			w.Write([]byte(`{
				"auth": null,
				"data": {
					"keys": ["foo", "foo/"]
				},
				"lease_duration": 2764800,
				"lease_id": "",
				"renewable": false
			}`))
			return

		case reqURL == "/v1/cf/broker/foo" && r.Method == "GET":
			w.WriteHeader(200)
			w.Write([]byte(`{
				"auth": null,
				"data": {
					"json": "{\"OrganizationGUID\": \"organization-guid\", \"SpaceGUID\": \"space-guid\"}"
				},
				"lease_duration": 2764800,
				"lease_id": "",
				"renewable": false
			}`))
			return

		case reqURL == "/v1/cf/broker/foo?list=true" && r.Method == "GET":
			w.WriteHeader(200)
			w.Write([]byte(`{
				"auth": null,
				"data": {
					"keys": ["foo", "foo/"]
				},
				"lease_duration": 2764800,
				"lease_id": "",
				"renewable": false
			}`))
			return

		case reqURL == "/v1/cf/broker/foo/foo" && r.Method == "GET":
			w.WriteHeader(200)
			w.Write([]byte(`{
				"auth": null,
				"data": {
					"json": "{\"OrganizationGUID\": \"organization-guid\", \"SpaceGUID\": \"space-guid\"}"
				},
				"lease_duration": 2764800,
				"lease_id": "",
				"renewable": false
			}`))
			return

		case reqURL == "/v1/cf/broker/instance-id" && r.Method == "PUT":
			w.WriteHeader(204)
			return

		case reqURL == "/v1/cf/broker/instance-id" && r.Method == "DELETE":
			w.WriteHeader(204)
			return

		case reqURL == "/v1/cf/broker/instance-id/binding-id" && r.Method == "PUT":
			w.WriteHeader(204)
			return

		case reqURL == "/v1/cf/broker/instance-id/binding-id" && r.Method == "GET":
			w.WriteHeader(200)
			w.Write([]byte(`{
				"auth": null,
				"data": {
					"json": "{\"OrganizationGUID\": \"organization-guid\", \"SpaceGUID\": \"space-guid\"}"
				},
				"lease_duration": 2764800,
				"lease_id": "",
				"renewable": false
			}`))
			return

		case reqURL == "/v1/cf/broker/instance-id/binding-id" && r.Method == "DELETE":
			w.WriteHeader(204)
			return

		// This call is for listing mounts themselves.
		case reqURL == "/v1/sys/mounts" && r.Method == "GET":
			w.WriteHeader(200)
			w.Write([]byte(`{
				"aws": {
					"type": "aws",
					"description": "AWS keys",
					"config": {
						"default_lease_ttl": 0,
						"max_lease_ttl": 0,
						"force_no_cache": false,
						"seal_wrap": false
					}
				},
				"sys": {
					"type": "system",
					"description": "system endpoint",
					"config": {
						"default_lease_ttl": 0,
						"max_lease_ttl": 0,
						"force_no_cache": false,
						"seal_wrap": false
					}
				}
			}`))
			return

		// These posts provide configs to the given endpoints, configs like:
		// {"config":{"default_lease_ttl":"","force_no_cache":false,"max_lease_ttl":""},"description":"","local":false,"type":"generic"}
		case reqURL == "/v1/sys/mounts/cf/broker" && r.Method == "POST":
			w.WriteHeader(204)
			return

		case reqURL == "/v1/sys/mounts/cf/instance-id/secret" && r.Method == "POST":
			w.WriteHeader(204)
			return

		case reqURL == "/v1/sys/mounts/cf/instance-id/transit" && r.Method == "POST":
			w.WriteHeader(204)
			return

		case reqURL == "/v1/sys/mounts/cf/organization-guid/secret" && r.Method == "POST":
			w.WriteHeader(204)
			return

		case reqURL == "/v1/sys/mounts/cf/space-guid/secret" && r.Method == "POST":
			w.WriteHeader(204)
			return

		case reqURL == "/v1/sys/policy/cf-instance-id" && r.Method == "PUT":
			w.WriteHeader(204)
			return

		case reqURL == "/v1/sys/policy/cf-instance-id" && r.Method == "DELETE":
			w.WriteHeader(204)
			return

		case reqURL == "/v1/auth/token/lookup-self" && r.Method == "GET":
			w.WriteHeader(200)
			w.Write([]byte(`{
				"data": {
					"accessor": "8609694a-cdbc-db9b-d345-e782dbb562ed",
					"creation_time": 1523979354,
					"creation_ttl": 2764800,
					"display_name": "ldap2-tesla",
					"entity_id": "7d2e3179-f69b-450c-7179-ac8ee8bd8ca9",
					"expire_time": null,
					"explicit_max_ttl": 0,
					"id": "cf64a70f-3a12-3f6c-791d-6cef6d390eed",
					"identity_policies": [
						"dev-group-policy"
					],
					"issue_time": "2018-04-17T11:35:54.466476078-04:00",
					"meta": {
						"username": "tesla"
					},
					"num_uses": 0,
					"orphan": true,
					"path": "auth/ldap2/login/tesla",
					"policies": [
						"default",
						"testgroup2-policy"
					],
					"renewable": true,
					"ttl": 2764790
				}
			}`))
			return

		default:
			// Some call was received that's not implemented here.
			w.WriteHeader(400)
			b, _ := json.Marshal(r)
			w.Write([]byte(fmt.Sprintf(`{"not_implemented": "%s"}`, b)))
			return
		}
	}))

	// To mimic main's behavior as closely as possible,
	// Vault's address is passed to the vaultClient via an env variable.
	os.Setenv("VAULT_ADDR", ts.URL)

	client, err := api.NewClient(nil)
	if err != nil {
		t.Fatal(err)
	}

	return &Environment{
		Context: context.Background(),
		Broker: &Broker{
			log:                log.New(os.Stdout, "", 0),
			vaultClient:        client,
			serviceID:          "0654695e-0760-a1d4-1cad-5dd87b75ed99",
			serviceName:        "hashicorp-vault",
			serviceDescription: "HashiCorp Vault Service Broker",
			planName:           "shared",
			planDescription:    "Secure access to Vault's storage and transit backends",
			vaultAdvertiseAddr: "https://127.0.0.1:8200",
			vaultRenewToken:    true,
			instances:          make(map[string]*instanceInfo),
			binds:              make(map[string]*bindingInfo),
		},
		InstanceID:       "instance-id",
		BindingID:        "binding-id",
		SpaceGUID:        "space-guid",
		OrganizationGUID: "organization-guid",
		Async:            false,
	}, ts.Close
}
