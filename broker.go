package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pkg/errors"
)

const (
	// VaultPlanName is the name of our plan, only one supported
	VaultPlanName = "shared"

	// VaultPlanDescription is the default description.
	VaultPlanDescription = "Secure access to Vault's storage and transit backends"

	// VaultPeriodicTTL is the token role periodic TTL.
	VaultPeriodicTTL = 5 * 86400
)

var _ brokerapi.ServiceBroker = (*Broker)(nil)

type bindingInfo struct {
	Organization  string
	Space         string
	Binding       string
	ClientToken   string
	Accessor      string
	LeaseDuration int
	Renew         time.Time
	Expires       time.Time

	timer *time.Timer
}

type instanceInfo struct {
	OrganizationGUID string
	SpaceGUID        string
}

type Broker struct {
	log    *log.Logger
	client *api.Client

	// service-specific customization
	serviceID          string
	serviceName        string
	serviceDescription string
	serviceTags        []string

	// vaultAdvertiseAddr is the address where Vault should be advertised to
	// clients.
	vaultAdvertiseAddr string

	// vaultRenewToken toggles whether the broker should renew the supplied token.
	vaultRenewToken bool

	// mountMutex is used to protect updates to the mount table
	mountMutex sync.Mutex

	// Binds is used to track all the bindings and perform
	// their renewal at (Expiration/2) intervals.
	binds    map[string]*bindingInfo
	bindLock sync.Mutex

	// instances is used to map instances to their space and org GUID.
	instances     map[string]*instanceInfo
	instancesLock sync.Mutex

	running bool
	runLock sync.Mutex

	stopCh chan struct{}
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

	// Create the stop channel
	b.stopCh = make(chan struct{}, 1)

	// Start background renewal
	if b.vaultRenewToken {
		go b.renewVaultToken()
	}

	// Ensure binds is initialized
	if b.binds == nil {
		b.binds = make(map[string]*bindingInfo)
	}

	// Ensure instances is initialized
	if b.instances == nil {
		b.instances = make(map[string]*instanceInfo)
	}

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
		inst = strings.Trim(inst, "/")

		if err := b.restoreInstance(inst); err != nil {
			return errors.Wrapf(err, "failed to restore instance data for %q", inst)
		}

		binds, err := b.listDir("cf/broker/" + inst + "/")
		if err != nil {
			return errors.Wrapf(err, "failed to list binds for instance %q", inst)
		}

		for _, bind := range binds {
			bind = strings.Trim(bind, "/")
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

// restoreInstance restores the data for the instance by the given ID.
func (b *Broker) restoreInstance(instanceID string) error {
	b.log.Printf("[INFO] restoring info for instance %s", instanceID)

	path := "cf/broker/" + instanceID

	secret, err := b.client.Logical().Read(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read instance info at %q", path)
	}
	if secret == nil || len(secret.Data) == 0 {
		b.log.Printf("[INFO] restoreInstance %s has no secret data", path)
		return nil
	}

	// Decode the binding info
	b.log.Printf("[DEBUG] decoding bind data from %s", path)
	info, err := decodeInstanceInfo(secret.Data)
	if err != nil {
		return errors.Wrapf(err, "failed to decode instance info for %s", path)
	}

	// Store the info
	b.instancesLock.Lock()
	b.instances[instanceID] = info
	b.instancesLock.Unlock()

	return nil
}

// listDir is used to list a directory
func (b *Broker) listDir(dir string) ([]string, error) {
	b.log.Printf("[DEBUG] listing directory %q", dir)
	secret, err := b.client.Logical().List(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "listDir %s", dir)
	}
	if secret == nil || len(secret.Data) == 0 {
		b.log.Printf("[INFO] listDir %s has no secret data", dir)
		return nil, nil
	}

	keysRaw, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("listDir %s keys are not []interface{}", dir)
	}
	keys := make([]string, len(keysRaw))
	for i, v := range keysRaw {
		typed, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("listDir %s key %q is not string", dir, v)
		}
		keys[i] = typed
	}

	return keys, nil
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
	if secret == nil || len(secret.Data) == 0 {
		b.log.Printf("[INFO] restoreBind %s has no secret data", path)
		return nil
	}

	// Decode the binding info
	b.log.Printf("[DEBUG] decoding bind data from %s", path)
	info, err := decodeBindingInfo(secret.Data)
	if err != nil {
		return errors.Wrapf(err, "failed to decode binding info for %s", path)
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
	return []brokerapi.Service{
		brokerapi.Service{
			ID:            b.serviceID,
			Name:          b.serviceName,
			Description:   b.serviceDescription,
			Tags:          b.serviceTags,
			Bindable:      true,
			PlanUpdatable: false,
			Plans: []brokerapi.ServicePlan{
				brokerapi.ServicePlan{
					ID:          fmt.Sprintf("%s.%s", b.serviceID, VaultPlanName),
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

	// Generate instance info
	info := &instanceInfo{
		OrganizationGUID: details.OrganizationGUID,
		SpaceGUID:        details.SpaceGUID,
	}
	payload, err := json.Marshal(info)
	if err != nil {
		err = errors.Wrap(err, "failed to encode instance json")
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	// Store the token and metadata in the generic secret backend
	instancePath := "cf/broker/" + instanceID
	b.log.Printf("[DEBUG] storing instance metadata at %s", instancePath)
	if _, err := b.client.Logical().Write(instancePath, map[string]interface{}{
		"json": string(payload),
	}); err != nil {
		err = errors.Wrapf(err, "failed to commit instance %s", instancePath)
		return brokerapi.ProvisionedServiceSpec{}, err
	}

	// Save the instance
	b.log.Printf("[DEBUG] saving instance %s to cache", instanceID)
	b.instancesLock.Lock()
	b.instances[instanceID] = info
	b.instancesLock.Unlock()

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

	// Delete the instance info
	instancePath := "cf/broker/" + instanceID
	b.log.Printf("[DEBUG] deleting instance info at %s", instancePath)
	if _, err := b.client.Logical().Delete(instancePath); err != nil {
		err = errors.Wrapf(err, "failed to delete instance info at %s", instancePath)
		return brokerapi.DeprovisionServiceSpec{}, err
	}

	// Delete the instance from the map
	b.log.Printf("[DEBUG] removing instance %s from cache", instanceID)
	b.instancesLock.Lock()
	delete(b.instances, instanceID)
	b.instancesLock.Unlock()

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

	// Get the instance for this instanceID
	b.log.Printf("[DEBUG] looking up instance %s from cache", instanceID)
	b.instancesLock.Lock()
	instance, ok := b.instances[instanceID]
	b.instancesLock.Unlock()
	if !ok {
		return binding, fmt.Errorf("no instance exists with ID %s", instanceID)
	}

	// Create a binding info object
	now := time.Now().UTC()
	expires := now.Add(time.Duration(secret.Auth.LeaseDuration) * time.Second)
	info := &bindingInfo{
		Organization:  instance.OrganizationGUID,
		Space:         instance.SpaceGUID,
		Binding:       bindingID,
		ClientToken:   secret.Auth.ClientToken,
		Accessor:      secret.Auth.Accessor,
		LeaseDuration: secret.Auth.LeaseDuration,
		Renew:         now,
		Expires:       expires,
	}
	data, err := json.Marshal(info)
	if err != nil {
		return binding, errors.Wrap(err, "failed to encode binding json")
	}

	// Store the token and metadata in the generic secret backend
	path := "cf/broker/" + instanceID + "/" + bindingID
	b.log.Printf("[DEBUG] storing binding metadata at %s", path)
	if _, err := b.client.Logical().Write(path, map[string]interface{}{
		"json": string(data),
	}); err != nil {
		a := secret.Auth.Accessor
		if err := b.client.Auth().Token().RevokeAccessor(a); err != nil {
			b.log.Printf("[WARN] failed to revoke accessor %s", a)
		}
		return binding, errors.Wrapf(err, "failed to commit binding %s", path)
	}

	// Setup Renew timer
	renew := time.Duration(secret.Auth.LeaseDuration) / 2 * time.Second
	info.timer = time.AfterFunc(renew, func() {
		b.handleRenew(info)
	})

	// Store the info
	b.log.Printf("[DEBUG] saving bind %s to cache", bindingID)
	b.bindLock.Lock()
	b.binds[bindingID] = info
	b.bindLock.Unlock()

	// Save the credentials
	binding.Credentials = map[string]interface{}{
		"address": b.vaultAdvertiseAddr,
		"auth": map[string]interface{}{
			"accessor": secret.Auth.Accessor,
			"token":    secret.Auth.ClientToken,
		},
		"backends": map[string]interface{}{
			"generic": "cf/" + instanceID + "/secret",
			"transit": "cf/" + instanceID + "/transit",
		},
		"backends_shared": map[string]interface{}{
			"organization": "cf/" + instance.OrganizationGUID + "/secret",
			"space":        "cf/" + instance.SpaceGUID + "/secret",
		},
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
	if secret == nil || len(secret.Data) == 0 {
		return fmt.Errorf("missing bind info for unbind for %s", path)
	}

	// Decode the binding info
	b.log.Printf("[DEBUG] decoding binding info for %s", path)
	info, err := decodeBindingInfo(secret.Data)
	if err != nil {
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
	b.log.Printf("[DEBUG] removing binding %s from cache", bindingID)
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
	b.log.Printf("[DEBUG] renewing token with accessor %s", info.Accessor)

	// If we are past the expiration time, no need to renew anymore
	if time.Now().UTC().After(info.Expires) {
		b.log.Printf("[WARN] aborting renewing expired token with accessor %s", info.Accessor)
		return
	}

	// Attempt to renew the token; we first create a new client because we want
	// to use RenewSelf and that requires setting the client token
	client, err := api.NewClient(nil)
	if err != nil {
		b.log.Printf("[ERR] unable to create new API client for renewal for accessor %s: %s",
			info.Accessor, err)
		return
	}
	client.SetToken(info.ClientToken)
	auth := client.Auth().Token()
	secret, err := auth.RenewSelf(0)
	if err != nil {
		b.log.Printf("[ERR] token renewal failed for accessor %s: %s",
			info.Accessor, err)
	}

	// Setup Renew timer
	var renew time.Duration
	if secret != nil && secret.Auth != nil {
		// TODO(armon): Should we update and persist the binding info?
		//now := time.Now().UTC()
		//expires := now.Add(time.Duration(secret.Auth.LeaseDuration) * time.Second)
		//info.LeaseDuration = secret.Auth.LeaseDuration
		//info.Renew = now
		//info.Expires = expires

		renew = time.Duration(secret.Auth.LeaseDuration) / 2 * time.Second
	} else {
		renew = 60 * time.Second
	}
	info.timer = time.AfterFunc(renew, func() {
		b.handleRenew(info)
	})
}

// renewVaultToken infinitely renews the brokers Vault token until stopped. This
// is designed to run as a background goroutine.
func (b *Broker) renewVaultToken() {
	for {
		secret, err := b.client.Auth().Token().LookupSelf()
		if err != nil {
			b.log.Printf("[ERR] renew-token: error renewing vault token: %s", err)
		}
		if secret == nil || len(secret.Data) == 0 {
			b.log.Printf("[ERR] renew-token: secret has no data: %s", err)
		}

		var dur time.Duration
		if secret.LeaseDuration > 0 {
			dur = time.Duration(secret.LeaseDuration) * time.Second
		} else {
			// Probably periodic
			ttl, ok := secret.Data["ttl"]
			if ok {
				switch ttl.(type) {
				case int, int64:
					dur = time.Duration(ttl.(int64)) * time.Second
				case string:
					dur, _ = time.ParseDuration(ttl.(string) + "s")
				case json.Number:
					dur, _ = time.ParseDuration(string(ttl.(json.Number)) + "s")
				}
			}
		}

		// Cut in half for safety buffer on renewal
		dur = dur / 2.0

		// Make sure we have a sane duration. We might not have a duration if Vault
		// is down or if some of the parsing above went awry.
		if dur == 0 {
			dur = 5 * time.Minute
		}

		// If we have a short duration, set it back to a second to prevent a
		// hot-loop from occurring.
		if dur < 1*time.Second {
			b.log.Printf("[WARN] dur %s is less than 1s, resetting", dur)
			dur = 1 * time.Second
		}

		b.log.Printf("[INFO] sleeping for %s", dur)

		select {
		case <-b.stopCh:
			return
		case <-time.After(dur):
			b.log.Printf("[INFO] renewing vault token")
			if _, err := b.client.Auth().Token().RenewSelf(0); err != nil {
				b.log.Printf("[WARN] failed to renew token: %s", err)
			}
		}
	}
}

func decodeBindingInfo(m map[string]interface{}) (*bindingInfo, error) {
	data, ok := m["json"]
	if !ok {
		return nil, fmt.Errorf("missing 'json' key")
	}

	typed, ok := data.(string)
	if !ok {
		return nil, fmt.Errorf("json data is %T, not string", data)
	}

	var info bindingInfo
	if err := json.Unmarshal([]byte(typed), &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func decodeInstanceInfo(m map[string]interface{}) (*instanceInfo, error) {
	data, ok := m["json"]
	if !ok {
		return nil, fmt.Errorf("missing 'json' key")
	}

	typed, ok := data.(string)
	if !ok {
		return nil, fmt.Errorf("json data is %T, not string", data)
	}

	var info instanceInfo
	if err := json.Unmarshal([]byte(typed), &info); err != nil {
		return nil, err
	}
	return &info, nil
}
