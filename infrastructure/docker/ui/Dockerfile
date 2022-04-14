FROM alpine:3.15

RUN apk --update add --no-cache libgit2 libgit2-dev go protoc make pkgconfig git cmake yarn gettext
RUN wget https://github.com/bufbuild/buf/releases/download/v1.0.0/buf-Linux-x86_64 -O /usr/local/bin/buf && chmod +x /usr/local/bin/buf

# adding go/bin to the PATH variable so that golang plug-ins can work.
ENV PATH $PATH:/root/go/bin

EXPOSE 3000

COPY start.sh /start.sh
RUN chmod +x /start.sh
CMD [ "/start.sh" ]