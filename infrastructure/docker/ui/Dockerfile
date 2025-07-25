FROM alpine:3.21

ARG UID=1000

ARG BUF_VERSION=v1.55.1
ARG BUF_BIN_PATH=/usr/local/bin

COPY buf_sha256.txt .

RUN apk --update add --no-cache curl make nodejs npm
RUN npm install -g pnpm@8.5.1
RUN echo "OS=$(uname) ARCH=$(uname -m)" > /tmp/os2.txt
RUN OS=$(uname) ARCH=$(uname -m) && \
    curl -sSL \
    "https://github.com/bufbuild/buf/releases/download/${BUF_VERSION}/buf-${OS}-${ARCH}" \
    -o "./buf-${OS}-${ARCH}" && \
    chmod +x "buf-${OS}-${ARCH}"

RUN OS=$(uname) ARCH=$(uname -m) && cp "./buf-${OS}-${ARCH}" "${BUF_BIN_PATH}/buf"
RUN OS=$(uname) ARCH=$(uname -m) && sha256sum "buf-${OS}-${ARCH}" | sha256sum -c

EXPOSE 3000

RUN adduser --disabled-password --gecos "" --home "/kp" --uid ${UID} kp

RUN chown -R kp:kp /kp

USER kp
CMD [ "sh","-c","cd /kp && pnpm i && pnpm start" ]
