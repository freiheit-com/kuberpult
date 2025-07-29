ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:main
FROM ${BUILDER_IMAGE} as builder
FROM alpine:3.21
ARG path=
ENV TZ=Europe/Berlin
RUN adduser --disabled-password --home "/kp" --uid 1000 kp
RUN chown -R kp:kp /kp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY services/cd-service/gitconfig /etc/gitconfig
COPY services/cd-service/policy.csv /etc/policy.csv
COPY services/cd-service/team.csv /etc/team.csv

COPY services/cd-service/cmd/server/lib/ /lib
COPY services/cd-service/cmd/server/usr/ /usr

USER kp
WORKDIR /kp
RUN mkdir -p database/migrations
COPY database/migrations /migrations

COPY services/cd-service/cmd/server/bin/main /main

CMD /main
