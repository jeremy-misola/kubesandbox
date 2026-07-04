#!/usr/bin/env bash
# KubeSandbox interactive shell setup.
#
# Sourced from /root/.bashrc on every ttyd shell. Each KubeSandboxSession pod
# is a single-use, ephemeral sandbox handed to exactly one user for one
# session, so this fires fresh every time someone opens a terminal — there is
# no "returning user" case to special-case.

cat <<'BANNER'

 _  __     _          ____                  _ _
| |/ /   _| |__   ___/ ___|  __ _ _ __   __| | |__   _____  __
| ' / | | | '_ \ / _ \___ \ / _` | '_ \ / _` | '_ \ / _ \ \/ /
| . \ |_| | |_) |  __/___) | (_| | | | | (_| | |_) | (_) >  <
|_|\_\__,_|_.__/ \___|____/ \__,_|_| |_|\__,_|_.__/ \___/_/\_\

  Ephemeral Kubernetes sandbox — this pod disappears when your session ends.
  kubectl (and 'k') are aliased to kubecolor for colorized output.

BANNER

alias kubectl='kubecolor'
alias k='kubecolor'
