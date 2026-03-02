FROM alpine:3.21
WORKDIR /tmp
ARG TRIVY_VERSION=0.69.2
ARG TRIVY_CHECKSUM=affa59a1e37d86e4b8ab2cd02f0ab2e63d22f1bf9cf6a7aa326c884e25e26ce3
RUN wget "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz"
RUN echo "${TRIVY_CHECKSUM} trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz" | sha256sum -c
RUN tar -xzf trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz && mv trivy /usr/local/bin/trivy && trivy --version
WORKDIR /trivy
COPY templates templates
COPY .trivyignore .trivyignore