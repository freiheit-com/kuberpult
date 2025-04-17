FROM alpine:3.21

RUN apk --update add --no-cache go protoc make pkgconfig git cmake gettext cmake g++ musl-dev openssl-dev python3 py3-pip libffi-dev curl libssh2-dev
RUN git clone https://github.com/libgit2/libgit2.git && \
    cd libgit2 && \
    git checkout v1.5.0 && \
    mkdir build && cd build && \
    cmake .. -DCMAKE_INSTALL_PREFIX=/usr -DBUILD_SHARED_LIBS=ON -DUSE_SSH=ON && \
    cmake --build . --target install && \
    cd ../.. && rm -rf libgit2

RUN tar --version
RUN wget https://github.com/bufbuild/buf/releases/download/v1.4.0/buf-Linux-x86_64 -O /usr/local/bin/buf && chmod +x /usr/local/bin/buf
RUN echo '9d38f8d504c01dd19ac9062285ac287f44788f643180545077c192eca9053a2c  /usr/local/bin/buf' | sha256sum -c

RUN wget https://get.helm.sh/helm-v3.8.0-linux-amd64.tar.gz
RUN tar -zxvf helm-v3.8.0-linux-amd64.tar.gz
RUN mv linux-amd64/helm /usr/local/bin/helm

# adding go/bin to the PATH variable so that golang plug-ins can work.
ENV PATH $PATH:/root/go/bin
