FROM alpine:3.21
WORKDIR /tmp
ARG TRIVY_VERSION=0.50.1
ARG TRIVY_CHECKSUM=4cf907dfe770d81931ee56fb996d285edbf4e0d2308bcfd0f4a3d80e93b0f146
RUN wget "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz"
RUN echo "${TRIVY_CHECKSUM} trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz" | sha256sum -c
RUN tar -xzf trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz && mv trivy /usr/local/bin/trivy && trivy --version
WORKDIR /trivy
COPY templates templates
COPY .trivyignore .trivyignore