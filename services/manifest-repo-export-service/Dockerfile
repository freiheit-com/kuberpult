ARG BUILDER_IMAGE_TAG
ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder

FROM ${BUILDER_IMAGE}:${BUILDER_IMAGE_TAG} AS builder
FROM alpine:3.22.1 AS compiler

RUN apk add --no-cache  \
        ca-certificates tar tzdata curl bash git \
        gcc g++ pkgconfig make cmake \
        go \
        python3 py3-pip \
        musl-dev libffi-dev openssl-dev libssh2-dev \
        sqlite sqlite-dev

ARG UID=1000

COPY --from=builder /usr/local/bin/buf /usr/local/bin/buf
COPY --from=builder /usr/local/go /usr/local/go

ENV PATH="/usr/local/go/bin:/kp/go/bin:${PATH}"

COPY services/manifest-repo-export-service/install-libgit2.sh /tmp/install-libgit2.sh
COPY services/manifest-repo-export-service/gitconfig /etc/gitconfig
RUN /tmp/install-libgit2.sh && rm /tmp/install-libgit2.sh

ENV TZ=Europe/Berlin
RUN adduser --disabled-password --home "/kp" --uid ${UID} kp
RUN chown -R kp:kp /kp

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY services/manifest-repo-export-service/cmd/server/lib/ /lib
COPY services/manifest-repo-export-service/cmd/server/usr/ /usr

USER kp
WORKDIR /kp

COPY --from=builder /kp/go.mod ./go.mod
COPY --from=builder /kp/go.sum ./go.sum

COPY --chown=kp:kp pkg pkg
RUN cd pkg && buf generate
RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
RUN cd pkg &&  oapi-codegen -generate "std-http-server" -o publicapi/server-gen.go -package publicapi publicapi/api.yaml

COPY --chown=kp:kp services/manifest-repo-export-service services/manifest-repo-export-service

RUN mkdir -p /bin
RUN cd services/manifest-repo-export-service && CGO_ENABLED=true GOOS=linux go build -o /tmp/main cmd/server/main.go

USER root
RUN mv /tmp/main /main
RUN ldd /main | awk '/=>/ {print $3}' | xargs -r -I '{}' sh -c 'if [ ! -e "/lib/$(basename {})" ]; then cp -v "{}" /lib/; fi'
RUN chmod +x /main

FROM alpine:3.22.1

ARG UID=1000

WORKDIR /kp

RUN adduser --disabled-password --home "/kp" --uid ${UID} kp

COPY --from=compiler /main /main
COPY --from=compiler /lib /lib

RUN chown -R kp:kp /kp

RUN mkdir -p database/migrations
COPY database/migrations /migrations

USER kp

CMD ["/main"]
