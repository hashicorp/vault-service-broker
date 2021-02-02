# HashiCorp Vault Service Broker

This repository provides an implementation of the [open service broker
API][open-broker-api] for [HashiCorp's Vault][vault]. The service broker
connects to an existing Vault cluster and can be used by multiple tenants within
Cloud Foundry.

[open-broker-api]: https://openservicebrokerapi.org/

## Getting Started

The HashiCorp Vault Service Broker does not run a Vault server for you. There is
an assumption that the Vault cluster is already setup and configured. This Vault
server does not need to be running under Cloud Foundry, OpenShift, Kubernetes,
etc, but it must be accessible from within those environments or wherever the
broker is deployed.

These getting started instructions assume the Vault server's address and token
are specified as the following environment variables:

```sh
$ export VAULT_ADDR="https://vault.company.internal/"
$ export VAULT_TOKEN="abcdef-134255-..."
```

Additionally the broker is configured to use basic authentication. The variables
will be:

```sh
$ export AUTH_USERNAME="vault"
$ export AUTH_PASSWORD="broker-secret-password"
```

### Creating a Dev Environment

Note: This section takes a good amount of time (likely, an hour or more) due to downloading a 16gb ISO and VirtualBox taking a while to create an environment.

1. Register for an account at `https://pivotal.io`.
2. Install the `cf` CLI: https://docs.cloudfoundry.org/cf-cli/install-go-cli.html.
3. Ensure you have VirtualBox installed.
4. Ensure you have ~30gb of disk storage available in your local environment for the VirtualBox ISO and the env `cf` will create.
5. Go through installation and configuration of `cf` through the page on the `cf login` step here: https://pivotal.io/platform/pcf-tutorials/getting-started-with-pivotal-cloud-foundry-dev/introduction. Ignore the sample app steps regarding a Spring app.
6. Note that when logging in following the example, the username is literally `user` and the password is literally `pass`, not the username and password you created during registration.
7. Run Vault locally: `$ vault server -dev`. Use the Vault token returned as the `VAULT_TOKEN` value exported above.
8. Make Vault reachable to your VM using ngrok (https://ngrok.com/): `$ ngrok http 8200`. Use the http URL shown as the `VAULT_ADDR` value exported above (example value: "http://4d4053a3.ngrok.io").

### Deploying the Broker

The first step is deploying the broker. This broker can run anywhere including
Cloud Foundry, Kubernetes, Heroku, HashiCorp [Nomad][nomad], or your local
laptop. This example shows running the broker under Cloud Foundry.

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
$ git clone https://github.com/hashicorp/vault-service-broker
$ cd vault-service-broker
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
$ cf set-env vault-broker SECURITY_USER_NAME "$AUTH_USERNAME"
$ cf set-env vault-broker SECURITY_USER_PASSWORD "$AUTH_PASSWORD"
$ cf restage vault-broker
```

Now that it's configured, start the broker:

```shell
$ cf start vault-broker
```

To verify the HashiCorp Vault broker is running, execute:

```shell
$ cf apps

name           requested state   instances   memory   disk   urls
vault-broker   started           1/1         256M     512M   vault-broker-torpedolike-reexploration.local.pcfdev.io
```

Grab the URL and save it in a variable or copy it to your clipboard - we will need this later.

```shell
export BROKER_URL=$(cf app vault-broker | grep -E -w 'urls:|routes:' | awk '{print $2}')
```

NOTE: Different versions of Cloud Foundry display this information differently. If
the result of the pipeline above is empty, try running `cf app vault-broker` and
look at the output. It is possible that the key has changed again and you'll
need to grep for that instead of `urls:` or `routes:`.

Again, there is no requirement that our broker run under Cloud Foundry - this
could be a URL pointing to any service that hosts the broker, which is just a
Golang HTTP server.

To verify the broker is working as expected, query its catalog:

```shell
$ curl -s "${AUTH_USERNAME}:${AUTH_PASSWORD}@${BROKER_URL}/v2/catalog"
```

The result will be JSON that includes the list of plans for the broker:

```javascript
{
  "services": [
    {
      "id": "0654695e-0760-a1d4-1cad-5dd87b75ed99",
      "name": "hashicorp-vault",
      "description": "HashiCorp Vault Service Broker",
      "bindable": true,
      "tags": [""],
      "plan_updateable": false,
      "plans": [
        {
          "id": "0654695e-0760-a1d4-1cad-5dd87b75ed99.shared",
          "name": "shared",
          "description": "Secure access to Vault's storage and transit backends",
          "free": true
        }
      ]
    }
  ]
}
```

The HashiCorp Vault Service Broker is now running under Cloud Foundry and ready
to receive requests.

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
$ cf create-service-broker vault-broker "${AUTH_USERNAME}" "${AUTH_PASSWORD}" "https://${BROKER_URL}" --space-scoped
```

Note: This will allow developers in the current space to access the broker. If
other developers in another space want to access it, they will need to create
their own instance. This is a good starting workflow, but more complex setups
should investigate [allowing access to plans globally or
per-organization][cf-service-acls].

Note: Here we are scoping the broker to a space. In larger installations, it may
be desirable to run a centralized instance of the broker that is accessible to
all spaces in the organization. For more information on this deployment pattern,
please see the notes in the [standard broker](#global-standard-broker) section.

To verify the command worked, query the marketplace. You should see the Vault broker
with a plan of 'default' in addition to any other services you may have access to:

```shell
$ cf marketplace
service           plans             description
hashicorp-vault   shared            HashiCorp Vault Service Broker
# ...
```

### Create a service instance and bind an app

After registering the service in the marketplace, it is now possible to create a
service instance and bind to it.

First, create a service instance using the default plan.  For this example we
will name the service instance 'my-vault':

```shell
$ cf create-service hashicorp-vault shared my-vault
```

With a service instance in place, you are ready to bind an app. Suppose we have
an app called 'my-app'. An example of my-app can be found in the `example` directory
along with instructions on how to deploy it.

```shell
$ cf bind-service my-app my-vault
```

You will need to restage the app to pick up the environment changes.

```shell
$ cf restage my-app
```

When the app starts back up, its `VCAP_SERVICES` environment variable will
contain an entry for the my-vault service.  You can confirm this by checking the
app's environment variables:

```shell
$ cf env my-app
```

The `VCAP_SERVICES` environment variable will have a section similar to
the following:

```json
{
	"hashicorp-vault": [{
		"credentials": {
			"address": "http://ae585ec6.ngrok.io/",
			"auth": {
				"accessor": "kMr3iCSlekSN2d1vpPjbjzUk",
				"token": "s.qgVrPa3eKawwDDkeOSXUaWZq"
			},
			"backends": {
				"generic": [
					"cf/7f1a12a9-4a52-4151-bc96-874380d30182/secret",
					"cf/c4073566-baee-48ae-88e9-7c7c7e0118eb/secret"
				],
				"transit": [
					"cf/7f1a12a9-4a52-4151-bc96-874380d30182/transit",
					"cf/c4073566-baee-48ae-88e9-7c7c7e0118eb/transit"
				]
			},
			"backends_shared": {
				"organization": "cf/8d4b992f-cca3-4876-94e0-e49170eafb67/secret",
				"space": "cf/bdace353-e813-4efb-8122-58b9bd98e3ab/secret"
			}
		},
		"label": "hashicorp-vault",
		"name": "my-vault",
		"plan": "shared",
		"provider": null,
		"syslog_drain_url": null,
		"tags": [],
		"volume_mounts": []
	}]
}
```

The keys of the `credentials` section are as follows:

- `address` - address to the Vault server to make requests against

- `auth.accessor` - token accessor which can be used for logging

- `auth.token` - token to supply with requests to Vault

- `backends.generic` - namespaces in Vault where this token has full CRUD access
  to the static secret storage ("generic") backend, one at the application level
  and the other at the service instance level

- `backends.transit` - namespaces in Vault where this token has full access to
  the transit ("encryption as a service") backend, one at the application level
  and the other at the service instance level

- `backends_shared.organization` - namespace in Vault where this token has
  read-only access to organization-wide data; all instances have read-only
  access to this path, so it can be used to share information across the
  organization.

- `backends_shared.space` - namespace in Vault where this token has read-write
  access to space-wide data; all instances have read-write access to this path,
  so it can be used to share information across the space.

## Internals

### Architecture and Assumptions

To ease in setup and administration, the HashiCorp Vault Service Broker makes a
few assumptions about the Vault setup including:

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
1. Mount the `generic` backend at `/cf/<app_id>/secret/`
1. Mount the `transit` backend at `/cf/<app_id>/transit/`
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

### Unbinding and Deleting

When unbinding from a service or deleting the service broker entirely, the
broker deletes an instance-specific data. For safety, the broker does not delete
an space or organization-specific mounts, even if there are no remaining service
brokers using it.

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
  this when configuring the HashiCorp Vault Service Broker.

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

- `PLAN_NAME` (default: "shared") - the name of the plan in the marketplace

- `PLAN_DESCRIPTION` (default: "Secure access to Vault's storage and transit backends") - description of the plan in the marketplace

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
  
- `VAULT_NAMESPACE` - (default: none) - namespace to use for all calls within Vault

- `SECURITY_USER_NAME` - (default: none) - username for basic auth

- `SECURITY_USER_PASSWORD` - (default: none) - password for basic auth

### Providing Configuration Through CredHub

Any of the Vault Service Broker's environment variables can be set through CredHub. 
Authenticating to CredHub is typically done by using a UAA server. To configure this,
the following environment variables must be set:

- `CREDHUB_URL` (default: none) - CredHub's base URL (ex. "https://example.com")
- `UAA_ENDPOINT` (default: none) - UAA's base URL (ex. "http://localhost:8080/uaa")
- `UAA_CLIENT_NAME` (default: none) - Client name to use when gaining CredHub auth token through UAA
- `UAA_CLIENT_SECRET` (default: none) - Client secret to use when gaining CredHub auth token through UAA

The following optional parameters are also available:

- `UAA_CA_CERTS` (default: none) - CA certs to use when authenticating to UAA
- `UAA_SKIP_VERIFICATION` (default: false) - Skip verifying certificates when calling UAA
- `UAA_INSECURE_ALLOW_ANY_SIGNING_METHOD` (default: false) - Allow any signing method when verifying UAA certs

Once this is set, Vault will check CredHub for all environment variables listed above.
All variables must be prefixed with "VAULT_SERVICE_BROKER_". For example:

- `VAULT_SERVICE_BROKER_SECURITY_USER_PASSWORD`: "$ecure_pa$$w0rd"
- `VAULT_SERVICE_BROKER_VAULT_RENEW`: "false"
- `VAULT_SERVICE_BROKER_SERVICE_TAGS`: "production,security"

Please keep the following in mind:
- You may set configuration through both CredHub and environment variables
- CredHub is preferred, so if a variable exists in both places, the CredHub value will prevail
- The values for the CredHub variables must be given as strings in the same format as you would an environment variable

### Granting Access to Other Paths

The service broker has an opinionated setup of policies and mounts to provide a
simplified user experience for getting started which matches the organizational
model of Cloud Foundry. However an application may require access to existing
data or backends.

Once the broker creates the policy for a service id `cf-<instance_id>` that
policy may be modified by a user with permissions in Vault to add additional
capabilities. The default policy can be discovered by reading it:

```sh
$ vault read -field=rules sys/policy/cf-<instance_id>
# ...
```

Append any additional rules to the end.

### Global Standard Broker

The default configuration and examples above use a "space scoped" broker. For
larger installations, it may be desirable to run a centralized instance of the
broker that is accessible to all spaces and applications in the system. This is
generally called a "standard broker" in Cloud Foundry terminology. To deploy a
global broker, an admin must add, publish, and permit access as follows.

First, create the broker. Note the lack of `--space-scoped` as compared to the
previous commands.

```shell
$ cf create-service-broker vault-broker "${AUTH_USERNAME}" "${AUTH_PASSWORD}" "https://${BROKER_URL}"
```

To verify the broker was created:

```shell
$ cf service-access
# ...

broker: vault-broker
   service           plan     access   orgs
   hashicorp-vault   shared   none
```

Notice the access is listed as "none". This broker will not appear in the
marketplace until activated by an admin. To activate, run:

```shell
$ cf enable-service-access hashicorp-vault
```

It is possible to restrict the access to particular organizations or plans.

```shell
$ cf enable-service-access hashicorp-vault -o my-org
```

Verify the plan is enabled:

```shell
$ cf service-access
# ...

broker: vault-broker
   service           plan     access   orgs
   hashicorp-vault   shared   all
```

And check in the marketplace

```shell
$ cf marketplace
service           plans             description
hashicorp-vault   shared            HashiCorp Vault Service Broker
# ...
```

Now all spaces have access to this centralized broker!

## Contributing

1. Clone the repo
1. Make changes on a branch
1. Test changes
1. Submit a Pull Request to GitHub

[cf-service-acls]: https://docs.cloudfoundry.org/services/access-control.html "Cloud Foundry Service ACLs"
[nomad]: https://www.nomadproject.io/ "Nomad by HashiCorp"
[vault]: https://www.vaultproject.io/ "Vault by HashiCorp"
[vault-periodic-token]: https://www.vaultproject.io/docs/concepts/tokens.html#token-time-to-live-periodic-tokens-and-explicit-max-ttls "Vault Periodic Tokens"
