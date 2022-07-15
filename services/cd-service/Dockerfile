FROM alpine:3.15
LABEL org.opencontainers.image.source https://github.com/freiheit-com/kuberpult
RUN apk --update add ca-certificates tzdata libgit2 git
ENV TZ=Europe/Berlin
COPY bin/main /
CMD ["/main"]
