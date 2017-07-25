#
# Builder
#
FROM golang:1.8 AS builder
LABEL maintainer "Seth Vargo <seth@sethvargo.com> (@sethvargo)"

WORKDIR /go/src/github.com/hashicorp/vault-service-broker

COPY . .

ARG LD_FLAGS=""
ENV LD_FLAGS="${LD_FLAGS}"

RUN \
  CGO_ENABLED="0" \
  GOOS="linux" \
  GOARCH="amd64" \
  go build -a -o "/vault-service-broker" -ldflags "${LD_FLAGS}"

#
# Final
#
FROM scratch
LABEL maintainer "Seth Vargo <seth@sethvargo.com> (@sethvargo)"

ADD "https://curl.haxx.se/ca/cacert.pem" "/etc/ssl/certs/ca-certificates.crt"
COPY --from=builder /vault-service-broker /vault-service-broker
ENTRYPOINT ["/vault-service-broker"]
