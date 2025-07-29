ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:latest
FROM ${BUILDER_IMAGE} AS builder
FROM alpine:3.21
ENV TZ=Europe/Berlin
COPY --from=builder /usr/local/bin/buf /usr/local/bin/buf
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:/kp/go/bin:${PATH}"
RUN adduser --disabled-password --home "/kp" --uid 1000 kp
RUN chown -R kp:kp /kp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /kp/go.mod /go.mod
COPY --from=builder /kp/go.sum /go.sum

COPY services/rollout-service/cmd/server/lib/ /lib
COPY services/rollout-service/cmd/server/usr/ /usr

USER kp
WORKDIR /kp
COPY ./services/rollout-service/cmd/server/bin/main /main

COPY --chown=kp:kp pkg pkg
RUN cd pkg && buf generate
RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
RUN cd pkg &&  oapi-codegen -generate "std-http-server" -o publicapi/server-gen.go -package publicapi publicapi/api.yaml

CMD /main
