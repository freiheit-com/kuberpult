ARG BUILDER_IMAGE_TAG
ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder
FROM ${BUILDER_IMAGE}:${BUILDER_IMAGE_TAG} AS builder
FROM alpine:3.21
ENV TZ=Europe/Berlin
COPY --from=builder /usr/local/bin/buf /usr/local/bin/buf
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:/kp/go/bin:${PATH}"
RUN adduser --disabled-password --home "/kp" --uid 1000 kp
RUN chown -R kp:kp /kp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo


COPY ./services/reposerver-service/cmd/server/lib/ /lib
COPY ./services/reposerver-service/cmd/server/usr/ /usr

USER kp
WORKDIR /kp
#RUN sh -c 'test -n "$(MAIN_PATH)" || exit 0; cd $(MAIN_PATH) && CGO_ENABLED=$(CGO_ENABLED) GOOS=linux go build -o bin/main . && cd ../.. && if [ "$(CGO_ENABLED)" = "1" ]; then ldd $(MAIN_PATH)/bin/main | tr -s [:blank:] '\n' | grep ^/ | xargs -I % install -D % $(MAIN_PATH)/%; fi'
#COPY ./services/reposerver-service/cmd/server/bin/main /main


COPY --from=builder /kp/go.mod ./go.mod
COPY --from=builder /kp/go.sum ./go.sum

COPY --chown=kp:kp pkg pkg
RUN cd pkg && buf generate
RUN mkdir -p /kp/pkg/publicapi
RUN go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
RUN cd pkg && oapi-codegen -generate "std-http-server" -o publicapi/server-gen.go -package publicapi publicapi/api.yaml

# TODO SU: do not copy main!
COPY --chown=kp:kp services/reposerver-service services/reposerver-service
RUN echo hello
RUN pwd

RUN cd services/reposerver-service && CGO_ENABLED=false GOOS=linux go build -o bin/main cmd/server/main.go
RUN ldd services/reposerver-service/bin/main | tr -s [:blank:] '\n' | grep ^/ | xargs -I % install -D % $(MAIN_PATH)/%

RUN echo hello world


CMD /main