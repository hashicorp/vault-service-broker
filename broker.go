package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/fatih/structs"
	uuid "github.com/hashicorp/go-uuid"
	"github.com/hashicorp/vault/api"
	"github.com/mitchellh/mapstructure"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pkg/errors"
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

type bindingInfo struct {
	Binding       string
	ClientToken   string
	Accessor      string
	LeaseDuration int
	Renew         time.Time
	Expires       time.Time

	timer *time.Timer
}

type Broker struct {
	log    *log.Logger
	client *api.Client

	// mountMutex is used to protect updates to the mount table
	mountMutex sync.Mutex

	// Binds is used to track all the bindings and perform
	// their renewal at (Expiration/2) intervals.
	binds    map[string]*bindingInfo
	bindLock sync.Mutex

	running bool
	runLock sync.Mutex
}

// Start is used to start the broker
func (b *Broker) Start() error {
	b.log.Printf("[INFO] starting broker")

	b.runLock.Lock()
	defer b.runLock.Unlock()

	// Do nothing if started
	if b.running {
		b.log.Printf("[DEBUG] broker is already running")
		return nil
	}

	// Ensure binds is initialized
	b.binds = make(map[string]*bindingInfo)

	// Ensure the generic secret backend at cf/broker is mounted.
	mounts := map[string]string{
		"cf/broker": "generic",
	}
	b.log.Printf("[DEBUG] creating mounts %#v", mounts)
	if err := b.idempotentMount(mounts); err != nil {
		return errors.Wrap(err, "failed to create mounts")
	}

	// Restore timers
	b.log.Printf("[DEBUG] restoring bindings")
	instances, err := b.listDir("cf/broker/")
	if err != nil {
		return errors.Wrap(err, "failed to list instances")
	}
	for _, inst := range instances {
		binds, err := b.listDir("cf/broker/" + inst + "/")
		if err != nil {
			return errors.Wrapf(err, "failed to list binds for instance %q", inst)
		}
		for _, bind := range binds {
			if err := b.restoreBind(inst, bind); err != nil {
				return errors.Wrapf(err, "failed to restore bind %q", bind)
			}
		}
	}

	// Log our restore status
	b.bindLock.Lock()
	b.log.Printf("[INFO] restored %d binds and %d instances",
		len(b.binds), len(instances))
	b.bindLock.Unlock()

	b.running = true
	return nil
}

// listDir is used to list a directory
func (b *Broker) listDir(dir string) ([]string, error) {
	b.log.Printf("[DEBUG] listing directory %q", dir)
	secret, err := b.client.Logical().List(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "listDir %s", dir)
	}
	if secret != nil && len(secret.Data) > 0 {
		keysRaw := secret.Data["keys"].([]string)
		return keysRaw, fmt.Errorf("listDir %s has no secret", dir)
	}
	return nil, nil
}

// restoreBind is used to restore a binding
func (b *Broker) restoreBind(instanceID, bindingID string) error {
	b.log.Printf("[INFO] restoring bind for instance %s for binding %s",
		instanceID, bindingID)

	// Read from Vault
	path := "cf/broker/" + instanceID + "/" + bindingID
	b.log.Printf("[DEBUG] reading bind from %s", path)
	secret, err := b.client.Logical().Read(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read bind info at %q", path)
	}
	if secret == nil {
		return nil
	}

	// Decode the binding info
	info := new(bindingInfo)
	b.log.Printf("[DEBUG] decoding bind data from %s", path)
	if err := mapstructure.Decode(secret.Data, info); err != nil {
		return errors.Wrap(err, "failed to decode binding info")
	}

	// Determine when we should renew
	nextRenew := info.Renew.Add(time.Duration(info.LeaseDuration/2) * time.Second)
	now := time.Now().UTC()

	// Determine when we should first first
	var renewIn time.Duration
	if nextRenew.Before(now) {
		renewIn = 5 * time.Second // Schedule immediate renew
	} else {
		renewIn = nextRenew.Sub(now)
	}

	// Setup Renew timer
	info.timer = time.AfterFunc(renewIn, func() {
		b.handleRenew(info)
	})

	// Store the info
	b.bindLock.Lock()
	b.binds[bindingID] = info
	b.bindLock.Unlock()
	return nil
}

// Stop is used to shutdown the broker
func (b *Broker) Stop() error {
	b.log.Printf("[INFO] stopping broker")

	b.runLock.Lock()
	defer b.runLock.Unlock()

	// Do nothing if shutdown
	if !b.running {
		return nil
	}

	// Stop all the renew timers
	b.bindLock.Lock()
	for _, info := range b.binds {
		info.timer.Stop()
	}
	b.bindLock.Unlock()

	b.running = false
	return nil
}

