FROM golang:1.17.3-alpine3.15 as golang
FROM docker:20.10.16-dind-alpine3.15
COPY --from=golang /usr/local/go/ /usr/local/go/
ENV PATH /usr/local/go/bin:$PATH
RUN apk add --no-cache libgit2 libgit2-dev go protoc make pkgconfig build-base
