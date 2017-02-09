# CloudFoundry Vault Service Broker

This repository provides a CloudFoundry service broker for [Vault](https://vaultproject.io).
The service broker allows an existing Vault cluster to be used by multiple tenants
from within CloudFoundry.

We make a number of assumptions about the Vault setup, including:

* Vault is already running, it will not be provisioned by the broker.
* Vault is being used by other applications, and is not exclusively used through CloudFoundry.
* All instances of an application will _share_ a token. This goes against how
  Vault is normally used, but this is a limitation of the service broker model.
* Tokens that are revoked by operators require CloudFoundry users to re-bind the instance,
  the broker has no way to update the credentials and restart the dependent applications.

When a new service instance is created using the broker, we perform the following:

* Mount the `generic` backend at `/cf/<organization_id>/secret/`
* Mount the `generic` backend at `/cf/<space_id>/secret/`
* Mount the `generic` backend at `/cf/<service_id>/secret/`
* Mount the `transit` backend at `/cf/<service_id>/transit/`

* Create the `cf-<service_id>` policy which allows:
    * Read to `/cf/<organization_id>/*`
    * Write to `/cf/<space_id>/*`
    * Write to `/cf/<service_id>/*`

* Create the `cf-<service_id>` token role which creates periodic tokens with
  the `cf-<service_id>` policy granted (see above).

This creates a read-only mount for sharing secrets organization wide,
as well as a writeable mount for sharing secrets within an CloudFoundry space
and the particular service instance. The transit backend is mounted to provide
encryption-as-a-service to applications as well.

When a service instance is bound to an application, we perform the following:

* Create a new token using the `cf-<service_id>` role
* Renew the token automatically in the background via the Broker
* Provide the binding credentials
    * `vault_token` - The Vault client token end applicaitons should use
    * `vault_token_accessor` - The Vault token accessor
    * `vault_path` - The path in Vault to write to (e.g. `/cf/<service_id/`)

It is important to note that all instances of a CloudFoundry application
will share the same `vault_token`. This is not the recommended pattern for
using Vault, but it is an existing limitation of the service broker model.

## Granting access to other paths

The service broker has an opinionated setup of policies and mounts to provide
a simplified user experience for getting started which matches the organizational
model of CloudFoundry. However an application may require access to existing
data or backends.

Once the broker creates the policy for a service id `cf-<service_id>` that
policy can simply be modified to add additional capabilities. The default
policy, along with additional capabilities would be like:

```hcl
# Auto-generated rules
path "cf/<service_id>" { capabilities = ["list"] }
path "cf/<service_id>/*" { policy = "write" }
path "cf/<space_id>" { capabilities = ["list"] }
path "cf/<space_id>/*" { policy = "write" }
path "cf/<org_id>" { capabilities = ["list"] }
path "cf/<org_id>/*" { policy = "read" }

# Additional rules to use the existing transit key foo
path "transit/encrypt/foo" { policy = "write" }
path "transit/decrypt/foo" { policy = "write" }
```

## Configuring the Service Broker

The service broker is designed to be configured using environment variables.
It currently recognizes the following:

* `LOG_LEVEL` - (Default "INFO") The logging level to use
* `PORT` - (Default "8000") The listen port to use.
* `SECURITY_USER_NAME` - (No default) The broker user name to authenticate
* `SECURITY_USER_PASSWORD` - (No default) The proker password to authenticate
* `VAULT_ADDR` - (Default "https://127.0.0.1:8200/") The full address of the Vault instance
* `VAULT_TOKEN` - (No default) The Vault token to use for the broker. This must currently
  be a root level token.

## Getting Started

# Decide on the credentials to use for authenticating
export USERNAME="vault"
export PASSWORD="vault"

# Push and start the broker service at vault-broker.SUFFIX
cf push --no-start
cf set-env vault-broker VAULT_ADDR "..."
cf set-env vault-broker VAULT_TOKEN "..."
cf set-env vault-broker SECURITY_USER_NAME "$USERNAME"
cf set-env vault-broker SECURITY_USER_PASSWORD "$PASSWORD"
cf start vault-broker

# Create the service broker
cf enable-service-access vault
cf create-service-broker vault "$USERNAME" "$PASSWORD" http://vault-broker.local.pcfdev.io
cf enable-service-access vault

# Verify the service broker is running
cf marketplace

# Create the service instance
cf create-service vault default vault
