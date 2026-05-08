# Bandolier

> *Every cluster, on the belt.*

**A LazerDude Labs project.** Bandolier is a self-contained Docker Compose stack that deploys k3s clusters on Proxmox through a React UI — no controller VM, no host-side Vault, no bash glue.

## What it does

- `docker compose up` on any host with Docker + Proxmox network reachability.
- A web UI prompts for a master password, then collects Proxmox + network + SSH inputs.
- Provisions a 1-server + 2-agent k3s cluster end-to-end via Terraform + Ansible.
- Streams live deployment logs to the browser.
- Stores all credentials in a containerized HashiCorp Vault (no host-side Vault required).
- Vault auto-recovers on restart via an in-stack unseal watcher.

## Quick start

```bash
git clone https://github.com/lazerdude-labs/bandolier.git
cd bandolier
./deploy/scripts/smoke.sh
# Open https://127.0.0.1 in your browser; accept the self-signed cert.
```

First-run setup asks for a master password. Subsequent visits log you straight into the cluster overview. The stack binds to `127.0.0.1:443` by default — loopback-only — so nothing is reachable from the LAN until you explicitly expose it.

### What gets deployed

Three containers and a small set of named volumes:

| Container | Role |
|---|---|
| `vault` | HashiCorp Vault — KV, AppRole, PKI |
| `vault-agent` | Idempotent first-run setup + long-running unseal watcher |
| `api` | Go service driving Terraform + Ansible, serving the REST/WS API |
| `ui` | Static React build behind nginx (terminates TLS to localhost) |

Volumes: `vault-data`, `vault-init-state`, `tf-state`, `app-data`, `tls`.

### Prerequisites

- Docker + Docker Compose v2.
- A reachable Proxmox host with an API token that can clone a cloud-init template (Rocky 9, Ubuntu, etc.).
- A `/24` (or larger) on a VLAN routable from your Proxmox host.
- A wildcard DNS record (or an authoritative DNS server you can update via TSIG) for the cluster's FQDN — optional, only required if you want Bandolier to issue per-app wildcard certs.

## How it's organized

```
api/        Go backend — Vault client, Terraform/Ansible drivers, REST/WS handlers
ui/         Vite + React + TypeScript frontend
terraform/  Proxmox VM provisioning module
ansible/    k3s + Traefik configuration
deploy/     docker-compose.yml + container images for vault, vault-agent, ui
```

The data model is designed for multi-cluster from day one even though v0.1 ships with single-cluster scope. Future profiles (red-team / blue-team scenario clusters) plug into the same shape.

## Tear down

```bash
cd deploy
docker compose down       # keep volumes (resume later)
docker compose down -v    # destroy volumes (fresh start)
```

## Security model

Bandolier is designed for a single trusted operator on a single host. The Vault unseal keys live in a docker volume on the host so that auto-recovery works after reboot — see [`deploy/vault-init/THREAT_MODEL.md`](deploy/vault-init/THREAT_MODEL.md) for the full trust boundary. If you need a stricter model, run Bandolier on a dedicated host gated by other means (separate VLAN, jump host, hardware-backed auto-unseal).

## Versioning

Bandolier follows [SemVer](https://semver.org). Breaking changes bump the major version. Pre-1.0 (`0.x.y`), minor versions may include breaking changes; CHANGELOG entries call those out explicitly.

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md). Bug reports and pull requests welcome via GitHub Issues.

## License

[MIT](LICENSE) — © LazerDude Labs.
