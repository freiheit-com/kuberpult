FROM golang:1.17-alpine
WORKDIR /execution-plan
COPY * ./
RUN go mod download
RUN go test ./...
RUN go build -o ./execution-plan

FROM alpine:3.15
RUN apk add make
VOLUME ["/repo"]
WORKDIR /repo
COPY --from=0 /execution-plan/execution-plan /execution-plan
ENTRYPOINT ["/execution-plan"]
