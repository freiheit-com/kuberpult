ARG PARENT_CONTAINER
FROM europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:${PARENT_CONTAINER} as builder
RUN apk --update add --no-cache curl make nodejs npm ca-certificates tzdata
RUN npm install -g pnpm@8.5.1

# cd-service:
ADD services/cd-service/cmd/server/ /kp/services/cd-service/cmd/server/
ADD services/cd-service/pkg /kp/services/cd-service/pkg

# frontend-service:
ADD services/frontend-service/cmd/server/ /kp/services/frontend-service/cmd/server/
ADD services/frontend-service/pkg /kp/services/frontend-service/pkg
ADD services/frontend-service/package.json /kp/services/frontend-service/
ADD services/frontend-service/src /kp/services/frontend-service/src
ADD services/frontend-service/buf* /kp/services/frontend-service/
ADD services/frontend-service/pnpm-lock.yaml /kp/services/frontend-service/
ADD services/frontend-service/pnpm-workspace.yaml /kp/services/frontend-service/
ADD services/frontend-service/tsconfig.json /kp/services/frontend-service/
ADD services/frontend-service/.prettierrc /kp/services/frontend-service/
ADD services/frontend-service/.nvmrc /kp/services/frontend-service/
ADD services/frontend-service/.npmrc /kp/services/frontend-service/
ADD services/frontend-service/.eslintrc /kp/services/frontend-service/
ADD services/frontend-service/public /kp/services/frontend-service/public
ADD pkg/api /kp/services/frontend-service/api

# global pkg:
ADD pkg /kp/pkg
COPY go.sum go.mod /kp/

# Build go:
WORKDIR /kp/
RUN make -C pkg/api
RUN GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o /kp/main /kp/services/frontend-service/cmd/server/main.go

# Build js:
WORKDIR /kp/services/frontend-service/
RUN pnpm i
RUN mkdir -p src/api
RUN buf generate --path api --timeout 5m
RUN pnpm build

FROM scratch
LABEL org.opencontainers.image.source https://github.com/freiheit-com/kuberpult
ENV TZ=Europe/Berlin
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /kp/main /main
COPY --from=builder /kp/services/frontend-service/build /build
CMD ["/main"]
