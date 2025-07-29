ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:main
FROM ${BUILDER_IMAGE} AS builder
FROM alpine:3.21
ARG CONTEXT
RUN apk add --no-cache ca-certificates tzdata alpine-sdk go make pkgconfig build-base git tar npm sqlite sqlite-dev bash cmake g++ musl-dev openssl-dev python3 py3-pip libffi-dev curl libssh2-dev
COPY ./$CONTEXT/install-libgit2.sh /tmp/install-libgit2.sh
COPY ./$CONTEXT/gitconfig /etc/gitconfig
RUN /tmp/install-libgit2.sh && rm /tmp/install-libgit2.sh
ENV TZ=Europe/Berlin
RUN adduser --disabled-password --home "/kp" --uid 1000 kp
RUN chown -R kp:kp /kp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY ./$CONTEXT/cmd/server/lib/ /lib
COPY ./$CONTEXT/cmd/server/usr/ /usr

USER kp
WORKDIR /kp
RUN mkdir -p database/migrations
COPY database/migrations /migrations
COPY ./$CONTEXT/cmd/server/bin/main /main

CMD /main