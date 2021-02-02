# CF Sample App

A sample [Go](https://golang.org/) application to deploy to Cloud Foundry which works out of the box.

For use in the README at https://github.com/hashicorp/vault-service-broker.

## Run locally

1. Install [Go](https://golang.org/doc/install)
1. Run `go run main.go`
1. Visit <http://localhost:8080>

## Run in the cloud

1. Run `cf push my-app -m 64M --random-route`
1. Visit the given URL
