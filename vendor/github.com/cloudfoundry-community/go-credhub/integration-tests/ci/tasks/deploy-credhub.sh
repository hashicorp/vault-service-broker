#!/bin/bash 

set -eu

BASEDIR=$(pwd)

mv bbl-cli/bbl*linux* /usr/local/bin/bbl
mv bosh-cli/bosh*linux* /usr/local/bin/bosh

chmod +x /usr/local/bin/*

cd bbl-state

eval "$(bbl print-env)"

for release in $(find ${BASEDIR} -name '*-bosh-release' -type d); do
    bosh upload-release --sha1="$(cat ${release}/sha1)" --version="$(cat ${release}/version)" "$(cat ${release}/url)"
done

for stemcell in $(find ${BASEDIR} -name '*-stemcell' -type d); do
    bosh upload-stemcell --sha1="$(cat ${stemcell}/sha1)" --version="$(cat ${stemcell}/version)" "$(cat ${stemcell}/url)"
done

internal_ip=10.0.16.190
external_ip=$(bosh int vars/director-vars-file.yml --path /go-credhub-external-ip)

bosh -n update-config ${BASEDIR}/source/integration-tests/manifest/vip-cloud-config.yml --type=cloud --name=vip-network
bosh -n -d credhub deploy ${BASEDIR}/source/integration-tests/manifest/credhub.yml \
    -o ${BASEDIR}/source/integration-tests/manifest/opsfile.yml \
    -v external-ip-address="${external_ip}" \
    -v internal-ip-address="${internal_ip}"