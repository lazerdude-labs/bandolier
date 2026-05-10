# Changelog

All notable changes to Bandolier will be documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.2] — 2026-05-10

Operator-quality-of-life release: clear destroyed clusters off the home screen, fix the broken first-time onboarding path, and pin the build-time toolchain. No breaking changes; pull `ghcr.io/lazerdude-labs/bandolier/{api,ui,vault-agent,tls-init}:0.1.2` (or `:0.1` / `:latest`) to upgrade.

### Added

- **Forget cluster** action on the cluster detail page. After `Destroy` flips a cluster to `destroyed`, the row used to stick on `/clusters` forever — there was no UI path to remove it. The new action drops the cluster row, cascades through `deployments` / `apps_repos` / `apps_installs`, and best-effort purges the per-cluster Vault paths (`proxmox`, `network`, `ssh`, `k3s`, `kubeconfig`, `join_token`, `wildcard_cert`). Backed by a new `DELETE /api/clusters/{id}` endpoint, gated to `pending | initialized | destroyed | error` so a live cluster can't be silently orphaned (live states return 409). Closes #12.
- **Architecture diagrams** in the README — a runtime component flowchart and a deploy-flow sequence diagram, rendered natively by GitHub-flavored Mermaid. Newcomers can see how the containers wire together without reading the compose file.
- **`BANDOLIER_TF_STATE_ROOT` and `BANDOLIER_LOG_ROOT` env vars** for relocating per-cluster terraform state and deploy/install log files at runtime. Defaults match the prior hardcoded values (`/var/lib/bandolier/tf-state` and `/var/lib/bandolier/logs`). The startup log line now reports both roots alongside `db` and `vault`, so it's visible at boot which paths the api is using. New `## Configuration` section in the README documents all seven `BANDOLIER_*` env vars in one table. Closes #15.

### Fixed

- **First-time install no longer pre-fills a master password or aborts on missing `jq`.** Reported by an early user. The README's quick-start pointed at `deploy/scripts/smoke.sh`, which is actually a CI/dev script — it wipes volumes (`docker compose down -v`), pre-fills the master password to `smoke-test-pw` for assertion harnesses, and requires `jq` + `curl` on the host with no preflight. New users following the quick-start would either (a) hit a missing-`jq` error mid-stream and miss the helpful access prompts at the end, or (b) end up with an unguessable hardcoded password. The README quick-start now reads `cd deploy && docker compose up -d --build`, which lands the user on the existing UI setup screen where they pick their own password. `smoke.sh` got a header banner spelling out that it's a destructive CI script not for first-time install, plus a `check_deps` preflight that fails fast with a list of missing tools (and a `dnf`/`apt-get` install hint) instead of partway through.
- **Cluster deploys no longer break when the primary distro mirror returns a 4xx.** Reported by an early user against `dl.rockylinux.org`. The Rocky 9 catalog entry now lists three preference-ordered mirrors (`dl.rockylinux.org`, `download.rockylinux.org`, `mirror.rackspace.com/rockylinux`); on deploy, `BuildTfvars` HEAD-probes each (5s timeout, `User-Agent: Bandolier/1` so mirror-side filters behave predictably) and hands terraform the first 2xx URL. If all probes fail from the api container, the deploy falls through to the primary URL with a `slog.Warn` rather than blocking — Proxmox's egress isn't guaranteed to match the api's. Custom-URL paths still produce a single-element list and probe-then-pick the same way. The UI's `Distro` type field is renamed `url: string` → `urls: string[]` to match. Closes #11.

### Security

- **Verify upstream binary checksums during the api image build.** Closes #6. `api/Dockerfile` now downloads the published SHA256SUMS / `.sha256` file alongside each binary (Terraform, kubectl, Helm) and runs `sha256sum -c` before installing. A tampered binary served from `releases.hashicorp.com`, `dl.k8s.io`, or `get.helm.sh` (CDN compromise, BGP hijack, supply-chain event upstream) will fail the check and abort the build. Versions are now in named `ARG`s (`TF_VERSION`, `KUBECTL_VERSION`, `HELM_VERSION`) so the next bump is a one-line change.

