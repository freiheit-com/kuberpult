FROM alpine:3.18

LABEL maintainer="hannesg,sven-urbanski-freiheit-com"
LABEL name="git-ssh"
LABEL description="Git over SSH with UID/GID handling"

RUN apk add --no-cache \
        git \
        openssh

WORKDIR /git/

# The git-shell permits execution only of server-side Git commands implementing
# the pull/push functionality, plus custom commands present in a subdirectory
# named `git-shell-commands` in the user’s home directory.
# [More info](https://git-scm.com/docs/git-shell)
COPY ./git-shell-commands/ ./git-shell-commands/

COPY ./fix-repos.sh ./sshd_config ./
COPY ./start.sh /
COPY ./check.sh /
RUN echo '' > /etc/motd && \
    chmod u+x \
        ./git-shell-commands/* \
        ./fix-repos.sh \
        /start.sh \
        /check.sh && \
    mkdir -p \
        ./.ssh/ \
        ./keys-host/ \
        ./keys/ \
        ./repos/

EXPOSE 22
VOLUME ["/git/keys-host/", "/git/keys/", "/git/repos/"]

HEALTHCHECK CMD sh /check.sh

CMD ["sh", "/start.sh"]
