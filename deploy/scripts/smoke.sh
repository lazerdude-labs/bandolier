#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE/.."

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
