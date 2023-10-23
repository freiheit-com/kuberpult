ARG PARENT_CONTAINER
FROM europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:${PARENT_CONTAINER} as builder

WORKDIR /kp/

RUN apk --update add ca-certificates tzdata

# cd-service:
ADD services/cd-service/cmd/server/ /kp/services/cd-service/cmd/server/
ADD services/cd-service/pkg /kp/services/cd-service/pkg

# rollout-service:
ADD services/rollout-service/cmd/server/ /kp/services/rollout-service/cmd/server/
ADD services/rollout-service/pkg /kp/services/rollout-service/pkg

# global pkg:
ADD pkg /kp/pkg
COPY go.sum go.mod /kp/

RUN GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o /kp/main /kp/services/rollout-service/cmd/server/main.go

FROM scratch
LABEL org.opencontainers.image.source https://github.com/freiheit-com/kuberpult
ENV TZ=Europe/Berlin
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /kp/main /main
CMD ["/main"]
