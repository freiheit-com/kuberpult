FROM golang:1.20.2-alpine3.17 as golang
FROM docker:23.0.2-cli-alpine3.17

COPY --from=golang /usr/local/go/ /usr/local/go/
ENV PATH /usr/local/go/bin:$PATH
RUN apk add --no-cache libgit2 libgit2-dev go protoc make pkgconfig build-base git tar npm sqlite sqlite-dev
RUN npm install -g pnpm@7.30.5
RUN wget https://github.com/bufbuild/buf/releases/download/v1.4.0/buf-Linux-x86_64 -O /usr/local/bin/buf && chmod +x /usr/local/bin/buf
RUN echo '9d38f8d504c01dd19ac9062285ac287f44788f643180545077c192eca9053a2c  /usr/local/bin/buf' | sha256sum -c
