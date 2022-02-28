FROM alpine:3.15

RUN apk --update add --no-cache libgit2 libgit2-dev go protoc make pkgconfig git cmake yarn gettext
RUN tar --version
RUN wget https://github.com/bufbuild/buf/releases/download/v1.0.0/buf-Linux-x86_64 -O /usr/local/bin/buf && chmod +x /usr/local/bin/buf

RUN wget https://get.helm.sh/helm-v3.8.0-linux-amd64.tar.gz
RUN tar -zxvf helm-v3.8.0-linux-amd64.tar.gz
RUN mv linux-amd64/helm /usr/local/bin/helm
