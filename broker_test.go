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

type Environment struct {
	Context          context.Context
	Broker           *Broker
	InstanceID       string
	BindingID        string
	SpaceGUID        string
	OrganizationGUID string
	Async            bool
}

func TestBroker(t *testing.T) {
	env, closer := defaultEnvironment(t)
	defer closer()

	t.Run("TestBroker_Start", env.TestBroker_Start)
	t.Run("TestBroker_Services", env.TestBroker_Services)
	t.Run("TestBroker_Provision", env.TestBroker_Provision)
	t.Run("TestBroker_Bind", env.TestBroker_Bind)
	t.Run("TestBroker_Unbind", env.TestBroker_Unbind)
	t.Run("TestBroker_Deprovision", env.TestBroker_Deprovision)
	t.Run("TestBroker_Update", env.TestBroker_Update)
	t.Run("TestBroker_LastOperation", env.TestBroker_LastOperation)
	t.Run("TestBroker_Stop", env.TestBroker_Stop)
}

func (env *Environment) TestBroker_Start(t *testing.T) {
	if err := env.Broker.Start(); err != nil {
		t.Fatal(err)
	}
}

func (env *Environment) TestBroker_Stop(t *testing.T) {
	if err := env.Broker.Stop(); err != nil {
		t.Fatal(err)
	}
}

func (env *Environment) TestBroker_Services(t *testing.T) {
	services := env.Broker.Services(env.Context)
	if len(services) != 1 {
		t.Fatalf("expected 1 service but received %d", len(services))
	}
}

func (env *Environment) TestBroker_Provision(t *testing.T) {
	details := brokerapi.ProvisionDetails{
		SpaceGUID:        env.SpaceGUID,
		OrganizationGUID: env.OrganizationGUID,
	}
	spec, err := env.Broker.Provision(env.Context, env.InstanceID, details, env.Async)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec, brokerapi.ProvisionedServiceSpec{}) {
		t.Fatalf("%+v differs from %+v", spec, brokerapi.ProvisionedServiceSpec{})
	}
}

func (env *Environment) TestBroker_Deprovision(t *testing.T) {
	spec, err := env.Broker.Deprovision(env.Context, env.InstanceID, brokerapi.DeprovisionDetails{}, env.Async)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec, brokerapi.DeprovisionServiceSpec{}) {
		t.Fatalf("%+v differs from %+v", spec, brokerapi.DeprovisionServiceSpec{})
	}
}

func (env *Environment) TestBroker_Bind(t *testing.T) {
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
}

func (env *Environment) TestBroker_Unbind(t *testing.T) {
	if err := env.Broker.Unbind(env.Context, env.InstanceID, env.BindingID, brokerapi.UnbindDetails{}); err != nil {
		t.Fatal(err)
	}
}

func (env *Environment) TestBroker_Update(t *testing.T) {
	spec, err := env.Broker.Update(env.Context, env.InstanceID, brokerapi.UpdateDetails{}, env.Async)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(spec, brokerapi.UpdateServiceSpec{}) {
		t.Fatalf("%+v differs from %+v", spec, brokerapi.UpdateServiceSpec{})
	}
}

func (env *Environment) TestBroker_LastOperation(t *testing.T) {
	lastOperation, err := env.Broker.LastOperation(env.Context, env.InstanceID, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lastOperation, brokerapi.LastOperation{}) {
		t.Fatalf("%+v differs from %+v", lastOperation, brokerapi.LastOperation{})
	}
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

		default:
			// Some call was received that's not implemented here.
			w.WriteHeader(400)
			b, _ := json.Marshal(r)
			w.Write([]byte(fmt.Sprintf(`{"not_implemented": %s"}`, b)))
			return
		}
	}))

	// To mimic main's behavior as closely as possible,
	// Vault's address is passed to the client via an env variable.
	os.Setenv("VAULT_ADDR", ts.URL)

	client, err := api.NewClient(nil)
	if err != nil {
		t.Fatal(err)
	}

	return &Environment{
		Context: context.Background(),
		Broker: &Broker{
			log:                log.New(os.Stdout, "", 0),
			client:             client,
			serviceID:          DefaultServiceID,
			serviceName:        DefaultServiceName,
			serviceDescription: DefaultServiceDescription,
			planName:           DefaultPlanName,
			planDescription:    DefaultPlanDescription,
			vaultAdvertiseAddr: DefaultVaultAddr,
			vaultRenewToken:    true,
		},
		InstanceID:       "instance-id",
		BindingID:        "binding-id",
		SpaceGUID:        "space-guid",
		OrganizationGUID: "organization-guid",
		Async:            false,
	}, ts.Close
}
