# Cloud Foundry HashiCorp Vault Broker Changelog

## v0.5.4 (Dec 2, 2020)
BUG FIXES:
- [#57](https://github.com/hashicorp/vault-service-broker/pull/57) fixes an issue where the broker sends too many requests to Vault during startup

## v0.5.3 (May 13, 2019)
BUG FIXES:
- [#49](https://github.com/hashicorp/vault-service-broker/pull/49) fixes an issue during upgrades when broker version <= 0.2.0
- [#48](https://github.com/hashicorp/vault-service-broker/pull/48) continue the unbind when the token has already expired or revoked out of band

## v0.5.2 (April 23, 2019)
BUG FIXES:
- [#47](https://github.com/hashicorp/vault-service-broker/pull/47) prevent errors when PCF does not provide an application id

## v0.5.1 (February 15, 2019)
BUG FIXES:
- [#43](https://github.com/hashicorp/vault-service-broker/pull/43) resolves a bug where it was impossible to unbind if a token had been deleted out-of-band

## v0.5.0 (January 30, 2019)
FEATURES:
- [#42](https://github.com/hashicorp/vault-service-broker/pull/42) adds support for a more polished-looking Marketplace UI

BUG FIXES:
- [#41](https://github.com/hashicorp/vault-service-broker/pull/41) solves multiple minor issues that caused black box testing to fail using `$ cf dev` locally - namely, it updates the go buildpack, the version of go used by cf for building, and deletes two unnecessary files that were symlinked outside the repo

## v0.4.0 (January 29, 2019)
FEATURES:
- [#33](https://github.com/hashicorp/vault-service-broker/pull/33) adds support for a broker-level namespace setting
- [#39](https://github.com/hashicorp/vault-service-broker/pull/39) adds support for application-level secrets engines but also contains a **breaking change** in the format of the JSON returned by the `Bind` call so should be adopted with caution

## v0.3.0 (January 23, 2019)

FEATURES:
- [#37](https://github.com/hashicorp/vault-service-broker/pull/37) adds support for all configuration variables to be provided through CredHub with UAA for CredHub authentication

IMPROVEMENTS:
- Adds functional tests documenting current behavior of the Vault Service Broker
- Adds Travis CI builds

BUG FIXES:
- Resolves [#25](https://github.com/hashicorp/vault-service-broker/issues/25), an extraneous error renewing the Vault token on startup


## v0.2.0 (July 24, 2017)

- Initial Release
- Implements the Bind, Unbind, Provision, and Deprovision calls for the Open Service Broker API
