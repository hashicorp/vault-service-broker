#!/bin/bash 

set -eu

BASEDIR=$(pwd)

mv bbl-cli/bbl*linux* /usr/local/bin/bbl
mv bosh-cli/bosh*linux* /usr/local/bin/bosh

chmod +x /usr/local/bin/*

cd bbl-state

eval "$(bbl print-env)"

bosh -n -d credhub delete-deployment