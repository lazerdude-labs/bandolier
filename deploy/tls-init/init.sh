#!/usr/bin/env bash
set -euo pipefail

TLS=/tls
mkdir -p "$TLS"

if [ -f "$TLS/server.crt" ] && [ -f "$TLS/server.key" ]; then
  echo "TLS material exists; nothing to do."
  exit 0
fi

openssl req -x509 -nodes -newkey rsa:2048 -days 825 \
  -keyout "$TLS/server.key" -out "$TLS/server.crt" \
  -subj "/CN=bandolier.localhost" \
  -addext "subjectAltName=DNS:bandolier.localhost,DNS:localhost,IP:127.0.0.1"

chmod 600 "$TLS/server.key"

echo "TLS bundle written to $TLS"
