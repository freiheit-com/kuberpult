FROM bufbuild/buf:1.26.1 as buf
FROM golang:1.24.2-bookworm

COPY --from=buf /usr/local/bin/buf /usr/local/bin/buf

ARG GO_CI_LINT_VERSION="v1.64.0"
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@$GO_CI_LINT_VERSION

ARG GO_EXHAUSTSTRUCT_VERSION="v3.3.1"
RUN go install github.com/GaijinEntertainment/go-exhaustruct/v3/cmd/exhaustruct@$GO_EXHAUSTSTRUCT_VERSION

RUN apt update && apt install --auto-remove npm ca-certificates tzdata libgit2-dev libsqlite3-dev protobuf-compiler  -y
RUN npm install -g pnpm@8.9.2

WORKDIR /
