FROM python:3.12-alpine
WORKDIR /deps

RUN apk add envsubst curl git

RUN pip install yamale yamllint

RUN curl --fail --silent --show-error --location --remote-name "https://get.helm.sh/helm-v3.14.2-linux-amd64.tar.gz" && \
    echo 0885a501d586c1e949e9b113bf3fb3290b0bbf74db9444a1d8c2723a143006a5 helm-v3.14.2-linux-amd64.tar.gz | sha256sum -c && \
    tar xzf helm-v3.14.2-linux-amd64.tar.gz && \
    mv linux-amd64/helm /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helm

RUN curl --fail --silent --show-error --location --remote-name "https://github.com/mikefarah/yq/releases/download/v4.41.1/yq_linux_amd64" && \
    echo ce0d5a61c256a463fd32f67f133e0c2948bc2cf77d44c42ff335a40e6bef34bf yq_linux_amd64 | sha256sum -c && \
    mv yq_linux_amd64 /usr/local/bin/yq && \
    chmod +x /usr/local/bin/yq

RUN curl --fail --silent --show-error --location --remote-name "https://github.com/helm/chart-testing/releases/download/v3.10.1/chart-testing_3.10.1_linux_amd64.tar.gz" && \
    echo 6f48140d92d4c4be52b31bf84a8b5d50fb815df721920d6751d4cbccdd3bac2b chart-testing_3.10.1_linux_amd64.tar.gz | sha256sum -c && \
    tar xzf chart-testing_3.10.1_linux_amd64.tar.gz && \
    mv ct /usr/local/bin/ct && \
    chmod +x /usr/local/bin/ct