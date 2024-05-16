VERSION 0.8
FROM busybox
ARG --global UID=1000
ARG --global target=docker

deps:
    ARG USERARCH
    IF [ "$USERARCH" = "arm64" ]
        FROM golang:1.21-bookworm
        RUN apt update && apt install --auto-remove ca-certificates tzdata libgit2-dev libsqlite3-dev -y
    ELSE
        FROM golang:1.21-alpine3.18
        RUN apk add --no-cache ca-certificates tzdata bash libgit2-dev sqlite-dev alpine-sdk
    END
    
    COPY buf_sha256.txt .
    ARG BUF_VERSION=v1.26.1
    ARG BUF_BIN_PATH=/usr/local/bin
    RUN OS=Linux ARCH=$(uname -m) && \
        wget "https://github.com/bufbuild/buf/releases/download/${BUF_VERSION}/buf-${OS}-${ARCH}" \
        -O "${BUF_BIN_PATH}/buf" && \
        chmod +x "${BUF_BIN_PATH}/buf"
    RUN OS=Linux ARCH=$(uname -m) && \
        SHA=$(cat buf_sha256.txt | grep "buf-${OS}-${ARCH}$" | cut -d ' ' -f1) && \
        echo "${SHA}  ${BUF_BIN_PATH}/buf" | sha256sum -c
    
    ARG GO_CI_LINT_VERSION="v1.51.2"
    RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@$GO_CI_LINT_VERSION

    RUN wget https://github.com/GaijinEntertainment/go-exhaustruct/archive/refs/tags/v3.2.0.tar.gz -O exhaustruct.tar.gz
    RUN echo 511d0ba05092386a59dca74b6cbeb99f510b814261cc04b68213a9ae31cf8bf6  exhaustruct.tar.gz | sha256sum -c
    RUN tar xzf exhaustruct.tar.gz
    WORKDIR go-exhaustruct-3.2.0
    RUN go build ./cmd/exhaustruct
    RUN mv exhaustruct /usr/local/bin/exhaustruct

    WORKDIR /kp
    RUN mkdir -p database/migrations
    COPY database/migrations database/migrations
    COPY go.mod go.sum ./
    RUN go mod download

    SAVE ARTIFACT go.mod
    SAVE ARTIFACT go.sum
    SAVE ARTIFACT $BUF_BIN_PATH/buf

migration-deps:
    FROM scratch

    COPY cd_database/ cd_database/

    SAVE ARTIFACT cd_database/migrations

cd-service:
    BUILD ./services/cd-service+$target --UID=$UID --service=cd-service

rollout-service:
    BUILD ./services/rollout-service+$target --UID=$UID --service=rollout-service

frontend-service:
    BUILD ./services/frontend-service+$target --UID=$UID --service=frontend-service

ui:
    BUILD ./services/frontend-service+$target-ui

all-services:
    BUILD ./pkg+deps
    BUILD ./services/cd-service+docker --service=cd-service --UID=$UID
    BUILD ./services/frontend-service+docker --service=frontend-service
    BUILD ./services/frontend-service+docker-ui

cache:
    BUILD ./services/cd-service+release --service=cd-service --UID=$UID
    BUILD ./services/rollout-service+release --service=rollout-service --UID=$UID
    BUILD ./services/frontend-service+release --service=frontend-service
    BUILD ./services/frontend-service+release-ui

commitlint:
    FROM node:18-bookworm
    WORKDIR /commitlint/
    RUN npm install --save-dev @commitlint/cli@18.4.3
    WORKDIR /commitlint/
    COPY .commitlintrc .commitlintrc
    COPY commitlint.msg commitlint.msg
    RUN cat ./commitlint.msg | npx commitlint --config .commitlintrc

test-all:
    BUILD ./services/cd-service+unit-test --service=cd-service
    BUILD ./services/rollout-service+unit-test --service=rollout-service
    BUILD ./services/frontend-service+unit-test --service=frontend-service
    BUILD ./services/frontend-service+unit-test-ui

