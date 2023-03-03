FROM alpine:3.15
# renovate: datasource=github-releases depName=bufbuild/buf
ENV BUFF_VERSION=v1.7.0
RUN apk --update add --no-cache curl make go git libgit2-dev pkgconfig build-base
RUN BIN="/usr/local/bin" && \
  curl -sSL \
    "https://github.com/bufbuild/buf/releases/download/v1.7.0/buf-Linux-x86_64" \
    -o "${BIN}/buf" && \
  chmod +x "${BIN}/buf"

RUN echo 'ab8f1808a9add5e0578d9f0b45bc3b9e01d7f4192023328b1041614cf792076f  /usr/local/bin/buf' | sha256sum -c

EXPOSE 8080
EXPOSE 8443

RUN adduser --disabled-password --gecos "" --home "/kp" --uid 1000 kp

RUN chown -R kp:kp /kp

# add the GOPATH for new user to PATH
ENV PATH $PATH:/kp/go/bin

COPY start.sh /kp/start.sh
RUN chmod +x /kp/start.sh

USER kp
CMD [ "/kp/start.sh" ]
