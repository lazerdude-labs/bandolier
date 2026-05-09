# Security policy

## Supported versions

Bandolier is in active 0.x development. Only the latest minor release receives security fixes; older 0.x releases are not maintained. Once 1.0 ships, this policy will be updated to cover the most recent two minor versions.

| Version | Supported |
|---|---|
| 0.1.x | ✅ |
| < 0.1 | ❌ |

## Reporting a vulnerability

**Do not file a public issue for a vulnerability.**

Use GitHub's private vulnerability reporting:

➡️ **[github.com/lazerdude-labs/bandolier/security/advisories/new](https://github.com/lazerdude-labs/bandolier/security/advisories/new)**

This is the only supported disclosure channel. Reports go directly to the maintainers in an end-to-end encrypted thread on GitHub, with an audit trail. We use this channel to coordinate triage, fix, and disclosure timing.

When reporting, include:

- What you found and the impact (read access, write access, code execution, etc.).
- A minimal reproduction (command-line or browser steps that exhibit the issue).
- The Bandolier version (`git describe`, the GitHub release tag, or the image tag from the Settings page).
- Whether you've published anything about the issue elsewhere.

## Response timeline

Bandolier is a single-maintainer project. We do our best to respond promptly but cannot offer a binding SLA:

- **Acknowledgement:** within a few business days, typically.
- **Triage and fix:** depends on severity and complexity. We coordinate the timeline with you in the advisory thread. For most issues, expect weeks rather than days. For critical issues (remote code execution, secret exfiltration), we prioritize and aim for a fix or workaround within a month — but we'll tell you honestly if that's not feasible.
- **Disclosure:** coordinated. We default to publishing an advisory once a fix is available; reporters can request earlier or later disclosure for legitimate reasons (academic publication, responsible reuse window, etc.).

## Scope

In scope:

- Code in `api/`, `ui/`, `terraform/`, `ansible/`, `deploy/`.
- Default configuration shipped with the stack (Vault policy, Compose definitions, Dockerfiles, CI workflows).
- Documentation that describes a security guarantee (e.g. `deploy/vault-init/THREAT_MODEL.md`).

Out of scope:

- Issues that require host-level compromise of the machine running Bandolier. The threat model assumes the operator host is trusted; see [`deploy/vault-init/THREAT_MODEL.md`](deploy/vault-init/THREAT_MODEL.md) for the explicit boundary.
- Issues caused by the operator deviating from the documented deployment topology (exposing the UI to the public internet without TLS terminator, running Bandolier on a multi-tenant host, etc.). The defaults bind to `127.0.0.1`; reports about behavior under deliberately weakened configuration aren't tracked here.
- Issues in upstream dependencies (HashiCorp Vault, Terraform, Ansible, k3s, the React stack). Report those upstream; we'll pin or work around once notified.

## Public disclosure

After a fix is released, we publish a brief advisory in the repository's [Security Advisories](https://github.com/lazerdude-labs/bandolier/security/advisories) tab and a corresponding entry in `CHANGELOG.md`. Reporters are credited unless they request otherwise.
