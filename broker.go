package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/lager"

	uuid "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/vault/api"
	"github.com/pivotal-cf/brokerapi"
)

const (
	// VaultBrokerName is the name we use for the broker
	VaultBrokerName = "vault"

	// VaultBrokerDescription is the description we use for the broker
	VaultBrokerDescription = "HashiCorp Vault Service Broker"

	// VaultPlanName is the name of our plan, only one supported
	VaultPlanName = "default"

	// VaultPlanDescription is the description of the plan
	VaultPlanDescription = "Secure access to a multi-tenant HashiCorp Vault cluster"

	// VaultPeriodicTTL is the token role periodic TTL.
	VaultPeriodicTTL = 5 * 86400
)

var _ brokerapi.ServiceBroker = (*Broker)(nil)

type Broker struct {
	log    lager.Logger
	client *api.Client

	// mountMutex is the
	mountMutex sync.Mutex

	shutdown     bool
	shutdownCh   chan struct{}
	doneCh       chan struct{}
	shutdownLock sync.Mutex
}

// Start is used to start the broker
func (b *Broker) Start() error {
	b.shutdownLock.Lock()
	defer b.shutdownLock.Unlock()

	// Do nothing if started
	if b.shutdownCh != nil {
		return nil
	}

	// Start the run loop
	b.shutdown = false
	b.shutdownCh = make(chan struct{})
	b.doneCh = make(chan struct{})
	go b.run(b.shutdownCh, b.doneCh)
	return nil
}

// Stop is used to shutdown the broker
func (b *Broker) Stop() error {
	b.shutdownLock.Lock()
	defer b.shutdownLock.Unlock()

	// Do nothing if shutdown
	if b.shutdown {
		return nil
	}

	// Signal shutdown and wait for exit
	b.shutdown = true
	close(b.shutdownCh)
	<-b.doneCh

	// Cleanup
	b.shutdownCh = nil
	b.doneCh = nil
	return nil
}

// run is the long running broker routine
func (b *Broker) run(stopCh chan struct{}, doneCh chan struct{}) {
	defer close(doneCh)
	for {
		select {
		case <-stopCh:
			return
		}
	}
}

func (b *Broker) Services(ctx context.Context) []brokerapi.Service {
	b.log.Debug("broker: providing services catalog")
	brokerID, err := uuid.GenerateUUID()
	if err != nil {
		b.log.Fatal("broker: failed to generate ID", err)
	}
	return []brokerapi.Service{
		brokerapi.Service{
			ID:            brokerID,
			Name:          VaultBrokerName,
			Description:   VaultBrokerDescription,
			Tags:          []string{},
			Bindable:      true,
			PlanUpdatable: false,
			Plans: []brokerapi.ServicePlan{
				brokerapi.ServicePlan{
					ID:          fmt.Sprintf("%s.%s", brokerID, VaultPlanName),
					Name:        VaultPlanName,
					Description: VaultPlanDescription,
					Free:        brokerapi.FreeValue(true),
				},
			},
		},
	}
}

// Provision is used to setup a new instance of Vault tenant. For each
// tenant we create a new Vault policy called "cf-instanceID". This is
// granted access to the service, space, and org contexts. We then create
// a token role called "cf-instanceID" which is periodic. Lastly, we mount
// the backends for the instance, and optionally for the space and org if
// they do not exist yet.
func (b *Broker) Provision(ctx context.Context, instanceID string, details brokerapi.ProvisionDetails, async bool) (brokerapi.ProvisionedServiceSpec, error) {
	b.log.Debug("provisioning new instance", lager.Data{
		"instance-id": instanceID,
		"org-id":      details.OrganizationGUID,
		"space-id":    details.SpaceGUID,
	})

	// Generate the new policy
	var buf bytes.Buffer
	inp := ServicePolicyTemplateInput{
		ServiceID:   instanceID,
		SpaceID:     details.SpaceGUID,
		SpacePolicy: "write",
		OrgID:       details.OrganizationGUID,
		OrgPolicy:   "read",
	}
	if err := GeneratePolicy(&buf, &inp); err != nil {
		b.log.Error("broker: failed to generate policy", err)
		return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("failed to generate policy: %v", err)
	}

	// Create the new policy
	policyName := "cf-" + instanceID
	sys := b.client.Sys()
	b.log.Info(fmt.Sprintf("broker: creating new policy: %s", policyName))
	if err := sys.PutPolicy(policyName, string(buf.Bytes())); err != nil {
		b.log.Error("broker: failed to create policy", err)
		return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("failed to create policy: %v", err)
	}

	// Create the new token role
	path := "/auth/token/roles/cf-" + instanceID
	data := map[string]interface{}{
		"allowed_policies": []string{policyName},
		"period":           VaultPeriodicTTL,
		"renewable":        true,
	}
	b.log.Info(fmt.Sprintf("broker: creating new token role: %s", path))
	if _, err := b.client.Logical().Write(path, data); err != nil {
		b.log.Error("broker: failed to create token role", err)
		return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("failed to create token role: %v", err)
	}

	// Determine the mounts we need
	mounts := map[string]string{
		"/cf/" + details.OrganizationGUID + "/secret": "generic",
		"/cf/" + details.SpaceGUID + "/secret":        "generic",
		"/cf/" + instanceID + "/secret":               "generic",
		"/cf/" + instanceID + "/transit":              "transit",
	}

	// TODO: Mount the backends
	b.log.Info(fmt.Sprintf("broker: setting up mounts: %#v", mounts))
	if mounts != nil {
		b.log.Error("broker: failed to setup mounts", nil)
		return brokerapi.ProvisionedServiceSpec{}, fmt.Errorf("failed to setup mounts: %v", nil)
	}

	// Done
	return brokerapi.ProvisionedServiceSpec{}, nil
}

