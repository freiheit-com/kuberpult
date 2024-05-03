ARG PARENT_CONTAINER
FROM europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:${PARENT_CONTAINER} as builder

WORKDIR /kp/

RUN apk --no-cache --update add ca-certificates tzdata

# cloudrun-service:
ADD services/cloudrun-service/cmd/server/ /kp/services/cloudrun-service/cmd/server/
ADD services/cloudrun-service/pkg /kp/services/cloudrun-service/pkg

# global pkg:
ADD pkg /kp/pkg
ADD Makefile.variables .
RUN make -C /kp/pkg proto
COPY go.sum go.mod /kp/

RUN GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o /kp/main /kp/services/cloudrun-service/cmd/server/main.go

FROM scratch
LABEL org.opencontainers.image.source https://github.com/freiheit-com/kuberpult
ENV TZ=Europe/Berlin
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /kp/main /main
CMD ["/main"]
