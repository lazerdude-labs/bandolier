# Threat Model — Bandolier Vault State

This document is the deliberate trust-model decision behind the way Bandolier stores its Vault unseal keys and AppRole credentials. It is meant to be read once, by anyone wondering "why aren't these encrypted?", and to settle that question for them.

## Deployment topology

Bandolier ships as a `docker compose` stack designed to run on a single homelab host. The intended operator is a single trusted admin who already has root on the host and unrestricted access to the local docker socket. Multi-tenancy and remote-administrator scenarios are not supported.

## What lives in the keys volume

The `vault-init-state` named volume contains two files written by the `vault-agent` service:

| File           | Contents                                           |
|----------------|----------------------------------------------------|
| `init.json`    | 5 unseal keys + Shamir share metadata + the initial root token |
| `approle.json` | role-id and secret-id for the `bandolier-api` AppRole |

Both files are mode `600`, owned by root, and only mounted into two containers: `vault-agent` (read/write) and `api` (read-only at `/vault-init-state:ro`).

## Trust boundary

**Trusted (assumed compromised → game over):**
- The host running Bandolier
- Anyone with root or sudo on that host
- Anyone with access to the docker daemon socket (`/var/run/docker.sock`)
- Anyone with write access to the docker volume backend on disk
- Anyone who can modify the local docker images or the registry they're pulled from

**Defended:**
- The network. Vault listens only on the `bandolier` docker bridge network; no host port mapping. The keys volume is not exposed outside the host. Communication is internal Docker-bridge traffic.
- The api's read access is read-only — even an api compromise cannot rewrite the keys.

## What an attacker who exfiltrates `init.json` can do

Full Vault access. Specifically:
- Unseal Vault (already trivial via the docker socket).
- Read every secret stored in the `bandolier/` KV mount, including provider credentials for every cluster the operator has configured.
- Sign certificates from the PKI mount.
- Mint a new root token via `vault operator generate-root` using the unseal keys.

This is functionally equivalent to "the host is fully compromised." If the threat model includes attackers who can read the keys volume but cannot otherwise touch the host, that scenario is not realistic for Bandolier's deployment topology — anyone who can read a docker volume already has either host root or docker-socket access.

## What `vault-agent` defends against

One thing only: **operational failure of recovery after a legitimate vault restart.** Vault uses Shamir-seal file storage, so it always comes up sealed after a host reboot, container restart, or image upgrade. Without `vault-agent`, the operator has to manually re-run an init script to re-apply the unseal keys; with it, recovery is automatic within ~10 seconds.

This is a pure functional-availability fix, not a security boundary.

## What this watcher does NOT defend against

- An attacker with the docker socket (they can `docker exec` into vault directly).
- An attacker with host root (they can read every file on disk).
- An attacker who modifies the watcher binary or the `hashicorp/vault:1.16` image (they can substitute their own keys).
- A malicious image registry serving a backdoored `hashicorp/vault:1.16` (image digest pinning would help, not implemented).
- Encrypted-at-rest threats (the keys are plaintext on disk).

## Why we don't encrypt the keys at rest

Encryption at rest only buys protection against an attacker who can read the volume but cannot read process memory or modify the watcher. On a single-host docker stack, no realistic attacker has that capability — the same docker socket that lets you read the volume lets you `docker exec` into the watcher and either dump its memory or replace its image.

The operational cost of encryption is concrete:

- The operator must supply a passphrase at every host boot before the API recovers.
- A host that reboots while the operator is away leaves the API broken until they SSH in.
- This is exactly the "I want secure but don't want issues" failure mode the watcher is meant to eliminate.

For deployments with stricter requirements, the right answer is not encrypted-at-rest unseal keys — it's running Vault on a host whose access is gated by other means: dedicated VLAN, jump host, MFA-gated SSH, hardware security module for auto-unseal, etc. Those are out of scope for Bandolier v1.

## Operational consequences for operators

- **Treat the host running Bandolier as a secrets host.** Don't run untrusted code on it. Don't grant docker socket access to anyone you wouldn't grant root to.
- **Back up `vault-init-state` and `vault-data` together** if you back them up at all. Restoring one without the other leaves Vault permanently sealed.
- **Don't share `init.json` outside the host.** Don't paste it into Slack. Don't email it to yourself. Don't put it in a normal git repo.
- **`docker logs deploy-vault-agent-1` is operator-readable.** The current script never logs key material, but anyone modifying `init.sh` should keep that contract — never `log` a variable holding a token, role-id, secret-id, or unseal key. Treat the log stream as if it were attached to a ticket someone will paste later.
- **If the host is compromised, rotate.** Issue a new Vault deployment from scratch. The leaked keys grant indefinite access to every secret Vault holds.

## PKI role: `traefik` is `allow_any_name=true`

`init.sh` (v0.1.6+) bootstraps a `traefik` PKI role with `allow_any_name=true` so the api's `IssueWildcardCert` can mint a wildcard cert for whatever cluster FQDN the operator picked in the wizard. Vault enforces the role's privilege boundary, but within that boundary the role accepts **any** common_name.

**Realistic attack:** an attacker who exfiltrates the api's AppRole `secret_id` (which is mounted at `/vault-init-state/approle.json` inside the api container, and which the `bandolier-api` policy grants `pki/issue/traefik`) can issue a self-signed cert for any internal name — `*.gitlab.rplab.lan`, `*.victim.local`, anything — and use it for in-LAN MITM against operators who installed the Bandolier root CA in their trust store.

**Why we accept this for v1:**
- The CA is a self-signed homelab root with **no public trust**. Certs it issues carry zero weight outside hosts where the operator deliberately added it.
- The AppRole `secret_id` is on the same volume as `init.json`; an attacker with file access to that volume already has full Vault access (the broader threat model above already grants them everything).
- The only marginal exposure is operators who add the Bandolier root to their workstation's trust store and then pivot through LAN services. The fix narrows attacker capability from "full Vault" to "full Vault minus the ability to mint convincing internal certs," which is a small delta.

**v2 mitigation:** scope the `traefik` role per-cluster at deploy time with `allowed_domains=[<cluster_fqdn>]` and `allow_subdomains=true`. The api would either rotate the role on each deploy or use a dynamically-templated role. Out of scope for v0.1.x.

## Future work

Items that would tighten the boundary, listed for completeness:

- Pin `hashicorp/vault:1.16` by digest (defends against backdoored re-publication).
- Add a hostport-bound recovery shell that requires a passphrase, separate from the boot-time unseal path (lets paranoid operators opt into encrypted-at-rest without breaking automatic recovery).
- Auto-unseal via cloud KMS / Vault transit / a dedicated HSM (changes the architecture; not appropriate for self-contained homelab deployment).
- Per-cluster scoped `traefik` PKI role (see "PKI role" above).

These are not blocked by this design and can be added later without rework.
