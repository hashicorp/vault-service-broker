#!/bin/bash 

set -eu

BASEDIR=$(pwd)

cp -Rvf bbl-plan/* overridden-bbl-plan
cp -Rvf source/integration-tests/bbl-overrides/* overridden-bbl-plan/terraform/
