# Changelog

All notable changes to Bandolier will be documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.8] — 2026-05-11

Two operator-experience fixes uncovered by a real v0.1.7 deploy: the wizard could silently skip its SSH step when the operator pressed Enter inside a network input, and the VLAN field rejected the documented "0 for untagged" value. Pull `ghcr.io/lazerdude-labs/bandolier/{api,ui,vault-agent,tls-init}:0.1.8` (or `:0.1` / `:latest`) to upgrade.

### Fixed

- **Initialize wizard no longer submits prematurely on Enter.** The wizard wraps all three steps in a single `<form>` and React Hook Form's `handleSubmit` happily validates the full schema on any submit event — including the implicit browser submit when Enter is pressed inside a single-line input. With sensible homelab defaults across every step (and SSH optional/blank), the form passed validation and submitted from step 1 (Network), skipping step 2 (SSH) entirely. Operators ended up at the cluster page without ever seeing the SSH configuration and got auto-generated keys they never knew were created. Fix gates submit at the DOM level: `preventDefault` unconditionally, route through the step-advance helper on steps 1–2, only actually submit on step 3 (SSH). Belt-and-suspenders `onKeyDown` interceptor intercepts Enter on non-textarea inputs and calls the step-advance helper directly so the user gets snappy advance behavior without the round-trip through React Hook Form's full-schema validator.
- **VLAN field is now optional.** Through v0.1.7 the wizard's Zod schema required `vlan: 1-4094`, even though `docs/proxmox-setup.md` documented "set to 0 for untagged" as the correct path for flat-network setups. The mismatch silently rejected the documented setup. Schema now accepts `0–4094`, the form labels the field "VLAN (optional)" with an inline hint, and the live summary shows "untagged" only when the operator deliberately picks 0 (an explicit `=== 0` check, so an undefined value during edit-mode load doesn't briefly render as "untagged"). The terraform vm module's `network_device.vlan_id` becomes `null` when the var is 0 — the bpg/proxmox provider omits the tag entirely, which is what Proxmox itself wants for a non-VLAN-aware bridge (or the default untagged VLAN on a VLAN-aware bridge). Root `network_vlan` variable gains a `validation` block asserting the `0–4094` range. The wizard surfaces an amber warning when VLAN 0 is selected, noting that the VM will join the bridge's native VLAN — operators should verify they know what that is on a VLAN-aware bridge before proceeding.

### Security

- **`POST /api/clusters/{id}/initialize` now bounds-checks `network.vlan` server-side (0–4094).** Caught in security review. The wizard's Zod schema enforces the range, but the api endpoint is reachable directly (authenticated user, no UI required) — without a server-side check, an out-of-bounds value would silently land in Vault before terraform's plan-time validation caught it, leaving a poisoned config behind. Returns 400 with a clear message when the value is out of range.
- **Wizard `onKeyDown` Enter-intercept now also exempts `<select>` elements.** Caught in code review. Without the exemption, pressing Enter on a focused dropdown (e.g. the distro selector) was intercepted and routed to step-advance, stealing the keystroke from its native "confirm selection" meaning.

## [0.1.7] — 2026-05-11

The Forget button is now safe to use against live clusters: it cascades through Destroy automatically, so the operator no longer has to remember the two-step Destroy-then-Forget dance. Pull `ghcr.io/lazerdude-labs/bandolier/{api,ui,vault-agent,tls-init}:0.1.7` (or `:0.1` / `:latest`) to upgrade.

### Added

