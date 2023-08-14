FROM alpine:3.18
LABEL org.opencontainers.image.source https://github.com/freiheit-com/kuberpult
RUN apk --update add ca-certificates tzdata libgit2 git sqlite-libs
RUN wget https://github.com/argoproj/argo-cd/releases/download/v2.1.2/argocd-linux-amd64 -O /usr/local/bin/argocd && chmod +x /usr/local/bin/argocd
ENV TZ=Europe/Berlin
COPY gitconfig /etc/gitconfig
COPY bin/main /
CMD ["/main"]
