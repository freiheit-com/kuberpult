FROM alpine:3.15

RUN apk --update add --no-cache libgit2 libgit2-dev go protoc make pkgconfig git cmake yarn gettext
RUN tar --version
RUN wget https://github.com/bufbuild/buf/releases/download/v1.4.0/buf-Linux-x86_64 -O /usr/local/bin/buf && chmod +x /usr/local/bin/buf
RUN echo '9d38f8d504c01dd19ac9062285ac287f44788f643180545077c192eca9053a2c  /usr/local/bin/buf' | sha256sum -c

RUN wget https://get.helm.sh/helm-v3.8.0-linux-amd64.tar.gz
RUN tar -zxvf helm-v3.8.0-linux-amd64.tar.gz
RUN mv linux-amd64/helm /usr/local/bin/helm

# adding go/bin to the PATH variable so that golang plug-ins can work.
ENV PATH $PATH:/root/go/bin
