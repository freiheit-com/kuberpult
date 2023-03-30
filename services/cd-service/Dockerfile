FROM alpine:3.17
LABEL org.opencontainers.image.source https://github.com/freiheit-com/kuberpult
RUN apk --update add ca-certificates tzdata libgit2 git sqlite
ENV TZ=Europe/Berlin
COPY gitconfig /etc/gitconfig
COPY bin/main /
CMD ["/main"]
