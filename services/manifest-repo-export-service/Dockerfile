ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/deps:latest
FROM ${BUILDER_IMAGE} as builder
FROM alpine:3.21
ENV TZ=Europe/Berlin
RUN adduser --disabled-password --home "/kp" --uid 1000 kp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /kp/database/migrations /migrations
RUN apk add --no-cache ca-certificates tzdata alpine-sdk go make pkgconfig build-base git tar bash cmake g++ musl-dev openssl-dev python3 py3-pip libffi-dev curl libssh2-dev
COPY ./install-libgit2.sh /tmp/install-libgit2.sh
COPY ./gitconfig /etc/gitconfig
RUN /tmp/install-libgit2.sh && rm /tmp/install-libgit2.sh

COPY cmd/server/lib/ /lib
COPY cmd/server/usr/ /usr
# --- FIX IS HERE ---
# 1. Explicitly create the directory that the application needs to write to.
#    The -p flag ensures parent directories are also created if they don't exist.
RUN mkdir -p /kp/kuberpult/repository_remote/

# 2. Change ownership of the user's entire home directory and all its contents.
#    This is done just before switching to the non-root user to ensure all
#    previously created files and directories have the correct owner.
RUN chown -R kp:kp /kp
# --- END OF FIX ---
USER kp
WORKDIR /kp
COPY ./cmd/server/bin/main /main

CMD /main