func (b *Broker) Services(ctx context.Context) []brokerapi.Service {
	b.log.Printf("[INFO] listing services")

	brokerID, err := uuid.GenerateUUID()
	if err != nil {
		b.log.Fatalf("[ERR] uuid generation failed: %s", err)
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
	b.log.Printf("[INFO] provisioning instance %s in %s/%s",
		instanceID, details.OrganizationGUID, details.SpaceGUID)

	// Generate the new policy
	var buf bytes.Buffer
	inp := ServicePolicyTemplateInput{
		ServiceID:   instanceID,
		SpaceID:     details.SpaceGUID,
		SpacePolicy: "write",
		OrgID:       details.OrganizationGUID,
		OrgPolicy:   "read",
	}

	b.log.Printf("[DEBUG] generating policy for %s", instanceID)
	if err := GeneratePolicy(&buf, &inp); err != nil {
		err = errors.Wrapf(err, "failed to generate policy for %s", instanceID)
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	// Create the new policy
	policyName := "cf-" + instanceID
	b.log.Printf("[DEBUG] creating new policy %s", policyName)
	if err := b.client.Sys().PutPolicy(policyName, buf.String()); err != nil {
		err = errors.Wrapf(err, "failed to create policy %s", policyName)
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	// Create the new token role
	path := "/auth/token/roles/cf-" + instanceID
	data := map[string]interface{}{
		"allowed_policies": policyName,
		"period":           VaultPeriodicTTL,
		"renewable":        true,
	}
	b.log.Printf("[DEBUG] creating new token role for %s", path)
	if _, err := b.client.Logical().Write(path, data); err != nil {
		err = errors.Wrapf(err, "failed to create token role for %s", path)
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	// Determine the mounts we need
	mounts := map[string]string{
		"/cf/" + details.OrganizationGUID + "/secret": "generic",
		"/cf/" + details.SpaceGUID + "/secret":        "generic",
		"/cf/" + instanceID + "/secret":               "generic",
		"/cf/" + instanceID + "/transit":              "transit",
	}

	// Mount the backends
	b.log.Printf("[DEBUG] creating mounts %#v", mounts)
	if err := b.idempotentMount(mounts); err != nil {
		err = errors.Wrapf(err, "failed to create mounts %#v", mounts)
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	// Done
	return brokerapi.ProvisionedServiceSpec{}, nil
}

// Deprovision is used to remove a tenant of Vault. We use this to
// remove all the backends of the tenant, delete the token role, and policy.
func (b *Broker) Deprovision(ctx context.Context, instanceID string, details brokerapi.DeprovisionDetails, async bool) (brokerapi.DeprovisionServiceSpec, error) {
	b.log.Printf("[INFO] deprovisioning %s", instanceID)

	// Unmount the backends
	mounts := []string{
		"/cf/" + instanceID + "/secret",
		"/cf/" + instanceID + "/transit",
	}
	b.log.Printf("[DEBUG] removing mounts %#v", mounts)
	if err := b.idempotentUnmount(mounts); err != nil {
		err = errors.Wrap(err, "failed to remove mounts")
		return brokerapi.DeprovisionServiceSpec{}, err
	}

	// Delete the token role
	path := "/auth/token/roles/cf-" + instanceID
	b.log.Printf("[DEBUG] deleting token role %s", path)
	if _, err := b.client.Logical().Delete(path); err != nil {
		err = errors.Wrapf(err, "failed to delete token role %s", path)
		return brokerapi.DeprovisionServiceSpec{}, err
	}

	// Delete the token policy
	policyName := "cf-" + instanceID
	b.log.Printf("[DEBUG] deleting policy %s", policyName)
	if err := b.client.Sys().DeletePolicy(policyName); err != nil {
		err = errors.Wrapf(err, "failed to delete policy %s", policyName)
		return brokerapi.DeprovisionServiceSpec{}, err
	}

	// Done!
	return brokerapi.DeprovisionServiceSpec{}, nil
}

// Bind is used to attach a tenant of Vault to an application in CloudFoundry.
// This should create a credential that is used to authorize against Vault.
func (b *Broker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	b.log.Printf("[INFO] binding service %s to instance %s",
		bindingID, instanceID)

	binding := brokerapi.Binding{}
	roleName := "cf-" + instanceID

	// Create the token
	renewable := true
	b.log.Printf("[DEBUG] creating token with role %s", roleName)
	secret, err := b.client.Auth().Token().CreateWithRole(&api.TokenCreateRequest{
		Policies:    []string{roleName},
		Metadata:    map[string]string{"cf-instance-id": instanceID, "cf-binding-id": bindingID},
		DisplayName: "cf-bind-" + bindingID,
		Renewable:   &renewable,
	}, roleName)
	if err != nil {
		err = errors.Wrapf(err, "failed to create token with role %s", roleName)
		return binding, err
	}
	if secret.Auth == nil {
		err = fmt.Errorf("secret with role %s has no auth", roleName)
		return binding, err
	}

	// Create a binding info object
	now := time.Now().UTC()
	expires := now.Add(time.Duration(secret.Auth.LeaseDuration) * time.Second)
	info := &bindingInfo{
		Binding:       bindingID,
		ClientToken:   secret.Auth.ClientToken,
		Accessor:      secret.Auth.Accessor,
		LeaseDuration: secret.Auth.LeaseDuration,
		Renew:         now,
		Expires:       expires,
	}

	// Store the token and metadata in the generic secret backend
	path := "cf/broker/" + instanceID + "/" + bindingID
	b.log.Printf("[DEBUG] storing metadata at %s", path)
	if _, err := b.client.Logical().Write(path, structs.Map(info)); err != nil {
		a := secret.Auth.Accessor
		if err := b.client.Auth().Token().RevokeAccessor(a); err != nil {
			b.log.Printf("[WARN] failed to revoke accessor %s", a)
		}
		return binding, errors.Wrapf(err, "failed to commit to broken for %s", path)
	}

	// Setup Renew timer
	renew := time.Duration(secret.Auth.LeaseDuration) / 2 * time.Second
	info.timer = time.AfterFunc(renew, func() {
		b.handleRenew(info)
	})

	// Store the info
	b.bindLock.Lock()
	b.binds[bindingID] = info
	b.bindLock.Unlock()

	// Save the credentials
	binding.Credentials = map[string]string{
		"vault_token_accessor": secret.Auth.Accessor,
		"vault_token":          secret.Auth.ClientToken,
		"vault_path":           "cf/" + instanceID,
	}
	return binding, nil
}

// Unbind is used to detach an applicaiton from a tenant in Vault.
func (b *Broker) Unbind(ctx context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	b.log.Printf("[INFO] unbinding service %s for instance %s",
		bindingID, instanceID)

	// Read the binding info
	path := "cf/broker/" + instanceID + "/" + bindingID
	b.log.Printf("[DEBUG] reading %s", path)
	secret, err := b.client.Logical().Read(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read binding info for %s", path)
	}
	if secret == nil {
		return fmt.Errorf("missing bind info for unbind for %s", path)
	}

	// Decode the binding info
	var info bindingInfo
	b.log.Printf("[DEBUG] decoding binding info for %s", path)
	if err := mapstructure.Decode(secret.Data, &info); err != nil {
		return errors.Wrapf(err, "failed to decode binding info for %s", path)
	}

	// Revoke the token
	a := info.Accessor
	b.log.Printf("[DEBUG] revoking accessor %s for path %s", a, path)
	if err := b.client.Auth().Token().RevokeAccessor(a); err != nil {
		return errors.Wrapf(err, "failed to revoke accessor %s", a)
	}

	// Delete the binding info
	b.log.Printf("[DEBUG] deleting binding info at %s", path)
	if _, err := b.client.Logical().Delete(path); err != nil {
		return errors.Wrapf(err, "failed to delete binding info at %s", path)
	}

	// Delete the bind if it exists, stop the renew timer
	b.bindLock.Lock()
	existing, ok := b.binds[bindingID]
	if ok {
		delete(b.binds, bindingID)
		existing.timer.Stop()
	}
	b.bindLock.Unlock()

	// Done
	return nil
}

// Not implemented, only used for multiple plans
func (b *Broker) Update(ctx context.Context, instanceID string, details brokerapi.UpdateDetails, async bool) (brokerapi.UpdateServiceSpec, error) {
	b.log.Printf("[INFO] updating service for instance %s", instanceID)
	return brokerapi.UpdateServiceSpec{}, nil
}

// Not implemented, only used for async
func (b *Broker) LastOperation(ctx context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	b.log.Printf("[INFO] returning last operation for instance %s", instanceID)
	return brokerapi.LastOperation{}, nil
}

// idempotentMount takes a list of mounts and their desired paths and mounts the
// backend at that path. The key is the path and the value is the type of
// backend to mount.
func (b *Broker) idempotentMount(m map[string]string) error {
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

// idempotentUnmount takes a list of mount paths and removes them if and only
// if they currently exist.
func (b *Broker) idempotentUnmount(l []string) error {
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

	for _, k := range l {
		k = strings.Trim(k, "/")
		if _, ok := mounts[k]; !ok {
			continue
		}
		if err := b.client.Sys().Unmount(k); err != nil {
			return err
		}
	}
	return nil
}

// handleRenew is used to handle renewing a token
func (b *Broker) handleRenew(info *bindingInfo) {
	b.log.Printf("[DEBUG] renewing token")

	// Attempt to renew the token
	auth := b.client.Auth().Token()
	secret, err := auth.Renew(info.ClientToken, 0)
	if err != nil {
		b.log.Fatalf("[ERR] token renewal failed: %s", err)
	}

	// Setup Renew timer
	var renew time.Duration
	if secret != nil {
		renew = time.Duration(secret.Auth.LeaseDuration) / 2 * time.Second
	} else {
		renew = 30 * time.Second
	}
	info.timer = time.AfterFunc(renew, func() {
		b.handleRenew(info)
	})
}
