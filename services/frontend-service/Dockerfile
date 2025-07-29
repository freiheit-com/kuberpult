ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:main
FROM ${BUILDER_IMAGE} AS builder
RUN apk --update add --no-cache curl make nodejs npm ca-certificates tzdata
RUN npm install -g pnpm@8.5.1
ARG CONTENXT
# frontend-service:
ADD $CONTEXT/cmd/server/ /kp/services/frontend-service/cmd/server/
ADD $CONTEXT/pkg /kp/services/frontend-service/pkg
ADD $CONTEXT/package.json /kp/services/frontend-service/
ADD $CONTEXT/src /kp/services/frontend-service/src
ADD $CONTEXT/buf* /kp/services/frontend-service/
ADD $CONTEXT/pnpm-lock.yaml /kp/services/frontend-service/
ADD $CONTEXT/pnpm-workspace.yaml /kp/services/frontend-service/
ADD $CONTEXT/tsconfig.json /kp/services/frontend-service/
ADD $CONTEXT/.prettierrc /kp/services/frontend-service/
ADD $CONTEXT/.nvmrc /kp/services/frontend-service/
ADD $CONTEXT/.npmrc /kp/services/frontend-service/
ADD $CONTEXT/.eslintrc /kp/services/frontend-service/
ADD $CONTEXT/public /kp/services/frontend-service/public
ADD pkg/api/v1 /kp/services/frontend-service/api
ADD infrastructure /kp/infrastructure

# global pkg:
ADD pkg /kp/pkg
COPY go.sum go.mod /kp/

# Build go:
WORKDIR /kp/
ADD Makefile.variables .
ADD Makefile .
RUN make -C pkg gen SKIP_BUILDER=1 PKG_WITHOUT_DOCKER=1
RUN GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o /kp/main /kp/services/frontend-service/cmd/server/main.go

# Build js:
WORKDIR /kp/services/frontend-service/
RUN pnpm i
RUN mkdir -p src/api
RUN buf generate --path api --timeout 5m
RUN pnpm build

FROM scratch
LABEL org.opencontainers.image.source=https://github.com/freiheit-com/kuberpult
ENV TZ=Europe/Berlin
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /kp/main /main
COPY --from=builder /kp/services/frontend-service/build /build
CMD ["/main"]
