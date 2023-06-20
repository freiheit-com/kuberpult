FROM alpine:3.17 AS builder
RUN apk --update add ca-certificates tzdata

FROM scratch
LABEL org.opencontainers.image.source https://github.com/freiheit-com/kuberpult
ENV TZ=Europe/Berlin
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY bin/main /
CMD ["/main"]
