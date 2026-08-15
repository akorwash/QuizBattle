#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

[[ "${EUID}" -eq 0 ]] || {
  echo "The Certbot deploy hook must run as root." >&2
  exit 77
}
command -v nginx >/dev/null || {
  echo "Nginx is unavailable; refusing to reload after certificate renewal." >&2
  exit 69
}
command -v systemctl >/dev/null || {
  echo "systemctl is unavailable; refusing to reload Nginx." >&2
  exit 69
}

nginx -t
systemctl reload nginx
