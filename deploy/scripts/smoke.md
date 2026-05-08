# Bandolier Manual Smoke (Phase 1.5)

Run after `bash deploy/scripts/smoke.sh` (or as a standalone UI walkthrough).

## 0. Boot

```bash
cd deploy && docker compose down -v && docker compose up -d --build
```

Open https://localhost. Accept self-signed cert.

## 1. First-run setup (`/setup`)

- Brand block shows "BANDOLIER · First-run setup · LazerDude Labs · v1.0.0"
- Master password + Confirm fields
- Eye toggle on each password field shows/hides the value
- Info box: "Vault unseal keys (5 of 5, 3-threshold)" — informational, not yet wired
- TLS footer: "127.0.0.1:443 · self-signed TLS"
- Submit → lands on `/login`

## 2. Login (`/login`)

- Same brand block + password field with eye toggle
- TLS footer + disabled "vault sealed?" stub
- Theme toggle (sun/moon) bottom-right
- Submit → lands on `/clusters`

## 3. Fleet view (`/clusters`)

- Top bar: BANDOLIER + "LazerDude Labs" tag (left), centered cluster switcher pill, right-side icon group (history / theme / settings / logout / avatar "O")
- Header: "Clusters" + sub-line "N clusters across 4 profiles · fleet view" + "+ New cluster" button
- 4 profile summary cards (icon + label + tag pill + big number + "N/total ready"):
  - Homelab · PRODUCTION (clickable, accent emerald)
  - Red Team · SCENARIO (greyed, Coming-soon pill, accent rose)
  - Blue Team · SCENARIO (greyed, Coming-soon pill, accent sky)
  - Grey Space · TARGET (greyed, Coming-soon pill, accent amber)
- Click a card → filters table; click again → all
- Filter pills row: All(N) + per-profile pills with colored dots
- Table columns: Cluster (name + sub-id), Profile (dot + label), Status, Nodes, Network, k3s, Last deploy, chevron
- Footer mutex note: "Cross-cluster operations run concurrently · per-cluster mutex prevents overlap"

## 4. New cluster (`/clusters/new`)

- Breadcrumb: Clusters / New
- Subtitle: "Pick a profile. Profiles bundle Terraform modules, Ansible playbooks, and Helm charts that fit a scenario."
- 2x2 profile picker grid; only Homelab selectable; v3 stubs show ComingSoonPill
- Cluster name card with vault path hint
- Cancel + "Create & configure →" buttons
- Create → navigates to `/clusters/$id/initialize`

## 5. Initialize wizard (`/clusters/$id/initialize`)

3-column layout:
- **Left rail:** Stepper (Proxmox / Network / SSH) — numbered circles, subtitle per step, click to jump (back free, forward validates)
- **Center:** form card, max-width ~560px, header "Step name · step N of 3"
  - Proxmox step: endpoint / node / storage / token id / token secret / SSH user / SSH password / ISO file name + "Test reachability" Coming-soon button
  - Network step: FQDN / CIDR / Gateway / DNS / master IP / agent IPs / VLAN / bridge name
  - SSH step: info card explaining Ed25519 keypair auto-generation
  - Back / Continue (or "Save & continue to deploy" on final step)
- **Right rail:** LiveSummary card (sticky) — updates on every keystroke; em-dash for unset fields

Submit → toast "Cluster initialized" → navigates to cluster overview

## 6. Cluster overview (`/clusters/$id`)

- Breadcrumb: Clusters / cluster-name
- Header: profile-colored dot + h1 + status badge + meta line (sub-id · N nodes · k3s vN · created Xd ago · last deploy Yh ago)
- **Live deploy banner** appears when status is `deploying|destroying|upgrading|initializing` and a deployment is running — clickable to live log
- **Action bar** (horizontal): primary (state-derived: Initialize / Deploy / Redeploy / Retry / Upgrade) | divider | secondary Upgrade (Coming soon) + Destroy (when ready/degraded/error) | spacer | small kubeconfig + Helm (Coming soon)
- **Left column (2/3 width):**
  - Nodes card: 6-column NodeTable (Name / Role / IP / Proxmox / k3s / Last health) with em-dashes for missing telemetry
  - Inline caption: "Per-node Proxmox / k3s / health columns surface real values once telemetry lands in Plan 2 phase 2."
  - Recent deployments table with "View all →" link to `/clusters/$id/deployments`
