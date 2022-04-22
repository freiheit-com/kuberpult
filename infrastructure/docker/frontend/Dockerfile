FROM alpine:3.15

RUN apk --update add --no-cache go make
RUN wget https://github.com/bufbuild/buf/releases/download/v1.0.0/buf-Linux-x86_64 -O /usr/local/bin/buf && chmod +x /usr/local/bin/buf

EXPOSE 8081

RUN adduser --disabled-password --gecos "" --home "/kp" --uid 1000 kp

RUN chown -R kp:kp /kp

# add the GOPATH for new user to PATH
ENV PATH $PATH:/kp/go/bin

COPY start.sh /kp/start.sh
RUN chmod +x /kp/start.sh

USER kp
CMD [ "/kp/start.sh" ]