/*
Broker unmounts all backends under /cf/SvcID/ (BTV)
*/

// Deprovision is used to remove a tenant of Vault. We use this to
// remove all the backends of the tenant, delete the token role, and policy.
func (b *Broker) Deprovision(ctx context.Context, instanceID string, details brokerapi.DeprovisionDetails, async bool) (brokerapi.DeprovisionServiceSpec, error) {
	b.log.Debug("deprovisioning new instance", lager.Data{
		"instance-id": instanceID,
	})

	// TODO: Unmount the backends

	// Delete the token role
	path := "/auth/token/roles/cf-" + instanceID
	b.log.Info(fmt.Sprintf("broker: deleting token role: %s", path))
	if _, err := b.client.Logical().Delete(path); err != nil {
		b.log.Error("broker: failed to delete token role", err)
		return brokerapi.DeprovisionServiceSpec{}, fmt.Errorf("failed to delete token role: %v", err)
	}

	// Delete the token policy
	policyName := "cf-" + instanceID
	b.log.Info(fmt.Sprintf("broker: deleting policy: %s", policyName))
	if err := b.client.Sys().DeletePolicy(policyName); err != nil {
		b.log.Error("broker: failed to delete policy", err)
		return brokerapi.DeprovisionServiceSpec{}, fmt.Errorf("failed to delete policy: %v", err)
	}

	// Done!
	return brokerapi.DeprovisionServiceSpec{}, nil
}

func (b *Broker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	b.log.Debug("binding service", lager.Data{
		"binding-id":  bindingID,
		"instance-id": instanceID,
	})

	binding := brokerapi.Binding{}
	roleName := "cf-" + instanceID

	// Create the secret
	renewable := true
	secret, err := b.client.Auth().Token().CreateWithRole(&api.TokenCreateRequest{
		Policies:       []string{},
		Metadata:       map[string]string{},
		TTL:            "",
		ExplicitMaxTTL: "",
		DisplayName:    "",
		Renewable:      &renewable,
	}, roleName)
	if err != nil {
		b.log.Error("failed creating secret", err)
		return binding, err
	}
	if secret.Auth == nil {
		err = errors.New("secret as no auth")
		b.log.Error("failed creating secret", err)
		return binding, err
	}

	// Ensure the generic secret backend at cf/broker is mounted.
	if err := b.IdempotentMounts(map[string]string{
		"cf/broker": "generic",
	}); err != nil {
		defer b.RevokeAccessor(secret.Auth.Accessor)
		b.log.Error("failed creating mounts", err)
		return binding, err
	}

	// Store the token and metadata in the generic secret backend
	path := "cf/broker/" + instanceID + "/" + bindingID
	if _, err := b.client.Logical().Write(path, map[string]interface{}{
		"token":       nil,
		"last_renew":  nil,
		"expire_time": nil,
	}); err != nil {
		defer b.RevokeAccessor(secret.Auth.Accessor)
		b.log.Error("failed to commit to broker", err)
		return binding, err
	}

	// Save the credentials
	binding.Credentials = nil

	return binding, nil
}

func (b *Broker) Unbind(ctx context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	b.log.Debug("unbinding service", lager.Data{
		"binding-id":  bindingID,
		"instance-id": instanceID,
	})

	return nil
}

func (b *Broker) Update(ctx context.Context, instanceID string, details brokerapi.UpdateDetails, async bool) (brokerapi.UpdateServiceSpec, error) {
	b.log.Debug("updating service", lager.Data{
		"instance-id": instanceID,
	})
	return brokerapi.UpdateServiceSpec{}, nil
}

func (b *Broker) LastOperation(ctx context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	b.log.Debug("returning last operation", lager.Data{
		"instance-id": instanceID,
	})

	return brokerapi.LastOperation{}, nil
}

// RevokeAccessor revokes the given token by accessor.
func (b *Broker) RevokeAccessor(a string) {
	if err := b.client.Auth().Token().RevokeAccessor(a); err != nil {
		b.log.Error("failed revoking accessor", err)
	}
}

// IdempotentMounts takes a list of mounts and their desired paths and mounts the
// backend at that path. The key is the path and the value is the type of
// backend to mount.
func (b *Broker) IdempotentMounts(m map[string]string) error {
	b.mountMutex.Lock()
	defer b.mountMutex.Unlock()
	result, err := b.client.Sys().ListMounts()
	if err != nil {
		return err
	}

	// Strip all leading and trailing things
	mounts := make(map[string]struct{})
	for k, _ := range result {
		k = strings.Trim(k, "/")
		mounts[k] = struct{}{}
	}

	for k, v := range m {
		k = strings.Trim(k, "/")
		if _, ok := mounts[k]; ok {
			continue
		}
		if err := b.client.Sys().Mount(k, &api.MountInput{
			Type: v,
		}); err != nil {
			return err
		}
	}

	return nil
}
