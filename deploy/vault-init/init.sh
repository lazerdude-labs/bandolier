#!/usr/bin/env bash
# vault-agent — first-run setup of bandolier's Vault, then a watch loop that
# re-unseals Vault whenever it comes up sealed (host reboot, container
# restart, etc.).
#
# Trust model and rationale: see ./THREAT_MODEL.md.

set -euo pipefail
umask 077

STATE=/state
KEYS_FILE=$STATE/init.json
APPROLE_FILE=$STATE/approle.json
INTERVAL=${VAULT_AGENT_INTERVAL:-10}

# mTLS material. Vault's listener runs tls_require_and_verify_client_cert, so
# every request must present the agent client cert that chains to the CA. All
# Vault API calls below go through vcurl(), never bare curl. VAULT_ADDR is the
# https endpoint (set in docker-compose.yml).
CACERT=${VAULT_CACERT:-/tls/ca.crt}
CLIENTCERT=${VAULT_CLIENT_CERT:-/tls/agent.crt}
CLIENTKEY=${VAULT_CLIENT_KEY:-/tls/agent.key}

vcurl() { curl --cacert "$CACERT" --cert "$CLIENTCERT" --key "$CLIENTKEY" "$@"; }

mkdir -p "$STATE"

log() {
  printf '%s vault-agent: %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"
}

# ---------- helpers ----------------------------------------------------------

wait_vault_responsive() {
  for _ in $(seq 1 60); do
    if vcurl -fsS "$VAULT_ADDR/v1/sys/health?standbyok=true&sealedcode=200&uninitcode=200" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  log "vault not reachable after 60s"
  return 1
}

vault_initialized() {
  vcurl -fsS "$VAULT_ADDR/v1/sys/init" | jq -e .initialized >/dev/null
}

vault_sealed() {
  vcurl -fsS "$VAULT_ADDR/v1/sys/seal-status" | jq -e .sealed >/dev/null
}

apply_unseal_keys() {
  # POST three keys (matches the secret_threshold the file was initialized
  # with). The jq slice avoids SIGPIPE-on-`head` interacting with pipefail.
  local keys k
  keys=$(jq -r '.keys[0:3][]' "$KEYS_FILE" 2>/dev/null) || keys=""
  if [ -z "$keys" ]; then
    # Empty keys would silently turn the for-loop into a no-op and the caller
    # would log "vault unsealed" while vault stayed sealed. Surface it instead.
    log "ERROR: no unseal keys parsed from $KEYS_FILE (missing or corrupt?)"
    return 1
  fi
  for k in $keys; do
    vcurl -fsS -X POST "$VAULT_ADDR/v1/sys/unseal" -d "{\"key\":\"$k\"}" >/dev/null
  done
}

# ---------- phase A: first-run setup -----------------------------------------
# Each step is independently gated so re-running is a no-op once converged.
# AppRole credentials, however, are written exactly once: regenerating
# secret_id on every container start would invalidate the api's persisted
# creds for no reason.

