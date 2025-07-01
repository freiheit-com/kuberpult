ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/deps:latest
FROM ${BUILDER_IMAGE}

RUN mv /kp/database/migrations /migrations
COPY ./cmd/server/bin/main /main

CMD /main
