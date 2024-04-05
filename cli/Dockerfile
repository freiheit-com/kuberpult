ARG PARENT_CONTAINER
FROM europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:${PARENT_CONTAINER} as builder

WORKDIR /kp/

COPY go.mod .
COPY main.go .

RUN go build

FROM scratch
COPY --from=builder /kp/cli .

ENTRYPOINT ["./cli"]