<!--
SYNC IMPACT REPORT
==================
Version change: TEMPLATE (uninitialized) → 1.0.0 (initial ratification)

Modified principles: N/A (initial ratification — all principles are new)

Added sections:
  - Core Principles (6 principles):
      I. Security-First (Default-Deny, Fail-Closed) — NON-NEGOTIABLE
      II. Stateless API & Self-Contained Stack
      III. Streaming Reliability
      IV. Validation Standard — NON-NEGOTIABLE
      V. Radical Simplicity
      VI. Versioning & Workflow Discipline
  - Relationship to Other Project Docs
  - Development Workflow
  - Governance

Removed sections: none (placeholder template replaced wholesale)

Templates requiring updates:
  ✅ .specify/templates/plan-template.md
     — "Constitution Check" gate self-derives from this file; no template edit needed.
  ✅ .specify/templates/spec-template.md
     — No constitution references in template; no edit needed.
  ✅ .specify/templates/tasks-template.md
     — No constitution references in template; no edit needed.

Runtime guidance docs reviewed:
  ✅ CLAUDE.md (project root) — already aligned; constitution defers to it for
     environment, workflow, and homelab conventions per precedence rules below.
  ✅ ~/.claude/CLAUDE.md (user root) — already aligned; constitution inherits
     validation standard and security baseline from it.
  ✅ docs/superpowers/specs/2026-04-30-bandolier-v1-design.md — referenced as
     authoritative v1 design; constitution principles cross-checked against it.

Follow-up TODOs: none. All placeholders concretely filled.
-->

# Bandolier Constitution

## Core Principles

### I. Security-First (Default-Deny, Fail-Closed) — NON-NEGOTIABLE

All security controls default to denying access; failure of any check halts the
operation rather than continuing on the happy path.

- **Secrets** live exclusively in HashiCorp Vault. They MUST NEVER appear in
  environment variables, configuration files, command-line arguments, log
  streams, or source code.
- **Proxmox authentication** uses API tokens only. Password authentication is
  prohibited.
- **Network exposure** defaults to loopback: the UI binds `127.0.0.1:443`.
  LAN exposure requires an explicit Docker Compose override file and is never
  the default.
- **Trust boundary** is single-operator. Shell access to the host equals Vault
  access by design; this is intentional and bounds the threat model.
- **Encryption in transit** applies everywhere, including internal VLANs:
  TLS-only on the UI, mTLS between `api` and `vault`.
- **Subprocess invocation** (Terraform, Ansible, vault CLI) passes inputs via
  environment variables or JSON files only. User-supplied strings MUST NEVER
  appear on a command line.
- **Log sanitization** runs as middleware before any stdout/stderr capture
  reaches logs, the WebSocket stream, or the UI. Secret-field redaction is
  non-optional.
- **Audit logging** records every state-changing action with actor, target,
  outcome, and timestamp.
- **Container hardening** is mandatory: non-root UIDs, `read_only: true`,
  `cap_drop: ALL`, no privileged mode, no host networking.

**Rationale**: Bandolier deploys and operates Kubernetes clusters with full
Proxmox infrastructure credentials. A single compromise of these controls
equals full lab compromise. Default-deny + fail-closed is the only posture
compatible with that blast radius.

### II. Stateless API & Self-Contained Stack

The API process holds no state across restarts. All persistent state lives in
exactly three places: Vault KV (secrets), SQLite (`app.db`, relational), and
named Docker volumes (`tf-state`, `tls`, `vault-data`, `app-data`). Nothing
persists on the host filesystem outside these volumes.

The Docker Compose stack ships with all tooling inside it: Terraform, Ansible,
Vault, and Python runtimes are in-container, never host-installed. The compose
stack IS the controller.

Provisioning is Terraform + Ansible only. Kubernetes operators, CRDs, and
in-cluster controllers for managing Bandolier itself are prohibited. Terraform
modules and Ansible playbooks MUST be idempotent — the failure-recovery model
is re-converge-on-retry, not step-resume.

Multi-cluster support is structural from day one. Vault paths and SQLite
schemas use `cluster_id` partitioning unconditionally, even when the v1 UI
hides the list view.

**Rationale**: Statelessness means any `api` container can be replaced without
data loss; self-containment means `docker compose up` on any host with Docker
is a complete installation. Idempotency and re-converge collapse the failure-
handling surface area to a single code path.

### III. Streaming Reliability

All long-running operations stream events to the browser over WebSocket.
Polling for operation status is prohibited. The event-type vocabulary is
bounded: `step_start`, `log`, `ansible_event`, `step_end`,
`deployment_complete`.

`docker compose up` MUST bootstrap the full stack — including Vault
initialization and auto-unseal — with zero manual intervention beyond
first-run master password setup.

Per-cluster operations serialize through an in-memory mutex. Concurrent
operations on different clusters are allowed; concurrent operations on the
same cluster return `409 Conflict`.

**Rationale**: Long deployments (5–15 minutes) need live operator feedback;
polling burns CPU and hides failures. Bounded event types let the UI and
backend evolve in lockstep without ad-hoc string parsing.

### IV. Validation Standard — NON-NEGOTIABLE

Code is not "done" until it has been validated a minimum of twice under
different conditions. The default expectation for v1 work is: (1) a green
GitLab pipeline pass, plus (2) a live deploy against the Proxmox lab.

The GitLab pipeline stages are fixed and ordered: `lint → test → validate →
build → e2e`. A red pipeline blocks merge to `main`. No exceptions.

