VERSION --wildcard-builds 0.8
FROM busybox
ARG --global UID=1000
ARG --global target=docker
ARG --global DOCKER_REGISTRY_URI="europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult"
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

    COPY database/ database/

    SAVE ARTIFACT database/migrations

cd-service:
    BUILD ./services/cd-service+$target --UID=$UID --service=cd-service

manifest-repo-export-service:
    BUILD ./services/manifest-repo-export-service+$target --UID=$UID --service=manifest-repo-export-service

rollout-service:
    BUILD ./services/rollout-service+$target --UID=$UID --service=rollout-service

frontend-service:
    BUILD ./services/frontend-service+$target --UID=$UID --service=frontend-service

ui:
    BUILD ./services/frontend-service+$target-ui

all-services:
    ARG tag="local"
    BUILD ./pkg+deps
    BUILD ./services/*+docker --tag=$tag --UID=$UID
    BUILD ./services/frontend-service+docker-ui

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
    BUILD ./services/manifest-repo-export-service+unit-test --service=manifest-repo-export-service
    BUILD ./services/rollout-service+unit-test --service=rollout-service
    BUILD ./services/frontend-service+unit-test --service=frontend-service
    BUILD ./services/frontend-service+unit-test-ui

integration-test-deps:
    FROM alpine/k8s:1.25.15
    ARG USERARCH
    RUN wget -O "/usr/bin/argocd" https://github.com/argoproj/argo-cd/releases/download/v2.7.5/argocd-linux-amd64 && \
        echo "a7680140ddb9011c3d282eaff5f5a856be18e8653ff9f0c7047a318f640753be /usr/bin/argocd" | sha256sum -c - && \
        chmod +x "/usr/bin/argocd"

    RUN echo "386eb267e0b1c1f000f1b7924031557402fffc470432dc23b9081fc6962fd69b /usr/bin/minikube" > minikube_amd64.sum
    RUN echo "0b6a17d230b4a605002981f1eba2f5aa3f2153361a1ab000c01e7a95830b40ba /usr/bin/minikube" > minikube_arm64.sum
    RUN wget -O "/usr/bin/minikube" https://github.com/kubernetes/minikube/releases/download/v1.33.1/minikube-linux-${USERARCH} && \
        cat minikube_${USERARCH}.sum | sha256sum -c - && \
        chmod +x "/usr/bin/minikube"
    
    SAVE ARTIFACT /usr/bin/kubectl
    SAVE ARTIFACT /usr/bin/helm
    SAVE ARTIFACT /usr/bin/argocd
    SAVE ARTIFACT /usr/bin/minikube

integration-test:
# We pick ubuntu here because it seems to have the least amount of issues.
# With alpine:3.18, we get occasional issues with gpg that says there's a process running already, even though there shouldn't be.
# Ubuntu:22.04 seems to solve this issue.
    FROM ubuntu:22.04
    RUN apt update && apt install --auto-remove -y curl gpg gpg-agent gettext bash git golang netcat-openbsd docker.io jq

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

    COPY tests/integration-tests/cluster-setup/docker-compose-k3s.yml .

    # Note that multiple commands here are writing to "." which is slightly dangerous, because
    # if there are files with the same name, old ones will be overridden.
    COPY charts/kuberpult .
    COPY database/migrations migrations
    COPY infrastructure/scripts/create-testdata/testdata_template/environments environments

    COPY infrastructure/scripts/create-testdata/create-release.sh .
    COPY tests/integration-tests integration-tests
    COPY go.mod go.sum .
    COPY pkg/conversion  pkg/conversion
    
    COPY +integration-test-deps/* /usr/bin/

    RUN echo GPG gen starting...
    RUN gpg --keyring trustedkeys-kuberpult.gpg --no-default-keyring --batch --passphrase '' --quick-gen-key kuberpult-kind@example.com
    RUN echo GPG export starting...
    RUN gpg --keyring trustedkeys-kuberpult.gpg --armor --export kuberpult-kind@example.com > /kp/kuberpult-keyring.gpg

    ARG GO_TEST_ARGS
    ARG --required kuberpult_version
    ENV VERSION=$kuberpult_version
    RUN envsubst < Chart.yaml.tpl > Chart.yaml

    ENV REGISTRY_URI=$DOCKER_REGISTRY_URI
    ENV ARGOCD_IMAGE_URI="quay.io/argoproj/argocd:v2.7.4"
    ENV DEX_IMAGE_URI="ghcr.io/dexidp/dex:v2.36.0"
    ENV CLOUDSQL_PROXY_IMAGE_URI="gcr.io/cloud-sql-connectors/cloud-sql-proxy:2.11.0"
    ENV REDIS_IMAGE_URI="public.ecr.aws/docker/library/redis:7.0.11-alpine"

    WITH DOCKER --load postgres:local=./infrastructure/docker/postgres+docker \
                --pull europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/git-ssh:1.1.1 \
                --pull ${DOCKER_REGISTRY_URI}/kuberpult-cd-service:${kuberpult_version} \
                --pull ${DOCKER_REGISTRY_URI}/kuberpult-frontend-service:${kuberpult_version} \
                --pull ${DOCKER_REGISTRY_URI}/kuberpult-rollout-service:${kuberpult_version} \
                --pull ${ARGOCD_IMAGE_URI} \
                --pull ${DEX_IMAGE_URI} \
                --pull ${CLOUDSQL_PROXY_IMAGE_URI} \
                --pull ${REDIS_IMAGE_URI}
        RUN --no-cache \
            echo Starting minikube cluster...; \
            minikube start --force; \
            echo Loading images...; \
            ./integration-tests/cluster-setup/load-images.sh && \
            ./integration-tests/cluster-setup/setup-postgres.sh && \
            ./integration-tests/cluster-setup/setup-cluster-ssh.sh && \
            ./integration-tests/cluster-setup/argocd-kuberpult.sh && \
            cd integration-tests && go test $GO_TEST_ARGS ./... && \
            echo ============ SUCCESS ============
    END
