ARG BUILDER_IMAGE_TAG
ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder
FROM ${BUILDER_IMAGE}:${BUILDER_IMAGE_TAG} AS builder
FROM alpine:3.21
ENV TZ=Europe/Berlin
COPY --from=builder /usr/local/bin/buf /usr/local/bin/buf
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:/kp/go/bin:${PATH}"
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

COPY --from=builder /kp/go.mod ./go.mod
COPY --from=builder /kp/go.sum ./go.sum

COPY --chown=kp:kp pkg pkg
RUN cd pkg && buf generate
RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
RUN cd pkg &&  oapi-codegen -generate "std-http-server" -o publicapi/server-gen.go -package publicapi publicapi/api.yaml

COPY --chown=kp:kp services/cd-service services/cd-service

RUN cd services/cd-service && CGO_ENABLED=true GOOS=linux go build -o bin/main cmd/server/main.go
RUN ldd services/cd-service/bin/main | tr -s [:blank:] '\n' | grep ^/ | xargs -I % install -D % $(MAIN_PATH)/%

CMD /main
