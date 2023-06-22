FROM alpine:3.18

ARG UID=1000

ARG BUF_VERSION=v1.21.0
ARG BUF_BIN_PATH=/usr/local/bin

COPY buf_sha256.txt .

RUN apk --update add --no-cache curl make go git libgit2-dev sqlite sqlite-dev pkgconfig build-base
RUN OS=$(uname) ARCH=$(uname -m) && \
    curl -sSL \
    "https://github.com/bufbuild/buf/releases/download/${BUF_VERSION}/buf-${OS}-${ARCH}" \
    -o "${BUF_BIN_PATH}/buf" && \
    chmod +x "${BUF_BIN_PATH}/buf"

RUN OS=$(uname) ARCH=$(uname -m) && \
    SHA=$(cat buf_sha256.txt | grep "buf-${OS}-${ARCH}$" | cut -d ' ' -f1) && \
    echo "${SHA}  ${BUF_BIN_PATH}/buf" | sha256sum -c

EXPOSE 8080
EXPOSE 8443

RUN adduser --disabled-password --gecos "" --home "/kp" --uid ${UID} kp

RUN chown -R kp:kp /kp

# add the GOPATH for new user to PATH
ENV PATH $PATH:/kp/go/bin

COPY start.sh /kp/start.sh
RUN chmod +x /kp/start.sh

USER kp
CMD [ "/kp/start.sh" ]