Each plan-scoped feature branch MUST produce working, demoable software at
completion. Half-finished plans MUST NOT merge to `main`.

**Rationale**: This rule inherits directly from `~/.claude/CLAUDE.md` and
exists because "looks right" has shipped broken infrastructure to lab before.
Two passes under different conditions catches the class of bugs where the
unit test passes but the deployed thing doesn't work.

### V. Radical Simplicity

Prefer fewer abstractions over clever architecture. Three similar lines of
code beat a premature abstraction. Do not design for hypothetical future
requirements.

Bandolier MUST run entirely on-premises. No dependency on cloud providers
(AWS, Azure, GCP) at any layer.

v2 and v3 features are explicitly out of scope for v1: cluster list UI,
profile picker, red/blue team profiles, helm chart browser, pod log viewer,
IngressRoute editor, TOTP, OIDC. References to these features in v1 code or
design are permitted only for forward-compatibility checks, never as partial
implementations.

**Rationale**: This is a homelab tool first, a shareable artifact second.
Operator clarity beats engineering polish. Forward-compatibility checks
ensure we don't paint v3 into a corner without paying for v3 features in v1.

### VI. Versioning & Workflow Discipline

SemVer is strictly enforced. Pre-1.0 minor version bumps MAY contain breaking
changes, but every breaking change MUST be documented in `CHANGELOG.md`.

All commits use conventional format: `type(scope): message`. Allowed types:
`feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `perf`, `ci`, `build`.

GitLab (`gitlab.rplab.lan`) is the authoritative source of truth for source
control. GitHub (`github.com/lazerdude-labs`) receives release-ready artifacts
via deliberate, manual push by the operator. Automated GitHub publish is
prohibited.

**Rationale**: Two upstreams without discipline guarantee divergence. The
manual GitHub step is intentional friction that prevents WIP from leaking to
the public-facing brand surface.

## Relationship to Other Project Docs

This constitution does not exist in isolation. The following documents define
the rest of the operating envelope:

- **Authoritative v1 design**:
  `docs/superpowers/specs/2026-04-30-bandolier-v1-design.md`. Treat as source
  of truth for architecture, data model, API surface, and state machine.
  Deviations require explicit operator approval.
- **Project-level conventions**: `CLAUDE.md` (project root) — git workflow,
  branch naming, plan-N execution pattern, inherited-asset rules for
  `terraform/` and `ansible/`.
- **Homelab-level conventions**: `~/.claude/CLAUDE.md` — network topology
  (VMBR0/VMBR1, VLAN 10/20/30), validation standard, security baseline,
  RHEL/SELinux defaults.

**Precedence rules** when a rule exists in both this constitution and a
`CLAUDE.md` file:

1. `CLAUDE.md` is authoritative for **environment, workflow, and homelab-wide
   conventions** (network addressing, git workflow, validation procedure,
   tooling defaults).
2. This constitution is authoritative for **Bandolier product architecture
   and security principles** (data model rules, API design, container
   hardening, threat model).
3. If a genuine conflict exists, raise it explicitly with the operator before
   proceeding. Do not silently resolve.

## Development Workflow

**Branch model**: `main` is the integration branch. Feature work lives on
`bandolier-v1-planN` branches (or analogous), commonly developed in a Git
worktree to keep `main` clean. Merge to `main` via merge request after the
GitLab pipeline is green.

**Spec-kit workflow integration**:

1. `/speckit-constitution` (this command) defines and amends these principles.
2. `/speckit-specify` creates feature specifications that MUST cite the
   principles they touch in a "Constitution Check" section.
3. `/speckit-plan` produces implementation plans that MUST pass the
   Constitution Check gate before Phase 0 research.
4. `/speckit-tasks` produces task lists scoped to a plan.
5. `/speckit-analyze` cross-checks artifacts for consistency with this
   constitution.

**Plan execution**: Each plan-N is a vertical slice that produces demoable
software. Plans use the superpowers stack (`feature-dev`,
`subagent-driven-development`, `test-driven-development`) for execution.
`/speckit-implement` is optional and not the default.

**Code review gates**: Every merge to `main` requires both the `code-review`
and `security-review` subagents to pass before merge.

## Governance

**Authority**: This constitution supersedes ad-hoc development practices.
When a development decision conflicts with a principle here, the principle
wins or the constitution is amended — not silently bypassed.

**Amendment procedure**:

1. Proposed amendments are drafted via `/speckit-constitution` invocation
   with the new text.
2. Amendments MUST include a rationale and a Sync Impact Report identifying
   every artifact requiring update.
3. Version bump follows SemVer:
   - **MAJOR**: Backward-incompatible removal or redefinition of a principle.
   - **MINOR**: New principle added, or material expansion of an existing
     principle.
   - **PATCH**: Clarifications, wording fixes, typo corrections.
4. The operator (Ryan) is the sole ratifying authority. No quorum needed;
   no asynchronous review process required.

**Compliance review**: Every `/speckit-plan` invocation produces a
Constitution Check section gated on these principles. Every
`/speckit-analyze` invocation reports principle violations as blocking
findings. A violation that cannot be removed MUST be entered in the plan's
Complexity Tracking table with explicit justification.

**Runtime guidance**: For day-to-day development conduct (commit style,
testing practice, RHEL package management, network topology), refer to
`CLAUDE.md` (project) and `~/.claude/CLAUDE.md` (user-level).

**Version**: 1.0.0 | **Ratified**: 2026-05-25 | **Last Amended**: 2026-05-25
