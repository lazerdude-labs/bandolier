#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# Bandolier — CI / developer smoke test.
#
# THIS IS NOT THE FIRST-TIME INSTALL PATH. For a normal install, see the
# README quick start: `cd deploy && docker compose up -d --build`.
#
# This script is destructive: the first thing it does is `docker compose
# down -v`, which wipes every Bandolier volume on the host (vault state,
# app database, terraform state, TLS certs). It then pre-fills the master
# password to a fixed value (`smoke-test-pw`) so the rest of the script can
# exercise the cluster create / deploy / destroy / redeploy / password-change
# cycle without an operator at the keyboard.
#
# Required tools on the host: docker (with the Compose v2 plugin), curl, jq.
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE/.."

# Preflight: fail fast with a single clear message if any required tool is
# missing. Without this, the script aborts mid-stream after the user has
# already destroyed their volumes via `compose down -v`, and they miss the
# how-to-access prompts at the end.
check_deps() {
  local missing=()
  command -v docker >/dev/null 2>&1 || missing+=("docker")
  docker compose version >/dev/null 2>&1 || missing+=("docker-compose-plugin (compose v2)")
  command -v curl >/dev/null 2>&1 || missing+=("curl")
  command -v jq >/dev/null 2>&1 || missing+=("jq")
  if [ ${#missing[@]} -gt 0 ]; then
    echo "ERROR: missing required tools on host:" >&2
    printf '  - %s\n' "${missing[@]}" >&2
    echo "" >&2
    echo "Install on Debian/Ubuntu: sudo apt-get install -y ${missing[*]/docker-compose-plugin (compose v2)/docker-compose-plugin}" >&2
    echo "Install on Rocky/RHEL:    sudo dnf install -y ${missing[*]/docker-compose-plugin (compose v2)/docker-compose-plugin}" >&2
    echo "" >&2
    echo "Note: this is the smoke-test script. For first-time install, see README." >&2
    exit 1
  fi
}
check_deps

echo "==> Bringing up stack..."
docker compose down -v >/dev/null 2>&1 || true
docker compose up -d --build
echo "==> Waiting for ui (api self-recovers once vault-agent finishes first-run setup)..."
for _ in $(seq 1 60); do
  if curl -ksSf https://127.0.0.1/api/health -o /dev/null; then break; fi
  sleep 2
done

echo "==> Setup master password..."
curl -ksSf -X POST https://127.0.0.1/api/auth/setup -H 'Content-Type: application/json' \
  -d '{"password":"smoke-test-pw"}'

echo "==> Login..."
curl -ksSf -X POST https://127.0.0.1/api/auth/login -H 'Content-Type: application/json' \
  -d '{"password":"smoke-test-pw"}' -c /tmp/bandolier-cookies.txt

echo "==> Create homelab cluster..."
CLUSTER_ID=$(curl -ksSf -X POST https://127.0.0.1/api/clusters -H 'Content-Type: application/json' \
  -b /tmp/bandolier-cookies.txt -d '{"name":"homelab","profile":"homelab"}' | jq -r .id)
echo "    cluster_id=$CLUSTER_ID"

echo ""
echo "==> Manual step: open https://127.0.0.1 in your browser, log in,"
echo "    initialize cluster $CLUSTER_ID with real Proxmox creds, click Deploy."
echo "    Watch the logs in /deployments/<id>."
echo ""
echo "==> Successful smoke completes when k3s nodes are ready:"
echo "    docker compose exec api ssh -i <key> rocky@192.0.2.21 'kubectl get nodes'"

# ----- Plan 2 Phase 1 additions: destroy → redeploy → password change -----

echo "==> Destroying cluster via API..."
DEP_ID=$(curl -kfsS -X POST -b /tmp/bandolier-cookies.txt \
  "https://127.0.0.1/api/clusters/$CLUSTER_ID/destroy" | jq -r .deployment_id)
echo "    destroy deployment: $DEP_ID"

for i in $(seq 1 60); do
  STATUS=$(curl -kfsS -b /tmp/bandolier-cookies.txt \
    "https://127.0.0.1/api/clusters/$CLUSTER_ID" | jq -r .status)
  echo "    [$i] status=$STATUS"
  [ "$STATUS" = "destroyed" ] && break
  [ "$STATUS" = "error" ]     && { echo "FAIL: cluster ended in error"; exit 1; }
  sleep 5
done

echo "==> Redeploying from destroyed state..."
REDEP_ID=$(curl -kfsS -X POST -b /tmp/bandolier-cookies.txt \
  "https://127.0.0.1/api/clusters/$CLUSTER_ID/deploy" | jq -r .deployment_id)
echo "    redeploy deployment: $REDEP_ID"

echo "==> Changing password..."
curl -kfsS -X POST -b /tmp/bandolier-cookies.txt -H 'Content-Type: application/json' \
  -d '{"current_password":"smoke-test-pw","new_password":"newSmokePassword12!"}' \
  "https://127.0.0.1/api/auth/change-password"
echo "==> Password changed (204 = success, no body)."