- **Right column (1/3 width):**
  - **Connection card**: FQDN + API endpoint (live from Vault), kubeconfig + join token (Coming soon)
  - **Network card**: CIDR / Gateway / DNS / FQDN / Master IP / Agent IPs (live from Vault)
  - **Proxmox card**: stubbed em-dash + Coming-soon pill (security: token id retrieval lands phase 2)

## 7. Deploy → live log view

Click Deploy → routes to `/deployments/$id`:
- Header: breadcrumb (Clusters / id / operation), h1 with operation name, status badge
- Right side: History + Back to cluster buttons
- Meta line: deployment id · started · duration
- 2-col grid: Steps (left, sticky) | LogStream (right, fixed height)
- LogStream toolbar:
  - Tabs: all / stdout / stderr / ansible (with counts)
  - Search input → filters lines + highlights matches with yellow `<mark>`
  - Pause/Resume autoscroll toggle (icon button with tooltip)
  - Copy visible icon button
- Bottom DeployBanner:
  - Running: spinner icon + message + Cancel (Coming soon)
  - Succeeded: green check + "Deployment succeeded in Xs." + View kubeconfig (Coming soon)
  - Failed: red X + error message + Retry (Coming soon)

## 8. Deployment history (`/clusters/$id/deployments`) — NEW route

Reachable via:
- "View all →" link in cluster overview's Recent deployments
- History icon in top bar (when a cluster is in context)

Page layout:
- Breadcrumb: Clusters / name / Deployments
- FilterPills: All / Succeeded / Failed / Running with counts
- Table: ID (mono accent) / Operation / Status / Started / Duration / Actor (em-dash) / chevron
- Row click → `/deployments/$id`

## 9. Destroy lifecycle

From cluster overview, click Destroy → modal with type-cluster-name confirmation → status flips through `destroying` → `destroyed`. Vault secrets (proxmox/network/ssh) retained; redeploy works without re-entering creds.

## 10. Settings (`/settings`)

4 sections:
- **Vault card**: status badge (sealed/unsealed), KV grid (Address / Version / Initialized / Auto-unseal / Auth), Show key fingerprints + Rotate root token (Coming soon)
- **Master password**: current / new / confirm form, "Update password"
- **Backup & restore**: 2-card layout (Download w/ status-ready border-l, Restore w/ destructive border-l) — both buttons Coming soon
- **Stack info**: ui / api / vault image tags + bind address (compose v1.0.0 mono badge)

## 11. Theme + navigation

- Theme toggle (top bar sun/moon icon) flips between dark and light — both render correctly across all routes
- Hard-refresh on each route — no state regressions, WebSocket replays history, StepList correctly tracks past steps
- ⌘K opens cluster switcher dropdown anywhere in the app — search filters by name + FQDN
- Top bar history icon → cluster's deployment history when a cluster is in context

## 12. Coming-soon stub inventory (rendered as pills/em-dashes/disabled controls)

Visible across the UI as Plan 2 phase 2 backlog markers:

- Per-node Proxmox VM ID / k3s version / health columns (NodeTable)
- kubeconfig retrieval + join token display (Connection card)
- Proxmox URL/Node/Storage/Token-id metadata + reachability test (Proxmox card)
- Vault unseal key fingerprints + Rotate root token (Settings)
- Backup download + Restore upload (Settings)
- Action rail Upgrade + Helm + cancel deploy + retry on failure
- Test reachability button on initialize wizard
- Deployment "Actor" tracking (history page)
- BYO SSH public key on initialize wizard

These are intentional stubs. Each carries a "Coming soon" pill or em-dash so operators know the surface exists but isn't wired yet.

## Phase 2 additions

After `docker compose down -v && docker compose up -d --build`:

