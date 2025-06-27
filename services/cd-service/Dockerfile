ARG BUILDER_IMAGE=europe-west3-docker.pkg.dev/fdc-public-docker-registry/kuberpult/infrastructure/docker/deps:latest
FROM ${BUILDER_IMAGE}

COPY gitconfig /etc/gitconfig
COPY policy.csv /etc/policy.csv
COPY team.csv /etc/team.csv

RUN mv /kp/database/migrations /migrations
COPY ./cmd/server/bin/main /main

CMD /main
