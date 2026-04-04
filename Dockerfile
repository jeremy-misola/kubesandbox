FROM alpine/k8s:1.32.13

USER root

RUN set -eux; \
    alpine_version="$(cut -d. -f1,2 /etc/alpine-release)"; \
    echo "https://dl-cdn.alpinelinux.org/alpine/v${alpine_version}/community" >> /etc/apk/repositories; \
    apk add --no-cache \
      ttyd \
      bash \
      ca-certificates \
      curl \
      git \
      vim \
      less \
      openssh-client

EXPOSE 8080

ENTRYPOINT ["ttyd", "-W", "-p", "8080", "bash"]
