# Security policy

## Supported versions

Bandolier is in active 0.x development. Only the latest minor release receives security fixes; older 0.x releases are not maintained. Once 1.0 ships, this policy will be updated to cover the most recent two minor versions.

| Version | Supported |
|---|---|
| 0.1.x | ✅ |
| < 0.1 | ❌ |

## Reporting a vulnerability

**Do not file a public issue for a vulnerability.**

Email the maintainer at `security@lazerdude-labs.dev` with the details. We acknowledge receipt within 72 hours and aim to issue a fix within 30 days for high-severity issues.

When reporting, include:

- What you found and the impact (read access, write access, code execution, etc.).
- A minimal reproduction (command-line or browser steps that exhibit the issue).
- The Bandolier version (`git describe` or `docker image inspect ghcr.io/lazerdude-labs/bandolier/api`).
- Whether you've published anything about the issue elsewhere.

We'll respond on the same email thread to coordinate a disclosure timeline.

## Scope

In scope:

- Code in `api/`, `ui/`, `terraform/`, `ansible/`, `deploy/`.
- Default configuration shipped with the stack (Vault policy, Compose definitions, Dockerfiles, CI workflows).
- Documentation that describes a security guarantee (e.g. `deploy/vault-init/THREAT_MODEL.md`).

Out of scope:

- Issues that require host-level compromise of the machine running Bandolier. The threat model assumes the operator host is trusted; see `deploy/vault-init/THREAT_MODEL.md` for the explicit boundary.
- Issues caused by the operator deviating from the documented deployment topology (exposing the UI to the public internet without TLS terminator, running Bandolier on a multi-tenant host, etc.).
- Issues in upstream dependencies (HashiCorp Vault, Terraform, Ansible, k3s). Report those upstream; we'll pin or work around once notified.

## Public disclosure

After a fix is released, we publish a brief advisory in the repository's [Security Advisories](https://github.com/lazerdude-labs/bandolier/security/advisories) tab and a corresponding entry in `CHANGELOG.md`. Reporters are credited unless they request otherwise.
