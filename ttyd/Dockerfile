FROM alpine/k8s:1.32.13

USER root

# Pin the kubecolor release we install below (https://github.com/kubecolor/kubecolor/releases).
ARG KUBECOLOR_VERSION=0.6.0

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

# --- kubecolor -----------------------------------------------------------
# Colorizes kubectl output; there's no Alpine package, so install the
# upstream release binary directly. kubecolor shells out to the real
# `kubectl` (already provided by the alpine/k8s base), so it works fine
# alongside the `kubectl`/`k` aliases set up below.
RUN set -eux; \
    case "$(uname -m)" in \
      x86_64)  kc_arch=amd64 ;; \
      aarch64) kc_arch=arm64 ;; \
      *) echo "unsupported arch for kubecolor: $(uname -m)" >&2; exit 1 ;; \
    esac; \
    curl -fsSL -o /tmp/kubecolor.tgz \
      "https://github.com/kubecolor/kubecolor/releases/download/v${KUBECOLOR_VERSION}/kubecolor_${KUBECOLOR_VERSION}_linux_${kc_arch}.tar.gz"; \
    tar -xzf /tmp/kubecolor.tgz -C /tmp kubecolor; \
    install -m 0755 /tmp/kubecolor /usr/local/bin/kubecolor; \
    rm -f /tmp/kubecolor.tgz /tmp/kubecolor; \
    kubecolor --kubecolor-version

# --- KubeSandbox shell personalization ------------------------------------
# Welcome banner + kubectl/k -> kubecolor aliases, sourced on every ttyd
# shell. ttyd runs `bash` directly (interactive, non-login), which reads
# ~/.bashrc — this appends a source line rather than replacing the file so
# any bashrc the base image ships with (e.g. kubectl completion) still runs.
COPY kubesandbox-shellrc.sh /etc/kubesandbox/shellrc.sh
RUN set -eux; \
    chmod 0644 /etc/kubesandbox/shellrc.sh; \
    printf '\n# KubeSandbox: welcome banner + kubecolor aliases\n. /etc/kubesandbox/shellrc.sh\n' >> /root/.bashrc

EXPOSE 8080

ENTRYPOINT ["ttyd", "-W", "-p", "8080", "bash"]
