ARG BUILDER_IMAGE_TAG
ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder

FROM ${BUILDER_IMAGE}:${BUILDER_IMAGE_TAG} AS builder
FROM alpine:3.21 AS compiler

ARG UID=1000

ENV TZ=Europe/Berlin

COPY --from=builder /usr/local/go /usr/local/go

ENV PATH="/usr/local/go/bin:/kp/go/bin:${PATH}"

RUN apk add protoc protobuf-dev

RUN adduser --disabled-password --home "/kp" --uid ${UID} kp
RUN chown -R kp:kp /kp

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY ./services/reposerver-service/cmd/server/lib/ /lib
COPY ./services/reposerver-service/cmd/server/usr/ /usr

USER kp
WORKDIR /kp

COPY --from=builder /kp/go.mod ./go.mod
COPY --from=builder /kp/go.sum ./go.sum

COPY --chown=kp:kp pkg pkg

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
RUN cd pkg && protoc --go_opt=paths=source_relative --go_out=. --go-grpc_opt=paths=source_relative,require_unimplemented_servers=false --go-grpc_out=. api/v1/api.proto

RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
RUN cd pkg && oapi-codegen -generate "std-http-server" -o publicapi/server-gen.go -package publicapi publicapi/api.yaml

COPY --chown=kp:kp services/reposerver-service services/reposerver-service

RUN mkdir -p /bin
RUN cd services/reposerver-service && CGO_ENABLED=false GOOS=linux go build -o /tmp/main cmd/server/main.go

USER root
RUN mv /tmp/main /main
RUN chmod +x /main

FROM alpine:3.21

ARG UID=1000

RUN adduser --disabled-password --home "/kp" --uid ${UID} kp

COPY --from=compiler /main /main
COPY --from=compiler /lib /lib

USER kp

WORKDIR /kp

CMD ["/main"]
