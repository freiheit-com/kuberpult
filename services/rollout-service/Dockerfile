ARG GIT_VERSION
FROM europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/swissknife:${GIT_VERSION} as swissknife
RUN apk --update add ca-certificates tzdata

WORKDIR /kp/

# cd-service:
ADD services/cd-service/cmd/server/ /kp/services/cd-service/cmd/server/
ADD services/cd-service/pkg /kp/services/cd-service/pkg

# rollout-service:
ADD services/rollout-service/cmd/server/ /kp/services/rollout-service/cmd/server/
ADD services/rollout-service/pkg /kp/services/rollout-service/pkg

# global pkg:
ADD pkg /kp/pkg
COPY go.sum go.mod /kp/

RUN go build -o /kp/main /kp/services/rollout-service/cmd/server/


FROM scratch
LABEL org.opencontainers.image.source https://github.com/freiheit-com/kuberpult
ENV TZ=Europe/Berlin
COPY --from=swissknife /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=swissknife /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=swissknife /kp/main /kuberpult/main
CMD ["/main"]
