FROM alpine:edge
RUN apk --update add ca-certificates tzdata libgit2
RUN wget https://github.com/argoproj/argo-cd/releases/download/v2.1.2/argocd-linux-amd64 -O /usr/local/bin/argocd && chmod +x /usr/local/bin/argocd
ENV TZ=Europe/Berlin
COPY bin/main /
CMD ["/main"]
