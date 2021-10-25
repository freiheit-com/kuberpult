FROM alpine:edge
RUN apk add --no-cache libgit2 libgit2-dev go protoc make pkgconfig git
RUN wget https://github.com/argoproj/argo-cd/releases/download/v2.1.2/argocd-linux-amd64 -O /usr/local/bin/argocd && chmod +x /usr/local/bin/argocd
