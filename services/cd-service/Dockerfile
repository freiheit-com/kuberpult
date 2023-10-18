ARG GIT_VERSION
FROM europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/swissknife:${GIT_VERSION} as swissknife

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
COPY --from=swissknife /kp/main /kuberpult/main
CMD ["/kuberpult/main"]
