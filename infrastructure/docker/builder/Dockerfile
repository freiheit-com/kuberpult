FROM bufbuild/buf:1.26.1 as buf
FROM golang:1.24.2-alpine3.21
# FROM docker:28.0.2-dind-alpine3.21

COPY --from=buf /usr/local/bin/buf /usr/local/bin/buf
# COPY --from=golang /usr/local/go/ /usr/local/go/
# ENV PATH /root/go/bin:/usr/local/go/bin:$PATH

RUN apk add --no-cache ca-certificates tzdata alpine-sdk go make pkgconfig build-base git tar npm sqlite sqlite-dev bash cmake g++ musl-dev openssl-dev python3 py3-pip libffi-dev curl libssh2-dev
COPY ./services/manifest-repo-export-service/install-libgit2.sh /tmp/install-libgit2.sh
RUN /tmp/install-libgit2.sh && rm /tmp/install-libgit2.sh

RUN npm install -g pnpm@8.9.2

ARG GO_CI_LINT_VERSION="v1.64.0"
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@$GO_CI_LINT_VERSION
ARG GO_EXHAUSTSTRUCT_VERSION="v3.3.1"
RUN go install github.com/GaijinEntertainment/go-exhaustruct/v3/cmd/exhaustruct@$GO_EXHAUSTSTRUCT_VERSION

# TODO: Uncomment this later once earthly has been completely removed (Ref: SRX-ELW2PF)
# WORKDIR /tmp/kp/
# COPY go.mod go.sum ./
# RUN go mod download
# WORKDIR /
