ARG BUILDER_TAG
FROM europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:${BUILDER_TAG} as builder

WORKDIR /kp/

COPY go.mod .
COPY main.go .

RUN go build

FROM scratch
COPY --from=builder /kp/cli .

ENTRYPOINT ["./cli"]