- **`DELETE /api/clusters/{id}?cascade=destroy` extends Forget to live clusters.** Through v0.1.6, Forget worked only on `pending | initialized | destroyed | error` clusters; clicking it on a `ready` or `degraded` cluster returned 409 with "destroy the cluster before deleting its configuration". Operators frequently skipped the Destroy step and were left with orphaned VMs in Proxmox. The new cascade mode (a) sets a `pending_forget` latch on the cluster row, (b) kicks off `terraform destroy` via the existing executor, (c) returns 202 with the destroy `deployment_id`. The executor's `runDestroy` success path reads the latch and completes the Forget — Vault paths purged, SQLite row dropped — after terraform finishes. On destroy failure (terraform exit non-zero, vault unreachable, etc.), the latch is cleared and the cluster stays in `error` so the operator can investigate before throwing away config. Transient states (`deploying | upgrading | destroying`) still 409 — operator must wait or cancel the in-flight deployment first. Schema migration 007 adds `clusters.pending_forget INTEGER NOT NULL DEFAULT 0`.
- **UI Forget button extends to `ready | degraded` clusters with state-aware modal copy.** When clicked on a live cluster, the modal title changes to "Destroy and forget cluster?" and the body spells out the two-step plan (terraform destroy → row drop) with an amber warning that VMs will be torn down. On confirm, the UI POSTs the cascade-flagged DELETE and navigates to the destroy deploy log page so the operator sees terraform output streaming live. The non-cascade path is unchanged. Distinct modal copy by design so an operator can't accidentally tear down VMs thinking they're just clearing a stale row.
- **Audit log gains `cluster_delete` started-phase entries** when cascade is used (action=`cluster_delete`, outcome=`started`, details include `phase: "destroy"`, `deployment_id`, `cascade: "destroy"`). The terminal `success` / `failure` row is still written by the executor after destroy completes (or the operator-initiated direct-forget path, unchanged).

### Changed

- **Forget-orchestration code lifted out of `*Handler` into a package fn.** `clusters.PurgeVaultSecrets(ctx, vault, id)` and `clusters.ForgetCluster(ctx, store, vault, id, name, status, actorID)` are now package-level, so both the operator-initiated DELETE path and the executor's cascade hook can invoke them without the handler dependency. Pure refactor — no behavior change in the non-cascade path.
- **`DELETE /api/clusters/{id}` now validates the cluster id against the generator shape (32 lowercase hex chars).** Defense in depth: `ForgetCluster` constructs Vault paths from this string, so a path-traversal-style id (`../auth/master`) hitting Vault would be catastrophic. chi's default `{id}` matching already rejects `/`, but the `isValidClusterID` guard makes the invariant explicit and survives any future routing refactor.

### Security

- **`pending_forget` latch is now cleared during `RecoverOrphanedDeployments`.** Caught in security review. Without this, an api restart mid-cascade-destroy left the latch set on the cluster row (which had transitioned to `error` via orphan recovery); the next operator-initiated destroy on that errored cluster would silently auto-forget on success — surprising the operator who just wanted to retry. Recovery now clears the latch alongside the deployment-failed + cluster-error transitions.
- **Cascade-destroy failure now emits a `cluster_delete:failure` audit row.** Previously a started `cluster_delete` row from the DELETE handler had no terminal counterpart in the audit log when the cascaded destroy itself failed (the terminal row went under `cluster_destroy`). Closes a forensic gap for operators filtering audit by action.

## [0.1.6] — 2026-05-10

Two latent bugs uncovered by the first end-to-end deploy on a clean Vault, post-v0.1.5: every cluster init died at the TLS-wildcard step because Vault PKI was never bootstrapped, and the deploy log stream returned 403 for any operator hitting the UI from a non-loopback host. Pull `ghcr.io/lazerdude-labs/bandolier/{api,ui,vault-agent,tls-init}:0.1.6` (or `:0.1` / `:latest`) to upgrade.

### Fixed

