VERSION 0.7
deps:
    FROM golang:1.21-bookworm
    WORKDIR /kp
    COPY go.mod go.sum ./
    RUN go mod download
    SAVE ARTIFACT go.mod
    SAVE ARTIFACT go.sum