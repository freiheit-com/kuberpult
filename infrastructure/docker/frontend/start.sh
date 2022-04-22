#!/bin/sh

cd /kp/kuberpult

go install -modfile=go.tools.mod \
  github.com/golang/protobuf/protoc-gen-go \
  google.golang.org/grpc/cmd/protoc-gen-go-grpc \
  github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway

cd /kp/kuberpult/services/frontend-service

make run
