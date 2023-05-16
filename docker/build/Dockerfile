FROM alpine:3.18
LABEL org.opencontainers.image.source https://github.com/freiheit-com/kuberpult
RUN apk add --no-cache libgit2 libgit2-dev go protoc make pkgconfig git sqlite sqlite-dev
RUN wget https://github.com/argoproj/argo-cd/releases/download/v2.1.2/argocd-linux-amd64 -O /usr/local/bin/argocd && chmod +x /usr/local/bin/argocd
