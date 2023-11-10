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

integration-test:
    FROM docker:24.0.7-dind-alpine3.18
    RUN apk add --no-cache curl
    RUN adduser -D -h "/kp" --uid 1000 kp
    WORKDIR /kp
    COPY services/cd-service services/cd-service
    COPY docker-compose-earthly.yml ./ 
    RUN chown -R kp:kp services/
    WITH DOCKER --compose docker-compose-earthly.yml \
                --pull europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/kuberpult-cd-service:1.12.0-24-g2e3316a \
                --pull europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/kuberpult-frontend-service:1.12.0-24-g2e3316a
        RUN docker ps -a && sleep 3 && docker ps && docker logs default-cd-service-1; until $(curl --output /dev/null --silent --head --fail localhost:8081/healthz);do echo Waiting for frontend-service && sleep 1;done; \
            until $(curl --output /dev/null --silent --head --fail localhost:8080/healthz);do echo Waiting for cd-service && sleep 1;done; \
            curl localhost:8080; curl localhost:8081; echo SUCCESSFULL
    END