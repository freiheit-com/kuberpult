FROM bufbuild/buf:1.26.1 AS buf
FROM golang:1.24.2-alpine3.21

COPY --from=buf /usr/local/bin/buf /usr/local/bin/buf

RUN apk add --no-cache ca-certificates tzdata alpine-sdk go make pkgconfig build-base git tar npm sqlite sqlite-dev bash cmake g++ musl-dev openssl-dev python3 py3-pip libffi-dev curl libssh2-dev
COPY ./services/manifest-repo-export-service/install-libgit2.sh /tmp/install-libgit2.sh
COPY ./services/manifest-repo-export-service/gitconfig /etc/gitconfig
RUN /tmp/install-libgit2.sh && rm /tmp/install-libgit2.sh
RUN npm install -g pnpm@8.9.2

ARG GO_CI_LINT_VERSION="v2.2.1"
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$GO_CI_LINT_VERSION
ARG GO_EXHAUSTSTRUCT_VERSION="v3.3.1"
RUN go install github.com/GaijinEntertainment/go-exhaustruct/v3/cmd/exhaustruct@$GO_EXHAUSTSTRUCT_VERSION

RUN curl --fail --silent --show-error --location --remote-name "https://get.helm.sh/helm-v3.14.2-linux-amd64.tar.gz"
RUN echo 0885a501d586c1e949e9b113bf3fb3290b0bbf74db9444a1d8c2723a143006a5 helm-v3.14.2-linux-amd64.tar.gz | sha256sum -c
RUN tar xzf helm-v3.14.2-linux-amd64.tar.gz
RUN mv linux-amd64/helm /usr/local/bin/helm
RUN chmod +x /usr/local/bin/helm

WORKDIR /kp
RUN mkdir -p database/migrations
COPY database/migrations database/migrations
COPY go.mod go.sum ./
RUN go mod download
COPY pkg pkg
RUN cd pkg && buf generate
RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
RUN cd pkg &&  oapi-codegen -generate "std-http-server" -o publicapi/server-gen.go -package publicapi publicapi/api.yaml
