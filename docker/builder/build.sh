#! /bin/sh

# exit if a command fails
set -xe

VERSION=1.5.0
OS=linux
ARCH=amd64

# Installing Prerequisites
apk add curl git make bash jq --no-cache

# Install docker-credentials-gr
curl -fsSL "https://github.com/GoogleCloudPlatform/docker-credential-gcr/releases/download/v${VERSION}/docker-credential-gcr_${OS}_${ARCH}-${VERSION}.tar.gz" \
    | tar xz --to-stdout ./docker-credential-gcr > /usr/bin/docker-credential-gcr && chmod +x /usr/bin/docker-credential-gcr
