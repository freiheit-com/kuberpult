ARG BUILDER_IMAGE_TAG
ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder

FROM ${BUILDER_IMAGE}:${BUILDER_IMAGE_TAG} AS builder

ENV PATH="/usr/local/go/bin:/kp/go/bin:${PATH}"

RUN apk add protoc protobuf-dev

# global pkg:
COPY --chown=kp:kp pkg pkg

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
RUN cd pkg && oapi-codegen -generate "std-http-server" -o publicapi/server-gen.go -package publicapi publicapi/api.yaml
RUN cd pkg && protoc --go_opt=paths=source_relative --go_out=. --go-grpc_opt=paths=source_relative,require_unimplemented_servers=false --go-grpc_out=. api/v1/api.proto

# frontend-service:
# js:
ADD services/frontend-service/package.json /kp/services/frontend-service/
ADD services/frontend-service/pnpm-lock.yaml /kp/services/frontend-service/
ADD services/frontend-service/pnpm-workspace.yaml /kp/services/frontend-service/
ADD services/frontend-service/tsconfig.json /kp/services/frontend-service/
ADD services/frontend-service/.prettierrc /kp/services/frontend-service/
ADD services/frontend-service/.nvmrc /kp/services/frontend-service/
ADD services/frontend-service/.npmrc /kp/services/frontend-service/
ADD services/frontend-service/.eslintrc /kp/services/frontend-service/

# go:
WORKDIR /kp/
COPY go.sum go.mod .
RUN go mod download
ADD services/frontend-service/cmd/server/ /kp/services/frontend-service/cmd/server/
ADD services/frontend-service/pkg /kp/services/frontend-service/pkg
ADD services/frontend-service/src /kp/services/frontend-service/src
ADD services/frontend-service/buf* /kp/services/frontend-service/
ADD services/frontend-service/public /kp/services/frontend-service/public
ADD pkg/api/v1 /kp/services/frontend-service/api
ADD infrastructure /kp/infrastructure


# Build go:
WORKDIR /kp/
ADD Makefile.variables .
ADD Makefile .
RUN make -C pkg gen SKIP_BUILDER=1 PKG_WITHOUT_DOCKER=1
RUN GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o /kp/main /kp/services/frontend-service/cmd/server/main.go

# Build js:
RUN cd /kp/services/frontend-service && pnpm i
WORKDIR /kp/services/frontend-service/
RUN mkdir -p src/api
RUN protoc --plugin=./node_modules/.bin/protoc-gen-ts_proto --ts_proto_opt=oneof=unions,useOptionals=messages,esModuleInterop=true,outputClientImpl=grpc-web --ts_proto_out=src api/api.proto
RUN pnpm build

FROM scratch
LABEL org.opencontainers.image.source=https://github.com/freiheit-com/kuberpult

ENV TZ=Europe/Berlin

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /kp/main /main
COPY --from=builder /kp/services/frontend-service/build /build
COPY --from=builder /kp/go.mod /go.mod
COPY --from=builder /kp/go.sum /go.sum

CMD ["/main"]
