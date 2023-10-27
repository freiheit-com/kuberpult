VERSION 0.7
FROM golang:1.21-bookworm

deps:
    WORKDIR /kp
    COPY go.mod go.sum ./
    RUN go mod download
    SAVE ARTIFACT go.mod
    SAVE ARTIFACT go.sum

all-services:
    ARG UID=1000
    BUILD ./services/cd-service+docker --service=cd-service --UID=$UID
    BUILD ./services/frontend-service+docker --service=frontend-service
    BUILD ./services/frontend-service+docker-ui

cache:
    ARG UID=1000
    BUILD ./services/cd-service+release --service=cd-service --UID=$UID
    BUILD ./services/frontend-service+release --service=frontend-service
    BUILD ./services/frontend-service+release-ui