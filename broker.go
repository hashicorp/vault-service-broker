package main

import (
	"context"

	"code.cloudfoundry.org/lager"

	"github.com/pivotal-cf/brokerapi"
)

var _ brokerapi.ServiceBroker = (*Broker)(nil)

type Broker struct {
	log lager.Logger
}

func (b *Broker) Start() error {
	return nil
}

func (b *Broker) Stop() error {
	return nil
}

func (b *Broker) Services(ctx context.Context) []brokerapi.Service {
	b.log.Debug("building services catalog")
	return nil
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
