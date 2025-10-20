FROM bufbuild/buf:1.55.1 AS buf
FROM golang:1.24.2-alpine3.21

COPY --from=buf /usr/local/bin/buf /usr/local/bin/buf

RUN apk add --no-cache  \
        ca-certificates tar tzdata curl bash git \
        gcc g++ pkgconfig make cmake \
        go \
        nodejs npm

RUN npm install -g pnpm@8.9.2

ARG GO_CI_LINT_VERSION="v2.2.1"
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin $GO_CI_LINT_VERSION
#RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$GO_CI_LINT_VERSION

RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

RUN curl --fail --silent --show-error --location --remote-name "https://get.helm.sh/helm-v3.14.2-linux-amd64.tar.gz"
RUN echo 0885a501d586c1e949e9b113bf3fb3290b0bbf74db9444a1d8c2723a143006a5 helm-v3.14.2-linux-amd64.tar.gz | sha256sum -c

RUN tar xzf helm-v3.14.2-linux-amd64.tar.gz && rm -rf helm-v3.14.2-linux-amd64.tar.gz
RUN mv linux-amd64/helm /usr/local/bin/helm
RUN chmod +x /usr/local/bin/helm

WORKDIR /kp

# Installing Go dependencies should be the only Go-code related step done on the builder image
COPY go.mod go.sum ./
#RUN go mod download

