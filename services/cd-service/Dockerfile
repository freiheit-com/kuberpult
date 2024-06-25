ARG PARENT_CONTAINER
FROM europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/builder:${PARENT_CONTAINER} as builder

WORKDIR /kp/

# cd-service
ADD services/cd-service/cmd/server/ /kp/services/cd-service/cmd/server/
ADD services/cd-service/pkg /kp/services/cd-service/pkg

# global:
ADD pkg /kp/pkg
ADD Makefile.variables .
COPY go.sum go.mod /kp/

RUN make -C /kp/pkg proto
RUN go build -o /kp/main /kp/services/cd-service/cmd/server/

FROM alpine:3.18
ARG UID=1000
ARG DIR
# note that this label has influence on githubs access permissions: https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility
LABEL org.opencontainers.image.source https://github.com/freiheit-com/kuberpult
RUN apk --update add ca-certificates tzdata libgit2 git sqlite-libs
RUN adduser --disabled-password --gecos "" --home "/kp" --uid ${UID} kp
RUN chown -R kp:kp /kp
ENV TZ=Europe/Berlin
COPY ${DIR}/gitconfig /etc/gitconfig
COPY --from=builder /kp/main /kuberpult/main
USER kp

# Kuberpult expects to be able to write to "./repository", so we need to define the workdir:
WORKDIR "/kp/"
CMD ["/kuberpult/main"]
