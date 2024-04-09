FROM golang:1.21.4-alpine3.17 as builder

WORKDIR /kp

COPY go.mod go.mod
COPY go.sum go.sum
COPY pkg pkg
COPY cmd cmd

RUN mkdir -p bin

RUN go build -o bin ./...

FROM scratch

COPY --from=builder /kp/bin/kuberpult-client .

ENTRYPOINT ["./kuberpult-client"]
