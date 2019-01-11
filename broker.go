package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/pivotal-cf/brokerapi"
	"github.com/pkg/errors"
)

const (
	// VaultPeriodicTTL is the token role periodic TTL.
	VaultPeriodicTTL = 5 * 24 * 60 * 60
)

// Ensure we implement the broker API
var _ brokerapi.ServiceBroker = (*Broker)(nil)

type bindingInfo struct {
	Organization string
	Space        string
	Binding      string
	ClientToken  string
	Accessor     string
	stopCh       chan struct{}
}

type instanceInfo struct {
	OrganizationGUID string
	SpaceGUID        string
}

type Broker struct {
	log         *log.Logger
	vaultClient *api.Client

	// service-specific customization
	serviceID          string
	serviceName        string
	serviceDescription string
	serviceTags        []string

	// plan-specific customization
	planName        string
	planDescription string

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

	// stopLock, stopped, and stopCh are used to control the stopping behavior of
	// the broker.
	stopLock sync.Mutex
	running  bool
	stopCh   chan struct{}
}

// Start is used to start the broker
func (b *Broker) Start() error {
	b.log.Printf("[INFO] starting broker")

	b.stopLock.Lock()
	defer b.stopLock.Unlock()

	// Do nothing if started
	if b.running {
		b.log.Printf("[DEBUG] broker is already running")
		return nil
	}

	// Create the stop channel
	b.stopCh = make(chan struct{})

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
	b.log.Printf("[DEBUG] creating mounts %s", mapToKV(mounts, ", "))
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

	secret, err := b.vaultClient.Logical().Read(path)
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
	secret, err := b.vaultClient.Logical().List(dir)
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
	secret, err := b.vaultClient.Logical().Read(path)
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

	// Start a renewer for this token
	info.stopCh = make(chan struct{})
	go b.renewAuth(info.ClientToken, info.Accessor, info.stopCh)

	// Store the info
	b.bindLock.Lock()
	b.binds[bindingID] = info
	b.bindLock.Unlock()
	return nil
}

// Stop is used to shutdown the broker
func (b *Broker) Stop() error {
	b.log.Printf("[INFO] stopping broker")

	b.stopLock.Lock()
	defer b.stopLock.Unlock()

	// Do nothing if shutdown
	if !b.running {
		return nil
	}

	// Close the stop channel and mark as stopped
	close(b.stopCh)
	b.running = false
	return nil
}