15. **Telemetry** — Deploy a homelab cluster, wait for `ready`. Cluster overview NodeTable now shows real `v1.31.12+k3s1` k3s version per node, "Ready 12s ago" relative time, and `pve-XX · vm-NNN` per node. Stop Proxmox API access (or break creds) → reload → k3s columns stay populated, Proxmox columns em-dash.

16. **Kubeconfig** — Connection card shows a download link `<cluster-name>.yaml ↓`. Click → file downloads. `kubectl --kubeconfig=<file> get nodes` returns 3 Ready nodes.

17. **Manual Retrieve** — If the auto-fetch failed (look in deploy log for "kubeconfig retrieval failed"), Connection card shows "Retrieve" button. Click → toast "kubeconfig retrieved" → download link replaces button.

18. **Upgrade** — Cluster overview ActionBar Upgrade button → modal opens prefilled with current version. Enter `v1.32.5+k3s1` → Confirm → redirected to deploy log → ansible runs upgrade → cluster returns to `ready` with new version visible in NodeTable.

19. **Audit log** — Top bar Activity icon → `/activity` shows full timeline. FilterPills (All / Auth / Clusters / Failed) work. Filter by Failed surfaces any prior failed deploys. Settings page shows "Recent activity" card with last 5 entries; "View all →" navigates to `/activity`.

20. **Audit instrumentation coverage** — Verify entries appear after each:
    - Login → `auth_login` success
    - Wrong-password attempt → `auth_login` failure
    - Cluster create → `cluster_create` success
    - Cluster initialize → `cluster_initialize` success
    - Deploy → `cluster_deploy` started + (succeeded | failed)
    - Destroy → `cluster_destroy` started + succeeded
    - Upgrade → `cluster_upgrade` started + (succeeded | failed)
    - Password change → `change_password` success

## Phase 3 additions

After `docker compose down -v && docker compose up -d --build`:

21. **Traefik in deploy** — Initialize a fresh cluster `apps-smoke-1`. Watch the deploy log; expect a `helm.install_traefik` step after `wait_for_ssh` and `ansible`. Cluster only flips to `ready` once Traefik is healthy. Verify by checking the deploy step list shows `helm.install_traefik` ✓.

22. **Connection card Traefik URL** — On `/clusters/apps-smoke-1`, the Connection sidebar shows a Traefik row with `https://traefik.<fqdn>` as a clickable link. Click → opens Traefik dashboard (assumes wildcard DNS configured per CLAUDE.md).

23. **Apps page lands** — Click ActionBar Apps button → `/clusters/apps-smoke-1/apps` renders with three tabs. Installed shows Traefik with SYSTEM badge. Catalog shows curated entries plus bitnami / grafana / prometheus-community / traefik repos pre-added at create time. Repos shows the four factory defaults.

24. **Add a repo** — Repos tab → Add repo, name `harbor`, URL `https://helm.goharbor.io`. Verify success toast, row appears in table. Catalog tab → harbor filter pill exists, harbor charts visible.

25. **Install grafana** — Catalog tab → grafana → install. Modal pre-fills release name `grafana`, namespace `default`. Expand Hostname; auto-suggested `grafana.<fqdn>`. Click Install → redirected to live install view → step `helm.install` runs → green banner. Apps list shows grafana with status Ready, URL clickable, no SYSTEM badge.

26. **Atomic rollback on failure** — Catalog tab → install something with intentionally bad values (e.g. wrong storage class). Watch live view; expect `helm.install` exits failed, banner red, install row marked failed. Cluster has no orphan resources (verify via `kubectl get all -n default`).

27. **Uninstall non-system** — Apps list grafana row → trash icon → modal opens (no name-typing required) → click Uninstall → live view streams uninstall → grafana row disappears.

28. **Uninstall Traefik blocked** — Apps list traefik row → trash icon → modal requires typing `traefik` to confirm. Without typing, button stays disabled.

29. **Audit coverage** — `/activity` shows: `app_repo_add` success (harbor), `app_install` started+succeeded (grafana), `app_install` started+failed (the broken-values one), `app_uninstall` started+succeeded (grafana). All entries have actor_id captured.