setup_if_needed() {
  if ! vault_initialized; then
    log "initializing vault"
    vcurl -fsS -X POST "$VAULT_ADDR/v1/sys/init" \
      -d '{"secret_shares":5,"secret_threshold":3}' > "$KEYS_FILE"
    chmod 600 "$KEYS_FILE"
  fi

  if vault_sealed; then
    log "unsealing vault (first-run)"
    apply_unseal_keys
  fi

  local root
  root=$(jq -r .root_token "$KEYS_FILE")
  export VAULT_TOKEN=$root

  if ! vcurl -fsS -H "X-Vault-Token: $VAULT_TOKEN" "$VAULT_ADDR/v1/sys/mounts" \
        | jq -e '.["bandolier/"]' >/dev/null; then
    log "enabling bandolier KV v2 mount"
    vcurl -fsS -X POST -H "X-Vault-Token: $VAULT_TOKEN" \
      "$VAULT_ADDR/v1/sys/mounts/bandolier" \
      -d '{"type":"kv","options":{"version":"2"}}' >/dev/null
  fi

  if ! vcurl -fsS -H "X-Vault-Token: $VAULT_TOKEN" "$VAULT_ADDR/v1/sys/mounts" \
        | jq -e '.["pki/"]' >/dev/null; then
    log "enabling pki mount"
    vcurl -fsS -X POST -H "X-Vault-Token: $VAULT_TOKEN" \
      "$VAULT_ADDR/v1/sys/mounts/pki" \
      -d '{"type":"pki","config":{"max_lease_ttl":"87600h"}}' >/dev/null
  fi

  # PKI bootstrap: root CA + `traefik` role. Required because the api's
  # IssueWildcardCert (api/internal/clusters/cert.go) calls
  # pki/issue/traefik on every cluster deploy at the TLS-wildcard step;
  # without these, deploys fail with "unknown role: traefik" after the
  # VMs are already provisioned.
  #
  # CA presence is detected by reading pki/ca/pem and looking for a PEM
  # header. A fresh mount returns 200 with an empty body; an initialized
  # mount returns the certificate. We deliberately do NOT use `-f` here:
  # under `set -o pipefail`, a transient Vault 5xx on this read would
  # abort setup_if_needed before reaching the generate call, leaving the
  # mount permanently CA-less. With `-sS`, a non-2xx returns whatever
  # body Vault produced (typically a JSON error), which won't contain
  # the PEM header — so the missing-CA branch fires and the generate
  # call below either succeeds or surfaces the underlying error itself.
  # Generation is one-shot: Vault rejects a second generate/internal
  # once a root issuer exists, which is why the check guards the call.
  if ! vcurl -sS -H "X-Vault-Token: $VAULT_TOKEN" \
        "$VAULT_ADDR/v1/pki/ca/pem" 2>/dev/null \
        | grep -q "BEGIN CERTIFICATE"; then
    log "generating pki root CA"
    vcurl -fsS -X POST -H "X-Vault-Token: $VAULT_TOKEN" \
      "$VAULT_ADDR/v1/pki/root/generate/internal" \
      -d '{"common_name":"Bandolier Homelab Root CA","ttl":"87600h","key_type":"rsa","key_bits":4096}' >/dev/null
  fi

  # The traefik role gates pki/issue/traefik. PUT is idempotent — re-runs
  # rewrite the same definition with no observable side effect. max_ttl
  # 8784h covers the api's 8760h request with 24h slack.
  #
  # Threat-model note: `allow_any_name=true` permits issuance for any
  # common_name. This is acceptable for v1 because (a) the CA is a
  # self-signed homelab root with no public trust, and (b) the only
  # caller is IssueWildcardCert with operator-set FQDNs. The realistic
  # risk is lateral movement: a compromised AppRole secret_id (granting
  # `pki/issue/traefik` per policy.hcl) could mint a cert for any
  # internal name (e.g. *.gitlab.rplab.lan) and pivot. v2 will scope
  # this with `allowed_domains` per-cluster at deploy time. See
  # ./THREAT_MODEL.md.
  vcurl -fsS -X PUT -H "X-Vault-Token: $VAULT_TOKEN" \
    "$VAULT_ADDR/v1/pki/roles/traefik" \
    -d '{"allow_any_name":true,"max_ttl":"8784h"}' >/dev/null

  vcurl -fsS -X PUT -H "X-Vault-Token: $VAULT_TOKEN" \
    "$VAULT_ADDR/v1/sys/policies/acl/bandolier-api" \
    --data-binary @<(jq -Rs '{policy:.}' /policy.hcl) >/dev/null

  if ! vcurl -fsS -H "X-Vault-Token: $VAULT_TOKEN" "$VAULT_ADDR/v1/sys/auth" \
        | jq -e '.["approle/"]' >/dev/null; then
    log "enabling approle auth method"
    vcurl -fsS -X POST -H "X-Vault-Token: $VAULT_TOKEN" \
      "$VAULT_ADDR/v1/sys/auth/approle" \
      -d '{"type":"approle"}' >/dev/null
  fi

  # Idempotent re-apply on every container start. secret_id_ttl: 0 means
  # existing secret-ids stay valid, so this never invalidates the api's
  # persisted creds. Footgun: changes to token_ttl/token_max_ttl take effect
  # only for new tokens; existing tokens keep the old limits until expiry.
  vcurl -fsS -X POST -H "X-Vault-Token: $VAULT_TOKEN" \
    "$VAULT_ADDR/v1/auth/approle/role/bandolier-api" \
    -d '{"token_policies":"bandolier-api","token_ttl":"1h","token_max_ttl":"4h","secret_id_ttl":"0"}' >/dev/null

  if [ ! -f "$APPROLE_FILE" ]; then
    log "generating approle role-id + secret-id"
    local role_id secret_id
    role_id=$(vcurl -fsS -H "X-Vault-Token: $VAULT_TOKEN" \
      "$VAULT_ADDR/v1/auth/approle/role/bandolier-api/role-id" \
      | jq -r .data.role_id)
    secret_id=$(vcurl -fsS -X POST -H "X-Vault-Token: $VAULT_TOKEN" \
      "$VAULT_ADDR/v1/auth/approle/role/bandolier-api/secret-id" \
      | jq -r .data.secret_id)
    jq -n --arg r "$role_id" --arg s "$secret_id" '{role_id:$r,secret_id:$s}' > "$APPROLE_FILE"
    chmod 600 "$APPROLE_FILE"
  fi

  unset VAULT_TOKEN
}

# ---------- phase B: watch loop ----------------------------------------------
# Polls /v1/sys/seal-status and re-applies unseal keys whenever vault comes up
# sealed. Steady-state idle ticks log nothing to keep `docker logs` quiet.
# Errors during apply are logged and retried on the next tick.

watch_loop() {
  log "watcher started, interval=${INTERVAL}s"
  local seal_status
  while :; do
    if seal_status=$(vcurl -fsS "$VAULT_ADDR/v1/sys/seal-status" 2>/dev/null); then
      if echo "$seal_status" | jq -e .sealed >/dev/null 2>&1; then
        log "vault is sealed; applying unseal keys"
        if apply_unseal_keys; then
          log "vault unsealed"
        else
          log "unseal failed (will retry in ${INTERVAL}s)"
        fi
      fi
    else
      log "seal-status check failed (vault unreachable?); will retry in ${INTERVAL}s"
    fi
    sleep "$INTERVAL"
  done
}

# ---------- main -------------------------------------------------------------

main() {
  wait_vault_responsive
  setup_if_needed
  watch_loop
}

main "$@"
