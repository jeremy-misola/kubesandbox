#!/bin/sh
# Generates /config.js at container start from environment variables, so a single
# built image can be promoted across environments (the Helm chart's `env:` block
# sets these). See docs/06-frontend-architecture.md §7.
set -eu

CONFIG_PATH="/usr/share/nginx/html/config.js"

cat > "$CONFIG_PATH" <<EOF
window.__ENV = {
  VITE_API_BASE: "${VITE_API_BASE:-/api}",
  VITE_PUBLIC_BASE_URL: "${VITE_PUBLIC_BASE_URL:-}",
  VITE_OIDC_ISSUER: "${VITE_OIDC_ISSUER:-}",
  VITE_OIDC_CLIENT_ID: "${VITE_OIDC_CLIENT_ID:-kubesandbox-frontend}",
  VITE_OIDC_REDIRECT_URI: "${VITE_OIDC_REDIRECT_URI:-}"
};
EOF

echo "runtime config written to $CONFIG_PATH"
