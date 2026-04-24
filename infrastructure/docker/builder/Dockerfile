FROM golang:1.25.8-alpine3.23

RUN apk add --no-cache  \
        ca-certificates tar tzdata curl bash git \
        gcc g++ pkgconfig make cmake \
        go protoc protobuf-dev \
        python3 py3-pip \
        musl-dev libffi-dev openssl-dev libssh2-dev \
        sqlite sqlite-dev \
        nodejs npm

COPY services/manifest-repo-export-service/install-libgit2.sh /tmp/install-libgit2.sh
COPY services/manifest-repo-export-service/gitconfig /etc/gitconfig
RUN /tmp/install-libgit2.sh && rm /tmp/install-libgit2.sh

RUN npm install -g pnpm@8.15.9

ARG GO_CI_LINT_VERSION="v2.9.0"
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin $GO_CI_LINT_VERSION

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1

ARG TARGETARCH
RUN HELM_SHA256=$(case "$TARGETARCH" in \
        amd64) echo "0885a501d586c1e949e9b113bf3fb3290b0bbf74db9444a1d8c2723a143006a5" ;; \
        arm64) echo "c65d6a9557bb359abc2c0d26670de850b52327dc3976ad6f9e14c298ea3e1b61" ;; \
    esac) && \
    curl --fail --silent --show-error --location --remote-name "https://get.helm.sh/helm-v3.14.2-linux-${TARGETARCH}.tar.gz" && \
    echo "${HELM_SHA256}  helm-v3.14.2-linux-${TARGETARCH}.tar.gz" | sha256sum -c && \
    tar xzf "helm-v3.14.2-linux-${TARGETARCH}.tar.gz" && \
    mv "linux-${TARGETARCH}/helm" /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helm && \
    rm -rf "helm-v3.14.2-linux-${TARGETARCH}.tar.gz" "linux-${TARGETARCH}"

WORKDIR /kp

COPY go.mod go.sum ./