integration-test-deps:
    FROM alpine/k8s:1.25.15
    RUN wget -O "/usr/bin/argocd" https://github.com/argoproj/argo-cd/releases/download/v2.7.5/argocd-linux-amd64 && \
        echo "a7680140ddb9011c3d282eaff5f5a856be18e8653ff9f0c7047a318f640753be /usr/bin/argocd" | sha256sum -c - && \
        chmod +x "/usr/bin/argocd"
    SAVE ARTIFACT /usr/bin/kubectl
    SAVE ARTIFACT /usr/bin/helm
    SAVE ARTIFACT /usr/bin/argocd

integration-test:
# We pick ubuntu here because it seems to have the least amount of issues.
# With alpine:3.18, we get occasional issues with gpg that says there's a process running already, even though there shouldn't be.
# Ubuntu:22.04 seems to solve this issue.
    FROM ubuntu:22.04
    RUN apt update && apt install --auto-remove -y curl gpg gpg-agent gettext bash git golang netcat-openbsd docker.io
    ARG GO_TEST_ARGS
    # K3S environment variables
    ENV KUBECONFIG=/kp/kubeconfig.yaml
    ENV K3S_TOKEN="Random"
    # Kuberpult/ArgoCd environment variables
    ENV ARGO_NAMESPACE=default
    # Git environment variables
    ENV GIT_NAMESPACE=git
    ENV SSH_HOST_PORT=2222
    ENV GIT_SSH_COMMAND='ssh -o UserKnownHostsFile=emptyfile -o StrictHostKeyChecking=no -i /kp/client'
    ENV GIT_AUTHOR_NAME='Initial Kuberpult Commiter'
    ENV GIT_COMMITTER_NAME='Initial Kuberpult Commiter'
    ENV GIT_AUTHOR_EMAIL='team.sre.permanent+kuberpult-initial-commiter@freiheit.com'
    ENV GIT_COMMITTER_EMAIL='team.sre.permanent+kuberpult-initial-commiter@freiheit.com'
    WORKDIR /kp

    COPY +integration-test-deps/* /usr/bin/
    COPY tests/integration-tests/cluster-setup/docker-compose-k3s.yml .

    RUN --no-cache echo GPG gen starting...
    RUN --no-cache gpg --keyring trustedkeys-kuberpult.gpg --no-default-keyring --batch --passphrase '' --quick-gen-key kuberpult-kind@example.com
    RUN --no-cache echo GPG export starting...
    RUN --no-cache gpg --keyring trustedkeys-kuberpult.gpg --armor --export kuberpult-kind@example.com > /kp/kuberpult-keyring.gpg
    # Note that multiple commands here are writing to "." which is slightly dangerous, because
    # if there are files with the same name, old ones will be overridden.
    COPY charts/kuberpult .
    COPY cd_database/migrations migrations
    COPY infrastructure/scripts/create-testdata/testdata_template/environments environments

    COPY infrastructure/scripts/create-testdata/create-release.sh .
    COPY tests/integration-tests integration-tests
    COPY go.mod go.sum .
    COPY pkg/ptr pkg/ptr

    ARG --required kuberpult_version
    ENV VERSION=$kuberpult_version
    RUN envsubst < Chart.yaml.tpl > Chart.yaml

    WITH DOCKER --compose docker-compose-k3s.yml
        RUN --no-cache \
            echo Waiting for K3s cluster to be ready; \
            sleep 10 && kubectl wait --for=condition=Ready nodes --all --timeout=300s && sleep 3; \
            ./integration-tests/cluster-setup/setup-cluster-ssh.sh; sleep 3; \
            ./integration-tests/cluster-setup/argocd-kuberpult.sh && \
            cd integration-tests && go test $GO_TEST_ARGS ./... && \
            echo ============ SUCCESS ============
    END
