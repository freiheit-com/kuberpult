# syntax=docker/dockerfile:1

# Standalone sandbox for running AI agents.
#
# The workspace is a bind mount of the host checkout (see devcontainer.json),
# so an agent in here reads and writes it directly. Credentials are the
# isolation boundary instead: Claude Code's live in their own volume, and the
# GitHub token is injected as an env var at connection time -- neither is ever
# written into this image. 
FROM debian:trixie-slim

ARG USERNAME=dev
ARG USER_UID=1000
ARG USER_GID=1000

ENV DEBIAN_FRONTEND=noninteractive
ENV LANG=C.UTF-8

# Base OS tooling only -- what mise needs to bootstrap itself, plus a login shell and everyday container basics; the dev toolchain proper comes from mise below.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        dnsutils \
        git \
        iproute2 \
        less \
        sudo \
        vim \
        zsh \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd --gid ${USER_GID} ${USERNAME} \
    && useradd --uid ${USER_UID} --gid ${USER_GID} --create-home --shell /bin/zsh ${USERNAME} \
    && echo "${USERNAME} ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/${USERNAME} \
    && chmod 0440 /etc/sudoers.d/${USERNAME}

# Create the Claude credential volume's mount point inside the image. Docker
# seeds a fresh named volume from whatever the image has at that path, so it
# comes up owned by ${USERNAME} instead of root and no runtime chown is needed.
RUN install -d -o ${USERNAME} -g ${USERNAME} /home/${USERNAME}/.claude

USER ${USERNAME}
WORKDIR /workspace

# Claude Code, via the native installer. 
# Claude Code can also be managed by 'mise', and if your setup uses 'mise' it is arguably
# better if you also use 'mise' to manage claude.
# Using the native installer here in the reference setup so that you can switch out 
# version management for dev tooling inpedendently of the AI harness.
ENV PATH="/home/${USERNAME}/.local/bin:${PATH}"
RUN curl -fsSL https://claude.ai/install.sh | bash

# mise, the package manager for the development toolchain. Everything the
# toolchain needs goes through it; a package it cannot provide is installed by
# hand in this Dockerfile rather than by pulling in a second package manager.
ARG MISE_VERSION=v2026.8.10
RUN curl -fsSL https://mise.run | MISE_VERSION=${MISE_VERSION} sh

# Shims on PATH rather than only `mise activate` in an rc file: shims also work
# in the non-interactive shells that `devcontainer exec`, VS Code tasks and
# agents run in. The zsh hook is layered on top of them so an interactive shell
# picks up a tool change without a reshim.
ENV PATH="/home/${USERNAME}/.local/share/mise/shims:${PATH}"
RUN echo 'eval "$(mise activate zsh)"' >> /home/${USERNAME}/.zshrc

# Install the toolchain at build time so it is part of the image: a fresh
# container then starts without first downloading a Go and a Node distribution.
COPY --chown=${USERNAME}:${USERNAME} mise.toml /home/${USERNAME}/.config/mise/config.toml
RUN mise install && mise reshim && mise cache clear

# claude-user-settings.json and scripts/ are baked into the image because
# /workspace is only a bind mount at container runtime, not something the
# build can read from. Copied last so a settings or script change doesn't
# bust the cache for the slower installs above.
COPY claude-user-settings.json /usr/local/share/devcontainer/claude-user-settings.json
COPY --chmod=0755 scripts/ /usr/local/bin/
