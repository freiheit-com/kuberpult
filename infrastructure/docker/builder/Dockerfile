FROM docker:23.0.6-cli-alpine3.17

RUN apk add --no-cache make git bash

RUN wget https://github.com/earthly/earthly/releases/download/v0.7.21/earthly-linux-amd64 -O /usr/local/bin/earthly && chmod +x /usr/local/bin/earthly
RUN echo '9f87af7cf2c9caf8cb60647d229234cbc29a5eced4635b241678116e6a8bc3a8  /usr/local/bin/earthly' | sha256sum -c
