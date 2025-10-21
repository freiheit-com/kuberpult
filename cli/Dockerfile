ARG BUILDER_IMAGE_TAG
ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder

FROM ${BUILDER_IMAGE}:${BUILDER_IMAGE_TAG} AS builder
FROM alpine:3.21 as compiler

ARG UID=1000

ENV TZ=Europe/Berlin

COPY --from=builder /usr/local/bin/buf /usr/local/bin/buf
COPY --from=builder /usr/local/go /usr/local/go

ENV PATH="/usr/local/go/bin:/kp/go/bin:${PATH}"

RUN adduser --disabled-password --home "/kp" --uid ${UID} kp

RUN chown -R kp:kp /kp

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

USER kp

WORKDIR /kp

COPY --from=builder /kp/go.mod ./go.mod
COPY --from=builder /kp/go.sum ./go.sum

COPY --chown=kp:kp cli cli

RUN cd cli && CGO_ENABLED=false GOOS=linux go build -o /kp/main cmd/kuberpult-client/main.go

FROM scratch
COPY --from=compiler /kp/main /kuberpult-client
ENTRYPOINT ["/kuberpult-client"]