30. **Profile collapse** — Create a redteam-tagged cluster `apps-smoke-2`. Verify it produces functionally identical infrastructure to a homelab cluster (3 VMs, Traefik installed, ready). Cluster card on fleet page shows redteam badge / colors.

## Phase 4 additions

After `docker compose down -v && docker compose up -d --build`:

31. **Initialize wizard new fields** — Network step shows DNS server, DNS zone, TSIG name, TSIG secret fields. Submitting with bad TSIG fails the wizard with a clear error (`dns pre-flight: ...`); the cluster does NOT flip to initialized. Fix the TSIG and resubmit; cluster reaches initialized.

32. **Wildcard DNS at deploy** — Initialize + deploy `phase4-smoke-1`. Watch deploy log; expect `dns.write_wildcard` ✓ → `tls.issue_wildcard` ✓ → `helm.install_traefik` ✓ before `ready`. Verify with `dig *.<fqdn> @192.0.2.5` returns `<master_ip>`. Test arbitrary subdomain: `dig random.<fqdn> @192.0.2.5` also returns `<master_ip>`.

33. **Wildcard TLS** — `vault kv get bandolier/clusters/<id>/wildcard_cert` shows cert + key + chain + expires_at. `kubectl get secret bandolier-wildcard-tls -n kube-system` exists with matching data. Open `https://traefik.<fqdn>` — cert is valid (no browser warning), Traefik dashboard loads. Open `https://anything-else.<fqdn>` — Traefik 404 page loads with valid cert.

