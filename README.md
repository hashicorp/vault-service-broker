# Cloud Foundry Vault Service Broker

This repository provides an implementation of a Cloud Foundry service broker for
[HashiCorp's Vault][vault]. The service broker connects to an existing Vault
cluster and can be used by multiple tenants within Cloud Foundry.

## Getting Started

The Cloud Foundry HashiCorp Vault Broker does not run a Vault server for you.
There is an assumption that the Vault cluster is already setup and configured.
This Vault server does not need to be running under Cloud Foundry, but it must
be accessible from within Cloud Foundry or wherever the broker is deployed.

These getting started instructions assume the Vault server's address and token
are specified as the following environment variables:

```sh
$ export VAULT_ADDR="https://vault.company.internal/"
$ export VAULT_TOKEN="abcdef-134255-..."
```

Additionally the broker is configured to use basic authentication. The variables
will be:

```sh
$ export USERNAME="vault"
$ export PASSWORD="vault"
```

### Deploying the Broker

The first step is deploying the broker. This broker can run anywhere including
Cloud Foundry, Heroku, HashiCorp [Nomad][nomad], or your local laptop. This
example shows running the broker under Cloud Foundry.

First, create a space in which to run the broker:

```shell
$ cf create-space vault-broker
```

Switch over to that space:

```shell
$ cf target -s vault-broker
```

Deploy the vault-broker by cloning this repository:

```shell
$ git clone https://github.com/hashicorp/cf-vault-broker
$ cd cf-vault-broker
```

And push to Cloud Foundry:

```shell
$ cf push --random-route --no-start
```

- The `--random-route` flag is optional, but it allows us to easily run more
  than one Vault broker if needed, instead of relying on "predictable names".

- The `--no-start` flag is important because we have not supplied the required
  environment variables to our application yet. It will fail to start now.

To configure the broker, provide the following environment variables:

```shell
$ cf set-env vault-broker VAULT_ADDR "$VAULT_ADDR"
$ cf set-env vault-broker VAULT_TOKEN "$VAULT_TOKEN"
$ cf set-env vault-broker SECURITY_USER_NAME "$USERNAME"
$ cf set-env vault-broker SECURITY_USER_PASSWORD "$PASSWORD"
```

Now that it's configured, start the broker:

```shell
$ cf start vault-broker
```

To verify the Cloud Foundry HashiCorp Vault broker is running, execute:

```shell
$ cf apps

name           requested state   instances   memory   disk   urls
vault-broker   started           1/1         256M     512M   vault-broker-torpedolike-reexploration.local.pcfdev.io
```

Grab the URL and save it in a variable or copy it to your clipboard - we will need this later.

```shell
BROKER_URL=$(cf app vault-broker | grep urls: | awk '{print $2}')
```

Again, there is no requirement that our broker run under Cloud Foundry - this
could be a URL pointing to any service that hosts the broker, which is just a
Golang HTTP server.

To verify the broker is working as expected, query its catalog:

```shell
$ curl -s "$USERNAME:$PASSWORD@${BROKER_URL}/v2/catalog"
```

The result will be JSON that includes the list of plans for the broker:

```javascript
{
  "services": [
    {
      "id": "0654695e-0760-a1d4-1cad-5dd87b75ed99",
      "name": "vault",
      "description": "HashiCorp Vault Service Broker",
      "bindable": true,
      "plan_updateable": false,
      "plans": [
        {
          "id": "0654695e-0760-a1d4-1cad-5dd87b75ed99.default",
          "name": "default",
          "description": "Secure access to a multi-tenant HashiCorp Vault cluster",
          "free": true
        }
      ]
    }
  ]
}
```

The HashiCorp Vault Broker is now running under Cloud Foundry and ready to
receive requests.

### Register the Vault Broker

Before it can bind to services, the broker must be registered with Cloud
Foundry. Remember that there is no requirement that the broker be running under
Cloud Foundry, so we will need to provide the broker registration service the
`BROKER_URL` from above.

To register the broker, an application developer first creates a new space where
they will request the broker access:

```shell
$ cf create-space example
$ cf target -s example
```

Next, register the broker in this space:

```shell
$ cf create-service-broker vault-broker vault vault "https://${BROKER_URL}" --space-scoped
```

To verify the command worked, query the marketplace. You should see the broker
in addition to the built-ins:

```shell
$ cf marketplace
# ...
vault          default           HashiCorp Vault Service Broker
```

### Generate Credentials through the Vault Broker

After registering the service in the marketplace, it is now possible to create a
service instance and bind to it.

```shell
$ cf create-service vault default my-vault
```

Next, create a service key:

```shell
$ cf create-service-key my-vault my-key
```

And finally retrieve credentials for this instance:

```shell
$ cf service-key my-vault my-key
```

The response will look like:

```javascript
{
 "address": "https://vault.company.internal:8200/",
 "auth": {
  "accessor": "b1074bb8-4d15-36cf-54dd-2716fb8ac91d",
  "token": "dff95895-6a03-0b29-6458-dc8602dc9df8"
 },
 "backends": {
  "generic": "cf/203f2469-04e4-47b8-bc17-f3af56df8019/secret",
  "transit": "cf/203f2469-04e4-47b8-bc17-f3af56df8019/transit"
 },
 "backends_shared": {
  "organization": "cf/3c88c61e-875b-4530-b269-970f926340c4/secret",
  "space": "cf/0348d384-f7d4-462b-9bd9-5a4c05b21b6c/secret"
 }
}
```

The keys are as follows:

- `address` - address to the Vault server to make requests against

- `auth.accessor` - token accessor which can be used for logging

- `auth.token` - token to supply with requests to Vault

- `backends.generic` - namespace in Vault where this token has full CRUD access
  to the static secret storage ("generic") backend

- `backends.transit` - namespace in Vault where this token has full access to
  the transit ("encryption as a service") backend

- `backends_shared.organization` - namespace in Vault where this token has
  read-only access to organization-wide data; all instances have read-only
  access to this path, so it can be used to share information across the
  organization.

- `backends_shared.space` - namespace in Vault where this token has read-write
  access to space-wide data; all instances have read-write access to this path,
  so it can be used to share information across the space.

## Internals

### Architecture and Assumptions

To ease in setup and administration, the Cloud Foundry HashiCorp Vault Broker
makes a few assumptions about the Vault setup including:

- The Vault server is already running and is accessible by the broker.

- The Vault server may be used by other applications (it is not exclusively tied
  to Cloud Foundry).

- All instances of an application will _share_ a token. This goes against the
  recommended Vault usage, but this is a limitation of the Cloud Foundry service
  broker model.

- Any Vault operations performed outside of Cloud Foundry will require users to
  rebind their instances.

When a new service instance is provisioned using the broker, it will mount the
following paths:

1. Mount the `generic` backend at `/cf/<organization_id>/secret/`
1. Mount the `generic` backend at `/cf/<space_id>/secret/`
1. Mount the `generic` backend at `/cf/<instance_id>/secret/`
1. Mount the `transit` backend at `/cf/<instance_id>/transit/`

The mount operation is idempotent, so service instances in the same organization
or space will not re-create the mount. These mount points will be returned to
the application in the secret data, so there is no need to "guess" or
interpolate these strings.

The read-only organization mount allows for sharing secrets organization wide,
and the read-write mount permits sharing secrets between applications in the
same Cloud Foundry Space. The transit backend is mounted to provide
"encryption-as-a-service" on a per-application level.

After mounting is complete, the broker generates a custom policy specific for
this instance which grants the following:

- Read-only access to `"cf/<organization_id>/*"`
- Read-write access to `"cf/<space_id>/*"`
- Full access to `"cf/<instance_id>/*"`

This policy is named `"cf-<instance_id>"` and can be further customized outside
of Cloud Foundry by a Vault administrator.

Next the broker creates a new token role. This role creates a periodic token
with the above policy attached. This **does not** create the token yet, just the
role for generating the token.

When a service instance is bound to an application, the broker performs the
following operations:

- Create a new token against the previous "cf-<instance_id>" role.

- Start a background process to renew this token

- Generate and returning the binding credentials (see above for the schema)

It is important to note that all instances of a Cloud Foundry application
will share the same `vault_token`. This is not the recommended pattern for
using Vault, but it is an existing limitation of the service broker model.

### Broker Vault Token Permissions

The Cloud Foundry Vault Broker requires a `VAULT_TOKEN` to operate. This token
should have elevated permissions in Vault, as it will be responsible for
generating new mounts and committing data to its internal data structure.

Here is a sample policy to assign to this token:

```hcl
# Manage internal state under "/broker", but since this token is going to
# generate children, it needs full management of the "/cf/*" space
path "/cf/" {
  capabilities = ["list"]
}

path "/cf/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

# List all mounts
path "sys/mounts" {
  capabilities = ["read", "list"]
}

# Create mounts under the "/cf/" prefix
path "sys/mounts/cf/*" {
  capabilities = ["create", "update", "delete"]
}

# Create policies with the "cf-*" prefix
path "sys/policy/cf-*" {
  capabilities = ["create", "update", "delete"]
}

# Create token role
path "/auth/token/roles/cf-*" {
  capabilities = ["create", "update", "delete"]
}

# Create tokens from role
path "/auth/token/create/cf-*" {
  capabilities = ["create", "update"]
}

# Revoke tokens by accessor
path "/auth/token/revoke-accessor" {
  capabilities = ["create", "update"]
}
```

Additionally, this token should be a [periodic token][vault-periodic-token]. The
Cloud Foundry Vault Broker will renew this periodic token automatically.

1. Authenticate as a root token or user with sudo privilege in Vault (this is
  required to create a periodic token):

  ```shell
  $ vault auth <token>
  ```

1. Create the policy specific for the broker:

  ```shell
  $ vault write sys/policy/cf-broker rules=@cf-broker.hcl
  ```

1. Create a periodic token

  ```shell
  $ vault token-create -period="30m" -orphan -policy=cf-broker
  ```

  Grab the value for "token" and store it somewhere safe for now - you will need
  this when configuring the Cloud Foundry Vault Service Broker.

### Service Broker Configuration

The service broker is designed to be configured using environment variables. It
currently recognizes the following.

- `SERVICE_DESCRIPTION` (default: "HashiCorp Vault Service Broker") -
  description of the service to show in the marketplace

- `SERVICE_ID` (default: "0654695e-0760-a1d4-1cad-5dd87b75ed99") - UUID of the
  broker

- `SERVICE_NAME` (default: "hashicorp-vault") - name of the service to show in
  the marketplace

- `SERVICE_TAGS` (default: none) - comma-separated list of tags for the service

- `PORT` (default: "8000") - port to bind and listen on as the server (broker)

- `VAULT_ADDR` (default: "https://127.0.0.1:8200") - address to the Vault server

- `VAULT_ADVERTISE_ADDR` (default: "$VAULT_ADDR") - address to advertise to
  clients as Vault's address. This defaults to the value supplied for
  `VAULT_ADDR`, but can be overridden. This is most useful when the broker
  communicates to Vault on a local subnet, but clients communicate through a
  public subnet.

- `VAULT_RENEW` (default: true) - enable renewal of the token provided to Vault.
  The token given to Vault is assumed to be a periodic token, and the broker
  will automatically renew it to prevent it from expiring. If an out-of-band
  process is managing the renewal, disable this by setting it to "false".

- `VAULT_TOKEN` (default: none) - token to authenticate the broker to Vault.
  This token should have permission to mount and unmount backends, read, list,
  and delete paths, and create tokens with role permissions. Please see the
  [Vault Token Permissions](#vault-token-permissions) section for more
  information on the requirements for this token.

- `SECURITY_USER_NAME` - (default: none) - username for basic auth

- `SECURITY_USER_PASSWORD` - (default: none) password for basic auth



[nomad]: https://www.nomadproject.io/ "Nomad by HashiCorp"
[vault]: https://www.vaultproject.io/ "Vault by HashiCorp"
[vault-periodic-token]: https://www.vaultproject.io/docs/concepts/tokens.html#token-time-to-live-periodic-tokens-and-explicit-max-ttls "Vault Periodic Tokens"














## Granting access to other paths

The service broker has an opinionated setup of policies and mounts to provide a
simplified user experience for getting started which matches the organizational
model of CloudFoundry. However an application may require access to existing
data or backends.

Once the broker creates the policy for a service id `cf-<instance_id>` that
policy can simply be modified to add additional capabilities. The default
policy, along with additional capabilities would be like:

```hcl
# Auto-generated rules
path "cf/<instance_id>" { capabilities = ["list"] }
path "cf/<instance_id>/*" { policy = "write" }
path "cf/<space_id>" { capabilities = ["list"] }
path "cf/<space_id>/*" { policy = "write" }
path "cf/<org_id>" { capabilities = ["list"] }
path "cf/<org_id>/*" { policy = "read" }

# Additional rules to use the existing transit key foo
path "transit/encrypt/foo" { policy = "write" }
path "transit/decrypt/foo" { policy = "write" }
```
