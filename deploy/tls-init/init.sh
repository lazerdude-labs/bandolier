#!/usr/bin/env bash
# tls-init — one-shot generation of all TLS material for the stack, written to
# the shared `tls` volume. Idempotent: every artifact is gated on existence, so
# re-runs (container restart, `compose up` after first boot) are no-ops and the
# already-issued certs/keys are never rotated out from under a running Vault.
#
# Two trust domains live here:
#   1. UI server cert (server.crt/.key) — browser-facing nginx TLS. Self-signed,
#      unchanged from v0.1.x (the operator's browser trusts it manually).
#   2. Internal mTLS PKI (ca.crt + vault/api/agent leaf certs) — secures the
#      api<->vault and vault-agent<->vault channels. Vault's listener runs
#      tls_require_and_verify_client_cert, so api and vault-agent each present a
#      client cert that chains to ca.crt. This satisfies the constitution's
#      "mTLS between api and vault" principle.
#
# Key readability: leaf keys are mode 0644. The tls volume is internal-only
# (never host-exposed) and mounted read-only into the trusted stack containers,
# which run under heterogeneous uids (vault=100 via su-exec, api, nginx) that
# can't share a single restrictive owner/group. Per the single-trusted-operator
# threat model (deploy/vault-init/THREAT_MODEL.md), volume/exec access is the
# real boundary, so file mode adds nothing here. The CA key (ca.key) is the one
# exception: 0600, and it never leaves this container — it only signs leaves.
set -euo pipefail
umask 077

TLS=/tls
CA_DAYS=3650
LEAF_DAYS=825

mkdir -p "$TLS"

# ---------------------------------------------------------------------------
# helpers
# ---------------------------------------------------------------------------

# issue_leaf <name> <subject-CN> <x509-extension-block>
# Generates <name>.key + <name>.crt signed by the CA using the supplied x509
# extension block. Idempotent on <name>.crt existence.
issue_leaf() {
  local name="$1" cn="$2" ext="$3"
  if [ -f "$TLS/$name.crt" ] && [ -f "$TLS/$name.key" ]; then
    return 0
  fi
  local extfile
  extfile="$(mktemp)"
  printf '%s\n' "$ext" > "$extfile"
  openssl genrsa -out "$TLS/$name.key" 2048
  openssl req -new -key "$TLS/$name.key" -subj "/CN=$cn" -out "$TLS/$name.csr"
  openssl x509 -req -in "$TLS/$name.csr" \
    -CA "$TLS/ca.crt" -CAkey "$TLS/ca.key" -CAcreateserial \
    -days "$LEAF_DAYS" -sha256 -extfile "$extfile" \
    -out "$TLS/$name.crt"
  rm -f "$TLS/$name.csr" "$extfile"
  # Both readable by any stack uid that mounts the internal tls volume (see
  # the key-readability note in the header).
  chmod 0644 "$TLS/$name.crt"
  chmod 0644 "$TLS/$name.key"
}

# ---------------------------------------------------------------------------
# 1. UI server cert (browser-facing nginx) — unchanged behavior
# ---------------------------------------------------------------------------
if [ ! -f "$TLS/server.crt" ] || [ ! -f "$TLS/server.key" ]; then
  openssl req -x509 -nodes -newkey rsa:2048 -days "$LEAF_DAYS" \
    -keyout "$TLS/server.key" -out "$TLS/server.crt" \
    -subj "/CN=bandolier.localhost" \
    -addext "subjectAltName=DNS:bandolier.localhost,DNS:localhost,IP:127.0.0.1"
  chmod 0644 "$TLS/server.key"
  chmod 0644 "$TLS/server.crt"
fi

# ---------------------------------------------------------------------------
# 2. Internal mTLS CA
# ---------------------------------------------------------------------------
if [ ! -f "$TLS/ca.crt" ] || [ ! -f "$TLS/ca.key" ]; then
  openssl req -x509 -nodes -newkey rsa:4096 -days "$CA_DAYS" \
    -keyout "$TLS/ca.key" -out "$TLS/ca.crt" \
    -subj "/CN=Bandolier Internal mTLS CA"
  chmod 0600 "$TLS/ca.key"   # never read by any other container
  chmod 0644 "$TLS/ca.crt"   # trust anchor for vault, api, agent
fi

# ---------------------------------------------------------------------------
# 3. Leaf certs
# ---------------------------------------------------------------------------
# Vault server cert — serverAuth + SANs the clients dial (service name `vault`,
# plus loopback for the in-container healthcheck).
issue_leaf vault "vault" \
"basicConstraints=CA:FALSE
keyUsage=digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:vault,DNS:localhost,IP:127.0.0.1"

# api client cert — clientAuth only.
issue_leaf api "bandolier-api" \
"basicConstraints=CA:FALSE
keyUsage=digitalSignature,keyEncipherment
extendedKeyUsage=clientAuth"

# vault-agent client cert — clientAuth only.
issue_leaf agent "vault-agent" \
"basicConstraints=CA:FALSE
keyUsage=digitalSignature,keyEncipherment
extendedKeyUsage=clientAuth"

echo "tls-init: TLS material ready in $TLS"
ls -l "$TLS"