34. **Bundle install** — Apps → Catalog. The `homelab-starter` entry shows with a BUNDLE pill and "install bundle" button. Click → modal shows nginx chart preselected (required, can't uncheck) with hostname `demo-nginx.<fqdn>`. Click Install bundle → live install view → step `helm.install:demo-nginx` runs → green banner. Apps list shows demo-nginx as a normal release. Click URL → nginx welcome page at `https://demo-nginx.<fqdn>` with valid cert.

35. **Ingress probing** — Install a chart that doesn't respect `ingress.hostname` (try a chart that uses `host` instead, e.g. some bitnami legacy chart). Install succeeds. Apps list URL cell shows ⚠ glyph; tooltip explains. Install log has the warning line. Re-install with Advanced > Hostname value path = `host`. Install succeeds without the warning glyph.

36. **Cert renewal** — Manually edit `bandolier/clusters/<id>/wildcard_cert` to set `expires_at` to a date within 7 days. Wait for the next hourly tick (or restart the API to trigger immediately). Check audit log: `cluster_cert_renew` row appears with old + new expires_at. Verify Vault has the new cert and the kube-system Secret was updated.

37. **Cert renewal — Connection card** — Connection sidebar shows "Wildcard *.<fqdn>  ·  expires <date>" where the date colors yellow if within 14 days, red if within 3 days, default otherwise.

38. **Audit coverage** — `/activity` shows: `cluster_dns_write` + `cluster_cert_issue` (one each per deploy), `app_bundle_install` started + succeeded, `cluster_cert_renew` (after the manual renewal trigger).

39. **Stub adapter feedback** — Initialize a fresh cluster with `pfsense` as the DNS kind (manually set in Vault — wizard always posts `bind` in v1). Test connection button returns `ErrNotImplemented`-style error. Confirms the stub adapter framework is in place.

40. **DNS off** — Initialize a cluster `phase4-smoke-3` with the "Manage DNS automatically" checkbox unchecked. Wizard submits without DNS pre-flight. Deploy log shows `dns.write_wildcard` ✓ as a no-op (no actual nsupdate call). Cluster reaches `ready`. Manually wire DNS to point `*.<fqdn>` at master IP via your own DNS provider. App URLs then resolve and show valid TLS (wildcard cert was still issued).

## Phase 5 additions

After `docker compose down -v && docker compose up -d --build`:

41. **WS auth — happy path** — Open dev tools Network tab; trigger a deploy. Observe two requests in order: `POST /api/auth/ws-token` returning `{token, expires_at}`, then `GET /ws/deployments/<id>/logs` upgrade with `Sec-WebSocket-Protocol: bandolier.ws.<token>` request header AND echoed in the response. Log streaming works as before.

42. **WS auth — rejection** — Manually delete the session cookie via dev tools. Trigger a deploy. `POST /api/auth/ws-token` returns 401; UI shows the failed log query (no crash). Log into the UI again, retry; works.

43. **WS auth — bypass attempt** — In a separate terminal, try connecting directly without a token: `wscat -c wss://localhost/ws/deployments/anything/logs --no-check`. Connection refused with 401 before upgrade.

44. **Vault renewal — proactive** — Leave the API container running. Visit `/settings`; Vault card shows token TTL counting down and last-renewed timestamp. After ~30 minutes (default LifetimeWatcher renewal cadence at half-TTL), refresh; TTL has reset and last-renewed has advanced. `/activity` shows a new `vault_token_renew` row with `source: renew`.

45. **Vault renewal — fallback re-login** — `docker compose exec vault vault token revoke <current api token>` (find the token via `vault list auth/token/accessors` filtered to the bandolier role). Within ~5-10 seconds, the API auto-re-logs in via role_id+secret_id. `/api/health` reflects fresh TTL. `/activity` shows a `vault_token_renew` row with `source: relogin`.

46. **Proxmox CA — strict mode** — Initialize a cluster with the Proxmox CA bundle field expanded and a valid CA pasted in. Verify `/api/clusters/<id>/nodes` returns enriched data (Proxmox VM IDs populated). The handshake succeeds against the strict TLS config.

47. **Proxmox CA — backward compat** — Initialize another cluster without the CA field expanded. Verify `/api/clusters/<id>/nodes` still returns enriched data (skip-verify path; no behavior change vs Phase 4).

48. **Proxmox CA — malformed PEM** — Initialize a cluster with the CA field set to garbage text (e.g. "not a cert"). Cluster creation succeeds but `/api/clusters/<id>/nodes` returns an error in the Proxmox section. Operator removes / fixes the bad PEM in Vault and the next telemetry refresh recovers.

49. **Telemetry cache — singleflight** — Open `/clusters/<id>` in two browser tabs simultaneously after `docker compose restart api`. `docker compose logs -f api` shows only ONE `kubectl get nodes` call (deduplicated).

50. **Telemetry cache — negative TTL** — Manually break a cluster's kubeconfig in Vault (set `clusters/<id>/kubeconfig.yaml` to empty). Visit `/clusters/<id>`; nodes list is empty (probe failed → empty rows cached). Restore the kubeconfig. Wait 5 seconds; visit again; data flows. Without the negative TTL change this would have been 30 seconds.

## Phase 6 additions

After `docker compose down -v && docker compose up -d --build`:

51. **Image catalog renders** — Initialize wizard Proxmox step shows "Image source" radio with "Rocky 9 GenericCloud" pre-selected in the dropdown. The "Custom" radio is unchecked. Image storage field defaults to "local". `GET /api/distros` (Network tab) returns the catalog with one Rocky 9 entry (id, label, url, sha256, file_name).

52. **Catalog flow — fresh deploy** — Submit the wizard with defaults (Rocky 9 catalog mode). Vault `clusters/<id>/proxmox` contains `distro: rocky9`, `image_storage: local`, empty `custom_url`/`custom_sha256`, and NO `iso_file_name`. Click Deploy; live log shows `terraform.apply` creating `proxmox_virtual_environment_download_file.cloud_image` (Proxmox host fetches Rocky qcow2 with sha256 verification). VMs boot from the resulting file_id `local:iso/Rocky-9-GenericCloud.latest.x86_64.qcow2`.

53. **Verify file landed on Proxmox** — SSH to Proxmox host: `ls -la /var/lib/vz/template/iso/Rocky-9-GenericCloud.latest.x86_64.qcow2`. File exists with the expected size (~600MB). `sha256sum` matches the catalog pin (`15d81d3434b298142b2fdd8fb54aef2662684db5c082cc191c3c79762ed6360c` at v1).

54. **Custom URL flow** — Initialize a fresh cluster. On Proxmox step pick "Custom" radio. Advanced expands; paste `https://dl.rockylinux.org/pub/rocky/9/images/x86_64/Rocky-9-GenericCloud.latest.x86_64.qcow2` + the matching sha256 from the .CHECKSUM file. Submit; deploy. Same happy path as catalog flow but exercising the custom URL code path.

55. **Validation rejection — both modes** — Open browser dev tools, submit an init request with both `distro: "rocky9"` AND `custom_url: "https://..."`. Backend returns 400 with `image: exactly one of distro or custom_url required`. Frontend's Zod refine catches the same case before submit.

56. **Validation rejection — custom URL without sha** — Submit with `custom_url` set but `custom_sha256` empty. Backend returns 400 with `custom_sha256 required when custom_url is set`. Frontend rejects the submit before it reaches the backend.

57. **Idempotent re-deploy** — `terraform apply` a second time on the same cluster (no underlying changes). Plan shows zero changes for the download_file resource (`overwrite = false` makes the resource detect the file exists and skip).

## Phase 7 additions

After `docker compose down -v && docker compose up -d --build`:

58. **Cancel deploy mid-flight** — Initialize cluster `cancel-test-1`. Click Deploy. Halfway through `terraform.apply`, click the Cancel button on the live deploy banner (now ENABLED, not Coming-soon). Confirmation modal accepts. Banner flips to `cancelled` variant ("Deploy cancelled at <step>. Click Destroy to clean up."). Cluster overview shows status `error`. `/activity` shows `cluster_deploy started → cancelled` audit pair. Click Destroy → terraform destroy cleans up half-applied VMs; cluster lands at `destroyed`.

59. **Retry on failure** — Force a deploy failure (e.g. break the Proxmox endpoint URL in Vault). Banner shows `failed` variant with enabled Retry button. Click Retry → fresh deployment row; `/clusters/$id/deployments` history page shows two rows. After fixing the endpoint, retry succeeds and cluster reaches `ready`.

60. **Join token — auto-fetch** — Successful deploy. Connection card Join token row shows the masked token (`K10::abcd1234...wxyz5678`) with a copy button. Click Copy; clipboard now has the full token. Use it from another node: `k3s agent --token ... --server https://<fqdn>:6443` succeeds.

61. **Join token — manual retry** — Manually delete `clusters/<id>/join_token` from Vault. Connection card switches to a Retrieve button (cluster is `ready`). Click Retrieve → token re-fetched, masked display reappears.

62. **BYO SSH key** — Initialize a new cluster `byo-test-1`. SSH step: paste a pre-generated keypair (public + private). Wizard accepts; live summary shows "BYO key". Submit; deploy proceeds with operator's key. SSH to master directly using your local copy of the private key — works.

63. **BYO validation** — Initialize again, paste only public key (private blank). Wizard rejects with "Both public + private key required (or both blank for auto-gen)". Server-side check in initialize handler also rejects with 400.

64. **Actor tracking** — Log in as user-1, deploy a cluster. `/clusters/<id>/deployments` history table Actor column shows `user-1` (instead of em-dash). Older pre-Phase-7 deployments still show em-dash (NULL actor_id).

65. **WS reconnect** — Start a long-running deploy. Mid-deploy, simulate a network blip: `docker compose stop ui` then ~3s later `docker compose start ui`. LogStream toolbar shows "reconnecting in 1s…" then resumes log flow. No log lines lost during the reconnect (Hub `Subscribe` replay handles backfill).

66. **WS reconnect terminal** — After a deploy succeeds, wait ~10s, kill the WS connection (browser dev tools → Network → close socket). The hook does NOT attempt to reconnect (terminal `deployment_complete` event already received).

67. **Apps install Cancel still Coming-soon** — Phase 7 wires Cancel for deploy/destroy/upgrade only; the install live view's Cancel button stays disabled in v1. Phase 8+ adds the apps-side cancel handler.
