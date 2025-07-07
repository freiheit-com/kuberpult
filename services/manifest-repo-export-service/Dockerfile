ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/deps:latest
FROM ${BUILDER_IMAGE} as builder
FROM alpine:3.21
ENV TZ=Europe/Berlin
RUN adduser --disabled-password --home "/kp" --uid 1000 kp
RUN chown -R kp:kp /kp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /kp/database/migrations /migrations
RUN apk add --no-cache ca-certificates tzdata alpine-sdk go make pkgconfig build-base git tar bash cmake g++ musl-dev openssl-dev python3 py3-pip libffi-dev curl libssh2-dev
COPY ./install-libgit2.sh /tmp/install-libgit2.sh
COPY ./gitconfig /etc/gitconfig
RUN /tmp/install-libgit2.sh && rm /tmp/install-libgit2.sh

COPY cmd/server/lib/ /lib
COPY cmd/server/usr/ /usr

USER kp
WORKDIR /kp
COPY ./cmd/server/bin/main /main

CMD /main