ARG GO_TAG
FROM europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/golang:${GO_TAG} as golang

WORKDIR /kp/

ADD services/cd-service/cmd/server/ /kp/services/cd-service/cmd/server/
ADD services/cd-service/pkg /kp/services/cd-service/pkg
ADD pkg /kp/pkg

RUN go build -o /kp/main /kp/services/cd-service/cmd/server/

FROM alpine:3.18
ARG DIR
LABEL org.opencontainers.image.source https://github.com/freiheit-com/kuberpult
RUN apk --update add ca-certificates tzdata libgit2 git sqlite-libs
ENV TZ=Europe/Berlin
COPY ${DIR}/gitconfig /etc/gitconfig
COPY --from=golang /kp/main /kuberpult/main
CMD ["/kuberpult/main"]