- **Vault PKI is now bootstrapped at first run.** The `vault-agent` container's `init.sh` enables the `pki/` mount but, through v0.1.5, never generated a root CA or created the `traefik` role that the api's `IssueWildcardCert` (api/internal/clusters/cert.go) calls during the TLS-wildcard step of every cluster deploy. Result: every cleanly-initialized Bandolier install died with `Error making API request. URL: PUT http://vault:8200/v1/pki/issue/traefik Code: 400. Errors: * unknown role: traefik` after the VMs were already provisioned. The bug was latent through v0.1.0–v0.1.5; v0.1.5 fixed the upstream Rocky CDN HEAD-block, which let the deploy reach the TLS step and surfaced this. `init.sh` now (1) generates a 4096-bit RSA root CA (`Bandolier Homelab Root CA`, 10y validity) when `pki/ca/pem` reports no certificate, and (2) PUTs the `traefik` role with `allow_any_name=true`, `max_ttl=8784h` (covers the api's 8760h request with 24h slack). Both ops are idempotent — the CA generate is gated by the existence check, and the role PUT is naturally rewrite-safe. Existing installs that already manually configured PKI via the v0.1.5 unblock commands are unaffected; the upgrade is a no-op for them.
- **Deploy log stream `/ws/deployments/<id>/logs` no longer 403s when the UI is accessed from a non-localhost origin.** The ui container's `nginx.conf` was setting `proxy_set_header Host $host` on the `/api/` block but missing it on the `/ws/` block, so the api saw `Host: api:8080` while the browser's `Origin` was the operator's hostname — `coder/websocket`'s same-origin check rejected the upgrade with 403. Affects every operator running Bandolier on a headless VM and accessing the UI from a separate workstation. Fix is a one-line addition to `nginx.conf` matching what the `/api/` block already does. The `BANDOLIER_WS_ORIGIN_PATTERNS` env var is still the right escape hatch for multi-host setups (FQDN + IP, multiple LAN names, etc.).

## [0.1.5] — 2026-05-10

Two-issue fix release driven by a real v0.1.4 deploy: the wizard now has a supported path around the upstream Rocky CDN HEAD-block, and the "Test reachability" button surfaces the exact `pveum acl modify` command when a token is missing the required role on a storage. Pull `ghcr.io/lazerdude-labs/bandolier/{api,ui,vault-agent,tls-init}:0.1.5` (or `:0.1` / `:latest`) to upgrade.

### Added

- **"Image already uploaded to Proxmox (skip download)" toggle** on the initialize wizard's Proxmox step. When enabled, terraform references an existing file at `<image_storage>:iso/<filename>` via a `data "proxmox_virtual_environment_file"` source instead of issuing the `proxmox_virtual_environment_download_file` call that the bpg provider routes through Proxmox's HEAD-blocked fetcher. Workaround for upstream CDN HEAD-blocks (Rocky's `dl.rockylinux.org` filters Proxmox's User-Agent and returns a hard 403 on HEAD), now first-class instead of a TROUBLESHOOTING.md side door. Wizard hint includes the expected catalog filename for the selected distro (e.g. `Rocky-9-GenericCloud.latest.x86_64.img`). Threads through `proxmox.image_pre_uploaded` in Vault, the `proxmox_image_pre_uploaded` tfvar, and a `count`-gated resource/data toggle in `terraform/cloud_image.tf`. Closes #23.

### Changed

- **"Test reachability" failure detail now suggests the precise `pveum acl modify` command** when Proxmox returns a 403 with a `Permission check failed (<path>, <privs>)` body — the most common token-vs-storage mismatch operators hit on first install. For `/storage/<name>` paths the detail now reads "token missing PVEDatastoreAdmin on /storage/<name>. Fix on Proxmox: `pveum acl modify /storage/<name> --tokens '<your-token-id>' --roles PVEDatastoreAdmin --propagate 1`". For `/nodes/<name>` paths it suggests `PVEAuditor` for the preflight and notes that `PVEVMAdmin` is also needed for deploy. The hint only fires on 403; 401 (bad/expired/revoked token) keeps the original "token unauthorized" detail so it can't misdirect to an ACL grant. Captured path/privilege values are allowlisted (`/[a-zA-Z0-9/_.-]{1,128}` and `[A-Za-z][A-Za-z0-9.|_-]{0,127}`) before being interpolated into the suggested command, so a malicious or misconfigured Proxmox can't reflect crafted shell snippets into the operator-facing detail. Previously the detail field surfaced the raw HTTP body and operators had to look up which privilege their role needed to grant.

### Security

- **Audit log records `image_pre_uploaded` choice per cluster init.** When the operator opts into the pre-upload path they bypass Proxmox's terraform-driven SHA256 verification (the `data "proxmox_virtual_environment_file"` source has no checksum field — Proxmox just trusts the file already at `<storage>:iso/<filename>`); the audit entry's `Details` now includes `image_pre_uploaded: bool` and `edit_mode: bool` so the forensic trail makes it clear when integrity was verifier-skipped on a given cluster init. Wizard hint also calls out the operator's responsibility to verify the SHA256 before checking the box.

## [0.1.4] — 2026-05-10

Wizard quality of life: the "Test reachability" button is live (was a coming-soon stub) and operators can now edit a cluster's configuration after first save. No breaking changes; pull `ghcr.io/lazerdude-labs/bandolier/{api,ui,vault-agent,tls-init}:0.1.4` (or `:0.1` / `:latest`) to upgrade.

### Added

- **"Test reachability" button is live** on the initialize wizard's Proxmox step (was a "coming soon" stub through v0.1.3). Pre-save: posts the current form values to a new `POST /api/proxmox/test` endpoint, which runs five validation checks against the operator's Proxmox host and returns a structured per-check result. Checks: endpoint reachable + token authenticates (combined `GET /api2/json/version`), node accessible, VM disk storage has `images` content type, image storage has `iso`, snippets storage has `snippets`. On failure, each check's `detail` field surfaces the precise fix — e.g. "Run: `pvesm set local --content backup,iso,vztmpl,snippets`" for the snippets check. Operator catches misconfigurations at the wizard, before the cluster row gets created. Backend lives in `api/internal/proxmox/` (new package) with httptest-based unit coverage of all five checks plus the bad-token short-circuit, missing-node, and missing-content-type paths.
- **Edit configuration** action on the cluster detail page for `initialized | destroyed | error` clusters (live states still require destroy + redeploy to change config — avoids surprise drift between persisted config and running VMs). Re-opens the initialize wizard pre-populated with the existing values; secret fields (token secret, password, private key, TSIG secret) show as blank with a "Leave blank to keep the existing value" hint. Backed by a new `GET /api/clusters/{id}/initialize` endpoint that returns sanitized values + a `secrets_present` array — secrets are never returned over the wire. The existing `POST /api/clusters/{id}/initialize` handler now merges empty secret fields with current Vault values when the cluster is in an editable state, so the operator only re-types secrets they actually want to change. State machine extended: `initialized → initializing` and `destroyed → initializing` are now permitted transitions; live states (deploying, ready, upgrading, destroying, degraded) explicitly block re-init.

## [0.1.3] — 2026-05-10

Operator-config plumbing release: actually respect the wizard's storage fields, fix a silent fallback to `local-lvm` for the cloud-init drive, add a `proxmox_snippets_storage` config field for non-standard snippet storages, and ship two ops-side docs covering the Proxmox setup and the failure modes real operators have hit. Pull `ghcr.io/lazerdude-labs/bandolier/{api,ui,vault-agent,tls-init}:0.1.3` (or `:0.1` / `:latest`) to upgrade.

### Fixed

- **VM disks now respect the operator's `proxmox.storage` form field.** Reported by an early user with a Ceph RBD-backed Proxmox setup. Through v0.1.2, `terraform/main.tf` hardcoded `datastore_id = "local-lvm"` (3×) inside the `vm_definitions` map regardless of what the operator put in the initialize wizard. The form input was silently dropped, and any host without `local-lvm` (RBD-backed setups, Ceph-only homelabs, etc.) couldn't deploy at all. The hardcoded values are now `var.proxmox_storage`, which is already wired through the Go-side `BuildTfvars`.
- **Cloud-init drive lands on the same datastore as the VM disk.** Same root cause as above: the bpg/proxmox provider's `initialization` block defaults to `local-lvm` when `datastore_id` is unset, which silently overrode the operator's storage config for the cloud-init drive. The `initialization` block in `terraform/modules/vm/main.tf` now sets `datastore_id = var.datastore_id` explicitly, so the cloud-init drive lands wherever the disk does.

### Added

- **`proxmox_snippets_storage` config field** for operators whose `local` storage doesn't have the `snippets` content type enabled. Threads through the wizard ("Snippets storage" field on the Proxmox step), `proxmox.snippets_storage` in the cluster's Vault config, the `proxmox_snippets_storage` tfvar, and `terraform/main.tf`'s `proxmox_virtual_environment_file` resource. Defaults to `"local"` so existing setups are unaffected. If you'd rather enable snippets on `local`, the Proxmox-side command is `pvesm set local --content backup,iso,vztmpl,snippets`.
- **`TROUBLESHOOTING.md`** consolidating real operator-reported failures and verified fixes: required Proxmox token permissions (`PVEDatastoreAdmin` per storage), snippets-content-type setup, the Rocky CDN HEAD-block + manual pre-upload workaround, the host-source / container-mount / live-workspace path triad, and useful diagnostic commands.
- **`docs/proxmox-setup.md`** — step-by-step Proxmox-side setup guide covering API token creation (UI + SSH), token permissions per storage, storage content types, VLAN-aware bridge configuration, the cloud-image catalog vs. pre-upload paths, and a verification checklist that maps directly onto the initialize wizard's fields. Linked from the README's Prerequisites section.

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
