FROM golang:1.24.2-alpine3.21 as golang
FROM docker:28.0.2-cli-alpine3.21

COPY --from=golang /usr/local/go/ /usr/local/go/
ENV PATH /usr/local/go/bin:$PATH
RUN apk add --no-cache go protoc make pkgconfig build-base git tar npm sqlite sqlite-dev bash cmake g++ musl-dev openssl-dev python3 py3-pip libffi-dev curl
RUN git clone https://github.com/libgit2/libgit2.git && \
    cd libgit2 && \
    git checkout v1.5.0 && \
    mkdir build && cd build && \
    cmake .. -DCMAKE_INSTALL_PREFIX=/usr -DBUILD_SHARED_LIBS=ON -DUSE_SSH=ON && \
    cmake --build . --target install && \
    cd ../.. && rm -rf libgit2

RUN npm install -g pnpm@8.5.1
RUN wget https://github.com/bufbuild/buf/releases/download/v1.23.1/buf-Linux-x86_64 -O /usr/local/bin/buf && chmod +x /usr/local/bin/buf
RUN echo 'b1f3484cfc092bc14e1f886ff2116a98562bf29df0be2130bbaac128eca0b628  /usr/local/bin/buf' | sha256sum -c

RUN wget https://github.com/earthly/earthly/releases/download/v0.8.4/earthly-linux-amd64 -O /usr/local/bin/earthly && chmod +x /usr/local/bin/earthly
RUN echo '1090e7b9fb65c0b9d3ce680dad2430dff2e0919a56f98de27156c3b9aa65f43d  /usr/local/bin/earthly' | sha256sum -c