## [0.1.1] — 2026-05-09

First release that ships pre-built container images. Operators can now pull `ghcr.io/lazerdude-labs/bandolier/{api,ui,vault-agent,tls-init}:0.1.1` instead of building from source. Includes a security fix for the deployment-log WebSocket endpoints.

### Added

- **Container images on GHCR.** Tag-triggered release workflow (`.github/workflows/release.yml`) builds and publishes four images to `ghcr.io/lazerdude-labs/bandolier/*` on every `v*.*.*` push: `api`, `ui`, `vault-agent`, `tls-init`. Each image is tagged with the full semver (`0.1.1`), the major.minor floating pin (`0.1`), and `latest` (only for stable semver tags, not pre-releases). Operators can now pin to specific versions instead of building from source.
- `SECURITY.md` documenting the vulnerability disclosure process (GitHub private advisories), supported versions, and what is in and out of scope for security reports.
- Issue templates for bug reports and feature requests (forms-style), plus a `config.yml` that disables blank issues and links to security advisories and discussions.
- Pull request template with type-of-change checkboxes, test plan, and a no-secrets-in-diff reminder.

### Fixed

- `errMessage` (UI helper for surfacing API errors in toast notifications) now performs a runtime body-shape check instead of relying on a TypeScript cast. If a future backend route returns an error body that isn't `{ error: string }` (array, raw string, nested object), the helper falls cleanly through to `Error.message` instead of rendering "[object Object]" or the generic `API <status>: <stringified body>` fallback.

### Changed

- Resolved pre-existing `lint:go` (errcheck + staticcheck) and `lint:ui` (`@typescript-eslint/no-explicit-any` + minor) debt that was masked behind `continue-on-error` flags. Both lint jobs are now enforced gates in CI.
- Migrated `nhooyr.io/websocket` (deprecated) to `github.com/coder/websocket` (the maintainer's new home; identical API).

### Security

- **WebSocket origin enforcement** in `/ws/deployments/{id}/logs` and `/ws/apps/installs/{id}/logs`. Previously `OriginPatterns: []string{"*"}` allowed any origin; replaced with default same-origin enforcement (the request host is always authorized; nothing else by default). Operators who run the UI on a different origin than the API (e.g. `npm run dev` against a remote API) can set `BANDOLIER_WS_ORIGIN_PATTERNS` (comma-separated host patterns, `path.Match` syntax) to allow additional origins. A bare `*` in the pattern list is dropped at parse time (with a one-shot warning log) so a misconfigured operator can't accidentally re-open the original CSRF window.

## [0.1.0] — 2026-05-07

Initial public release.

### Added

- Self-contained Docker Compose stack that deploys k3s clusters on Proxmox.
- React UI for cluster initialization, deployment, destroy, and Helm app installs.
- Go API driving Terraform (VM provisioning) and Ansible (k3s + Traefik configuration) over a Proxmox API token.
- Containerized HashiCorp Vault (KV v2 + PKI + AppRole) for credential storage.
- `vault-agent` long-running watcher that auto-unseals Vault on restart, eliminating the manual unseal step that breaks operators after host reboots.
- Master-password authentication; AppRole-scoped tokens for the API; never-on-host Vault deployment.
- Wildcard TLS issuance via internal PKI; live deployment log streaming over WebSocket.
- Profiles for homelab, blue-team, red-team, and grey-space cluster shapes (single-cluster v0.1; multi-cluster scenarios planned).
- App ecosystem with bundle installs (Helm charts grouped by use case).
- Audit log of operator actions with structured action constants.

### Notes

- Pre-1.0: minor version bumps may include breaking changes. We'll call them out explicitly here.
- The Vault threat model (single-host, single-operator, plaintext unseal keys on a docker volume) is documented in [`deploy/vault-init/THREAT_MODEL.md`](deploy/vault-init/THREAT_MODEL.md). If you need a different trust model, see that file for guidance.
