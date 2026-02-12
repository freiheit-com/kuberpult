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

USER kp
WORKDIR /kp

COPY --from=builder /kp/go.mod ./go.mod
COPY --from=builder /kp/go.sum ./go.sum
RUN go mod download

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY services/cd-service/gitconfig /etc/gitconfig
COPY services/cd-service/policy.csv /etc/policy.csv
COPY services/cd-service/team.csv /etc/team.csv

COPY services/cd-service/cmd/server/lib/ /lib
COPY services/cd-service/cmd/server/usr/ /usr

COPY --chown=kp:kp pkg pkg

RUN cd pkg && protoc --go_opt=paths=source_relative --go_out=. --go-grpc_opt=paths=source_relative,require_unimplemented_servers=false --go-grpc_out=. api/v1/api.proto
RUN cd pkg && oapi-codegen -generate "std-http-server" -o publicapi/server-gen.go -package publicapi publicapi/api.yaml

COPY --chown=kp:kp services/cd-service services/cd-service

RUN mkdir -p /bin
RUN cd services/cd-service && CGO_ENABLED=false GOOS=linux go build -o /tmp/main cmd/server/main.go

USER root
RUN mv /tmp/main /main
RUN chmod +x /main

CMD ["/main"]

FROM alpine:3.21

ARG UID=1000

WORKDIR /kp

RUN adduser --disabled-password --home "/kp" --uid ${UID} kp

COPY --from=compiler /main /main
COPY --from=compiler /lib /lib

RUN mkdir -p database/migrations
COPY database/migrations /migrations

USER kp

CMD ["/main"]
