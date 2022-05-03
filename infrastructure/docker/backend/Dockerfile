FROM alpine:3.15

RUN apk --update add --no-cache make go git libgit2-dev pkgconfig build-base
RUN wget https://github.com/bufbuild/buf/releases/download/v1.4.0/buf-Linux-x86_64 -O /usr/local/bin/buf && chmod +x /usr/local/bin/buf
RUN echo '9d38f8d504c01dd19ac9062285ac287f44788f643180545077c192eca9053a2c  /usr/local/bin/buf' | sha256sum -c

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