func (b *Broker) Services(ctx context.Context) []brokerapi.Service {
	b.log.Printf("[INFO] listing services")
	return []brokerapi.Service{
		{
			ID:            b.serviceID,
			Name:          b.serviceName,
			Description:   b.serviceDescription,
			Tags:          b.serviceTags,
			Bindable:      true,
			PlanUpdatable: false,
			Plans: []brokerapi.ServicePlan{
				{
					ID:          fmt.Sprintf("%s.%s", b.serviceID, b.planName),
					Name:        b.planName,
					Description: b.planDescription,
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

	// Create the spec to return
	var spec brokerapi.ProvisionedServiceSpec

	// Generate the new policy
	var buf bytes.Buffer
	inp := ServicePolicyTemplateInput{
		ServiceID: instanceID,
		SpaceID:   details.SpaceGUID,
		OrgID:     details.OrganizationGUID,
	}

	b.log.Printf("[DEBUG] generating policy for %s", instanceID)
	if err := GeneratePolicy(&buf, &inp); err != nil {
		return spec, b.wErrorf(err, "failed to generate policy for %s", instanceID)
	}

	// Create the new policy
	policyName := "cf-" + instanceID
	b.log.Printf("[DEBUG] creating new policy %s", policyName)
	if err := b.vaultClient.Sys().PutPolicy(policyName, buf.String()); err != nil {
		return spec, b.wErrorf(err, "failed to create policy %s", policyName)
	}

	// Create the new token role
	path := "/auth/token/roles/cf-" + instanceID
	data := map[string]interface{}{
		"allowed_policies": policyName,
		"period":           VaultPeriodicTTL,
		"renewable":        true,
	}
	b.log.Printf("[DEBUG] creating new token role for %s", path)
	if _, err := b.vaultClient.Logical().Write(path, data); err != nil {
		return spec, b.wErrorf(err, "failed to create token role for %s", path)
	}

	// Determine the mounts we need
	mounts := map[string]string{
		"/cf/" + details.OrganizationGUID + "/secret": "generic",
		"/cf/" + details.SpaceGUID + "/secret":        "generic",
		"/cf/" + instanceID + "/secret":               "generic",
		"/cf/" + instanceID + "/transit":              "transit",
	}

	// Mount the backends
	b.log.Printf("[DEBUG] creating mounts %s", mapToKV(mounts, ", "))
	if err := b.idempotentMount(mounts); err != nil {
		return spec, b.wErrorf(err, "failed to create mounts %s", mapToKV(mounts, ", "))
	}

	// Generate instance info
	info := &instanceInfo{
		OrganizationGUID: details.OrganizationGUID,
		SpaceGUID:        details.SpaceGUID,
	}
	payload, err := json.Marshal(info)
	if err != nil {
		return spec, b.wErrorf(err, "failed to encode instance json")
	}

	// Store the token and metadata in the generic secret backend
	instancePath := "cf/broker/" + instanceID
	b.log.Printf("[DEBUG] storing instance metadata at %s", instancePath)
	if _, err := b.vaultClient.Logical().Write(instancePath, map[string]interface{}{
		"json": string(payload),
	}); err != nil {
		return spec, b.wErrorf(err, "failed to commit instance %s", instancePath)
	}

	// Save the instance
	b.log.Printf("[DEBUG] saving instance %s to cache", instanceID)
	b.instancesLock.Lock()
	b.instances[instanceID] = info
	b.instancesLock.Unlock()

	// Done
	return spec, nil
}

// Deprovision is used to remove a tenant of Vault. We use this to
// remove all the backends of the tenant, delete the token role, and policy.
func (b *Broker) Deprovision(ctx context.Context, instanceID string, details brokerapi.DeprovisionDetails, async bool) (brokerapi.DeprovisionServiceSpec, error) {
	b.log.Printf("[INFO] deprovisioning %s", instanceID)

	// Create the spec to return
	var spec brokerapi.DeprovisionServiceSpec

	// Unmount the backends
	mounts := []string{
		"/cf/" + instanceID + "/secret",
		"/cf/" + instanceID + "/transit",
	}
	b.log.Printf("[DEBUG] removing mounts %s", strings.Join(mounts, ", "))
	if err := b.idempotentUnmount(mounts); err != nil {
		return spec, b.wErrorf(err, "failed to remove mounts")
	}

	// Delete the token role
	path := "/auth/token/roles/cf-" + instanceID
	b.log.Printf("[DEBUG] deleting token role %s", path)
	if _, err := b.vaultClient.Logical().Delete(path); err != nil {
		return spec, b.wErrorf(err, "failed to delete token role %s", path)
	}

	// Delete the token policy
	policyName := "cf-" + instanceID
	b.log.Printf("[DEBUG] deleting policy %s", policyName)
	if err := b.vaultClient.Sys().DeletePolicy(policyName); err != nil {
		return spec, b.wErrorf(err, "failed to delete policy %s", policyName)
	}

	// Delete the instance info
	instancePath := "cf/broker/" + instanceID
	b.log.Printf("[DEBUG] deleting instance info at %s", instancePath)
	if _, err := b.vaultClient.Logical().Delete(instancePath); err != nil {
		return spec, b.wErrorf(err, "failed to delete instance info at %s", instancePath)
	}

	// Delete the instance from the map
	b.log.Printf("[DEBUG] removing instance %s from cache", instanceID)
	b.instancesLock.Lock()
	delete(b.instances, instanceID)
	b.instancesLock.Unlock()

	// Done!
	return spec, nil
}

// Bind is used to attach a tenant of Vault to an application in CloudFoundry.
// This should create a credential that is used to authorize against Vault.
func (b *Broker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	b.log.Printf("[INFO] binding service %s to instance %s",
		bindingID, instanceID)

	// Create the binding to return
	var binding brokerapi.Binding

	// Create the role name to create the token against
	roleName := "cf-" + instanceID

	// Create the token
	renewable := true
	b.log.Printf("[DEBUG] creating token with role %s", roleName)
	secret, err := b.vaultClient.Auth().Token().CreateWithRole(&api.TokenCreateRequest{
		Policies:    []string{roleName},
		Metadata:    map[string]string{"cf-instance-id": instanceID, "cf-binding-id": bindingID},
		DisplayName: "cf-bind-" + bindingID,
		Renewable:   &renewable,
	}, roleName)
	if err != nil {
		return binding, b.wErrorf(err, "failed to create token with role %s", roleName)
	}
	if secret.Auth == nil {
		return binding, b.errorf("secret with role %s has no auth", roleName)
	}

	// Get the instance for this instanceID
	b.log.Printf("[DEBUG] looking up instance %s from cache", instanceID)
	b.instancesLock.Lock()
	instance, ok := b.instances[instanceID]
	b.instancesLock.Unlock()
	if !ok {
		return binding, b.errorf("no instance exists with ID %s", instanceID)
	}

	// Create a binding info object
	info := &bindingInfo{
		Organization: instance.OrganizationGUID,
		Space:        instance.SpaceGUID,
		Binding:      bindingID,
		ClientToken:  secret.Auth.ClientToken,
		Accessor:     secret.Auth.Accessor,
	}
	data, err := json.Marshal(info)
	if err != nil {
		return binding, b.wErrorf(err, "failed to encode binding json")
	}

	// Store the token and metadata in the generic secret backend
	path := "cf/broker/" + instanceID + "/" + bindingID
	b.log.Printf("[DEBUG] storing binding metadata at %s", path)
	if _, err := b.vaultClient.Logical().Write(path, map[string]interface{}{
		"json": string(data),
	}); err != nil {
		a := secret.Auth.Accessor
		if err := b.vaultClient.Auth().Token().RevokeAccessor(a); err != nil {
			b.log.Printf("[WARN] failed to revoke accessor %s", a)
		}
		return binding, errors.Wrapf(err, "failed to commit binding %s", path)
	}

	// Setup Renew timer
	info.stopCh = make(chan struct{})
	go b.renewAuth(info.ClientToken, info.Accessor, info.stopCh)

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
	secret, err := b.vaultClient.Logical().Read(path)
	if err != nil {
		return b.wErrorf(err, "failed to read binding info for %s", path)
	}
	if secret == nil || len(secret.Data) == 0 {
		return b.errorf("missing bind info for unbind for %s", path)
	}

	// Decode the binding info
	b.log.Printf("[DEBUG] decoding binding info for %s", path)
	info, err := decodeBindingInfo(secret.Data)
	if err != nil {
		return b.wErrorf(err, "failed to decode binding info for %s", path)
	}

	// Revoke the token
	a := info.Accessor
	b.log.Printf("[DEBUG] revoking accessor %s for path %s", a, path)
	if err := b.vaultClient.Auth().Token().RevokeAccessor(a); err != nil {
		return b.wErrorf(err, "failed to revoke accessor %s", a)
	}

	// Delete the binding info
	b.log.Printf("[DEBUG] deleting binding info at %s", path)
	if _, err := b.vaultClient.Logical().Delete(path); err != nil {
		return b.wErrorf(err, "failed to delete binding info at %s", path)
	}

	// Delete the bind if it exists, stopping any renewers
	b.log.Printf("[DEBUG] removing binding %s from cache", bindingID)
	b.bindLock.Lock()
	existing, ok := b.binds[bindingID]
	if ok {
		delete(b.binds, bindingID)
		if existing.stopCh != nil {
			close(existing.stopCh)
		}
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
	result, err := b.vaultClient.Sys().ListMounts()
	if err != nil {
		return err
	}

	// Strip all leading and trailing things
	mounts := make(map[string]struct{})
	for k := range result {
		k = strings.Trim(k, "/")
		mounts[k] = struct{}{}
	}

	for k, v := range m {
		k = strings.Trim(k, "/")
		if _, ok := mounts[k]; ok {
			continue
		}
		if err := b.vaultClient.Sys().Mount(k, &api.MountInput{
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
	result, err := b.vaultClient.Sys().ListMounts()
	if err != nil {
		return err
	}

	// Strip all leading and trailing things
	mounts := make(map[string]struct{})
	for k := range result {
		k = strings.Trim(k, "/")
		mounts[k] = struct{}{}
	}

	for _, k := range l {
		k = strings.Trim(k, "/")
		if _, ok := mounts[k]; !ok {
			continue
		}
		if err := b.vaultClient.Sys().Unmount(k); err != nil {
			return err
		}
	}
	return nil
}

// renewAuth renews the given token. It is designed to be called as a goroutine
// and will log any errors it encounters.
func (b *Broker) renewAuth(token, accessor string, stopCh <-chan struct{}) {
	// Sleep for a random number of milliseconds. This helps prevent a thundering
	// herd in the event a broker is restarted with a lot of bindings.
	time.Sleep(time.Duration(rand.Intn(5000)) * time.Millisecond)

	// Use renew-self instead of lookup here because we want the freshest renew
	// and we can find out if it's renewable or not.
	secret, err := b.vaultClient.Auth().Token().RenewTokenAsSelf(token, 0)
	if err != nil {
		b.log.Printf("[ERR] renew-token (%s): error looking up self: %s", accessor, err)
		return
	}

	renewer, err := b.vaultClient.NewRenewer(&api.RenewerInput{
		Secret: secret,
	})
	if err != nil {
		b.log.Printf("[ERR] renew-token (%s): failed to create renewer: %s", accessor, err)
		return
	}
	go renewer.Renew()
	defer renewer.Stop()

	for {
		select {
		case err := <-renewer.DoneCh():
			if err != nil {
				b.log.Printf("[ERR] renew-token (%s): failed: %s", accessor, err)
			}
			b.log.Printf("[WARN] renew-token (%s): renewer stopped: token probably expired!", accessor)
			return
		case renewal := <-renewer.RenewCh():
			remaining := "no auth data"
			if renewal.Secret != nil && renewal.Secret.Auth != nil {
				seconds := renewal.Secret.Auth.LeaseDuration
				remaining = (time.Duration(seconds) * time.Second).String()
			}
			b.log.Printf("[INFO] renew-token (%s): successfully renewed token (%s)", accessor, remaining)
		case <-stopCh:
			b.log.Printf("[INFO] renew-token (%s): stopping renewer: unbind requested", accessor)
			return
		case <-b.stopCh:
			return
		}
	}
}

// renewVaultToken is a convenience wrapper around renewAuth which looks up
// metadata about the token attached to this broker and starts the renewer.
func (b *Broker) renewVaultToken() {
	secret, err := b.vaultClient.Auth().Token().LookupSelf()
	if err != nil {
		b.log.Printf("[ERR] renew-token: failed to lookup client vault token: %s", err)
		return
	}
	if expireTime, ok := secret.Data["expire_time"]; ok && expireTime == nil {
		b.log.Printf("[INFO] renew-token: vault token will never expire so doesn't need to be renewed, stopping renewal process")
		return
	}

	secret, err = b.vaultClient.Auth().Token().RenewSelf(0)
	if err != nil {
		b.log.Printf("[ERR] renew-token: failed to renew client vault token: %s", err)
		return
	}
	if secret.Auth == nil {
		b.log.Printf("[ERR] renew-token: renew-self came back with empty auth")
		return
	}
	b.renewAuth(secret.Auth.ClientToken, secret.Auth.Accessor, nil)
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

func mapToKV(m map[string]string, joiner string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	r := make([]string, len(keys))
	for i, k := range keys {
		r[i] = fmt.Sprintf("%s=%s", k, m[k])
	}
	return strings.Join(r, joiner)
}

// error wraps the given error into the logger and returns it. Vault likes to
// have multiline error messages, which don't mix well with the service broker's
// logging model. Here we strip any newline characters and replace them with a
// space.
func (b *Broker) error(err error) error {
	b.log.Printf("[ERR] %s", strings.Replace(err.Error(), "\n", " ", -1))
	return err
}

// errorf creates a new error from the string and returns it.
func (b *Broker) errorf(s string, f ...interface{}) error {
	return b.error(fmt.Errorf(s, f...))
}

// wErrorf wraps the given error with the string/formatter, logs, and returns
// it.
func (b *Broker) wErrorf(err error, s string, f ...interface{}) error {
	return b.error(errors.Wrapf(err, s, f...))
}
