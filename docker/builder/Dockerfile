FROM mikefarah/yq:4.5.0 AS yq
FROM docker:20.10.16-dind-alpine3.15
ADD build.sh build.sh
RUN chmod +x build.sh && ./build.sh && rm build.sh
COPY --from=golang:1.13-alpine /usr/local/go/ /usr/local/go/
COPY --from=yq --chown=root:root /usr/bin/yq /usr/bin/
ENV PATH="/usr/local/go/bin:${PATH}"
