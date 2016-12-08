package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

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
)

var (
	// Verify the ServiceBroker implementes the brokerapi.
	_ brokerapi.ServiceBroker = (*Broker)(nil)

	// ErrNotImplemented is the error returned when a signature is not implemented.
	ErrNotImplemented = errors.New("not implemented")
)

type Broker struct {
	log    lager.Logger
	client *api.Client

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

func (b *Broker) Provision(ctx context.Context, instanceID string, details brokerapi.ProvisionDetails, async bool) (brokerapi.ProvisionedServiceSpec, error) {
	b.log.Debug("provisioning new instance", lager.Data{
		"instance-id": instanceID,
	})

	return brokerapi.ProvisionedServiceSpec{}, nil
}

func (b *Broker) Deprovision(ctx context.Context, instanceID string, details brokerapi.DeprovisionDetails, async bool) (brokerapi.DeprovisionServiceSpec, error) {
	b.log.Debug("deprovisioning new instance", lager.Data{
		"instance-id": instanceID,
	})

	return brokerapi.DeprovisionServiceSpec{}, nil
}

func (b *Broker) Bind(ctx context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	b.log.Debug("binding service", lager.Data{
		"binding-id":  bindingID,
		"instance-id": instanceID,
	})

	return brokerapi.Binding{}, nil
}

func (b *Broker) Unbind(ctx context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	b.log.Debug("unbinding service", lager.Data{
		"binding-id":  bindingID,
		"instance-id": instanceID,
	})

	return nil
}

// Update is not implemented.
func (b *Broker) Update(ctx context.Context, instanceID string, details brokerapi.UpdateDetails, async bool) (brokerapi.UpdateServiceSpec, error) {
	return brokerapi.UpdateServiceSpec{}, ErrNotImplemented
}

// LastOperation is not implemented.
func (b *Broker) LastOperation(ctx context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	return brokerapi.LastOperation{}, ErrNotImplemented
}
