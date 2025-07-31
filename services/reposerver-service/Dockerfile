ARG BUILDER_IMAGE_TAG
ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder
FROM ${BUILDER_IMAGE}:${BUILDER_IMAGE_TAG} AS builder
FROM alpine:3.21
ENV TZ=Europe/Berlin
COPY --from=builder /usr/local/bin/buf /usr/local/bin/buf
ENV PATH="/usr/local/go/bin:/kp/go/bin:${PATH}"
RUN adduser --disabled-password --home "/kp" --uid 1000 kp
RUN chown -R kp:kp /kp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo


COPY ./services/reposerver-service/cmd/server/lib/ /lib
COPY ./services/reposerver-service/cmd/server/usr/ /usr

USER kp
WORKDIR /kp
COPY ./services/reposerver-service/cmd/server/bin/main /main
COPY --from=builder /kp/go.mod ./go.mod
COPY --from=builder /kp/go.sum ./go.sum

COPY --chown=kp:kp pkg pkg
RUN cd pkg && buf generate
RUN mkdir -p /kp/pkg/publicapi
RUN cd pkg && oapi-codegen -generate "std-http-server" -o publicapi/server-gen.go -package publicapi publicapi/api.yaml
CMD /main