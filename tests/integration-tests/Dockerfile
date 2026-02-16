FROM alpine/k8s:1.30.14 AS kube_deps
RUN wget -O "/usr/bin/argocd" https://github.com/argoproj/argo-cd/releases/download/v2.7.5/argocd-linux-amd64 && \
    echo "a7680140ddb9011c3d282eaff5f5a856be18e8653ff9f0c7047a318f640753be /usr/bin/argocd" | sha256sum -c - && \
    chmod +x "/usr/bin/argocd"
FROM golang:1.25-bookworm
RUN apt update && apt install --auto-remove -y curl gpg gpg-agent gettext bash git golang netcat-openbsd docker.io postgresql-client
ARG GO_TEST_ARGS=""
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
COPY --from=kube_deps /usr/bin/kubectl /usr/bin/kubectl
COPY --from=kube_deps /usr/bin/helm /usr/bin/helm
COPY --from=kube_deps /usr/bin/argocd /usr/bin/argocd
COPY tests/integration-tests/cluster-setup/docker-compose-k3s.yml .
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download
RUN gpg --keyring trustedkeys-kuberpult.gpg --no-default-keyring --batch --passphrase '' --quick-gen-key kuberpult-kind@example.com
RUN gpg --keyring trustedkeys-kuberpult.gpg --armor --export kuberpult-kind@example.com > /kp/kuberpult-keyring.gpg

COPY charts/kuberpult .
COPY infrastructure/scripts/create-testdata/testdata_template/environments environments

COPY infrastructure/scripts/create-testdata/create-release.sh .
COPY infrastructure/scripts/create-testdata/create-environments.sh .
COPY infrastructure/scripts/create-testdata/testdata_template/environments ./testdata_template/environments
COPY tests/integration-tests tests/integration-tests
COPY tests/integration-tests/docker_entrypoint.sh docker_entrypoint.sh
COPY pkg pkg
COPY database/migrations /migrations

ARG kuberpult_version=""
ENV VERSION=$kuberpult_version
ARG charts_version=""
ENV CHARTS_VERSION=$charts_version
RUN envsubst < Chart.yaml.tpl > Chart.yaml
RUN chmod +x docker_entrypoint.sh

ENTRYPOINT [ "/kp/docker_entrypoint.sh" ]


