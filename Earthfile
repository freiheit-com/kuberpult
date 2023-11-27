VERSION 0.7
FROM busybox
ARG --global UID=1000
ARG --global target=docker

deps:
    ARG USERARCH
    ARG BUF_VERSION=v1.26.1
    ARG BUF_BIN_PATH=/usr/local/bin

    IF [ "$USERARCH" = "arm64" ]
        FROM golang:1.21-bookworm
        RUN apt update && apt install --auto-remove ca-certificates tzdata libgit2-dev libsqlite3-dev -y
    ELSE
        FROM golang:1.21-alpine3.18
        RUN apk add --no-cache ca-certificates tzdata libgit2-dev sqlite-dev alpine-sdk
    END
    
    WORKDIR /kp

    COPY go.mod go.sum ./
    RUN go mod download

    COPY buf_sha256.txt .
    RUN OS=Linux ARCH=$(uname -m) && \
        wget "https://github.com/bufbuild/buf/releases/download/${BUF_VERSION}/buf-${OS}-${ARCH}" \
        -O "${BUF_BIN_PATH}/buf" && \
        chmod +x "${BUF_BIN_PATH}/buf"
    RUN OS=Linux ARCH=$(uname -m) && \
        SHA=$(cat buf_sha256.txt | grep "buf-${OS}-${ARCH}$" | cut -d ' ' -f1) && \
        echo "${SHA}  ${BUF_BIN_PATH}/buf" | sha256sum -c

    SAVE ARTIFACT go.mod
    SAVE ARTIFACT go.sum
    SAVE ARTIFACT buf_sha256.txt
    SAVE ARTIFACT $BUF_BIN_PATH/buf

cd-service:
    BUILD ./services/cd-service+$target --UID=$UID --service=cd-service

rollout-service:
    BUILD ./services/rollout-service+$target --UID=$UID --service=rollout-service

frontend-service:
    BUILD ./services/frontend-service+$target --UID=$UID --service=frontend-service

ui:
    BUILD ./services/frontend-service+$target-ui --UID=$UID

all-services:
    BUILD ./services/cd-service+docker --service=cd-service --UID=$UID
    BUILD ./services/frontend-service+docker --service=frontend-service
    BUILD ./services/frontend-service+docker-ui

cache:
    BUILD ./services/cd-service+release --service=cd-service --UID=$UID
    BUILD ./services/rollout-service+release --service=rollout-service --UID=$UID
    BUILD ./services/frontend-service+release --service=frontend-service
    BUILD ./services/frontend-service+release-ui

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
    FROM docker:24.0.7-dind-alpine3.18

    ARG --required kuberpult_version

    # K3S environment variables
    ENV KUBECONFIG=/kp/kubeconfig.yaml
    ENV K3S_TOKEN="Random"

    # Kuberpult/ArgoCd environment variables
    ENV VERSION=$kuberpult_version
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

    RUN apk add --no-cache curl gpg gpg-agent gettext bash git go

    COPY +integration-test-deps/* /usr/bin/
    COPY tests/integration-tests/cluster-setup/docker-compose-k3s.yml .
    COPY charts/kuberpult .
    COPY infrastructure/scripts/create-testdata/testdata_template/environments environments
    COPY infrastructure/scripts/create-testdata/create-release.sh .
    COPY tests/integration-tests integration-tests
    COPY go.mod go.sum .
    COPY pkg/ptr pkg/ptr
    
    RUN envsubst < Chart.yaml.tpl > Chart.yaml
    RUN envsubst < values.yaml.tpl > values.yaml

    RUN gpg --keyring trustedkeys-kuberpult.gpg --no-default-keyring --batch --passphrase '' --quick-gen-key kuberpult-kind@example.com
    RUN gpg --keyring trustedkeys-kuberpult.gpg --armor --export kuberpult-kind@example.com > kuberpult-keyring.gpg
    
    WITH DOCKER --compose docker-compose-k3s.yml
        RUN --no-cache \
            echo Waiting for K3s cluster to be ready; \
            sleep 10 && kubectl wait --for=condition=Ready nodes --all --timeout=300s && sleep 3; \
            ./integration-tests/cluster-setup/setup-cluster-ssh.sh; sleep 3; \
            echo Waiting for git server to be ready; \
            kubectl wait --for=condition=Ready pods -l app=git-server -n $GIT_NAMESPACE --timeout=60s || exit 1; \
            ./integration-tests/cluster-setup/argocd-kuberpult.sh && \
            cd integration-tests && go test ./... && \
            echo ============ SUCCESS ============
    END