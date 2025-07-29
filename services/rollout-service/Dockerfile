ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:latest
FROM ${BUILDER_IMAGE} AS builder
FROM alpine:3.21
ENV TZ=Europe/Berlin
ARG CONTEXT
RUN adduser --disabled-password --home "/kp" --uid 1000 kp
RUN chown -R kp:kp /kp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY $CONTEXT/cmd/server/lib/ /lib
COPY $CONTEXT/cmd/server/usr/ /usr

USER kp
WORKDIR /kp
COPY ./$CONTEXT/cmd/server/bin/main /main

CMD /main
