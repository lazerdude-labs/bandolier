# Contributing to Bandolier

Thanks for your interest. Bandolier is an early-stage open-source project; bug reports, design feedback, and pull requests are all welcome.

## Filing issues

Open an issue on GitHub. Useful information to include:

- What you expected to happen and what actually happened.
- The output of `docker compose ps` and the relevant container logs (`docker logs <container>`).
- Your Proxmox version + the cloud image you tried to deploy.
- Whether you can reproduce on a clean stack (`docker compose down -v && ./deploy/scripts/smoke.sh`).

For security issues, **don't open a public issue**. See [SECURITY.md](SECURITY.md) for the disclosure process.

## Pull requests

1. Fork → feature branch → PR against `main`.
2. Keep PRs scoped to one logical change. Refactors and feature work belong in separate PRs.
3. CI must pass before review. The pipeline is in [`.github/workflows/ci.yml`](.github/workflows/ci.yml) and runs:
   - `terraform fmt -check` + `terraform validate` (enforced)
   - `golangci-lint run ./...` (enforced)
   - `go test ./...` (enforced)
   - `npm run lint` + `npm run typecheck` for the UI (enforced)
4. New code under `api/` ships with tests. New UI components ship with at least a render test.
5. Use [Conventional Commits](https://www.conventionalcommits.org): `feat(scope): summary`, `fix(scope): summary`, `chore: …`, etc. Squash-merge keeps `main` clean.

## Local development

The Compose stack is the canonical dev environment:

```bash
cd deploy
docker compose up -d --build
docker compose logs -f api ui          # follow logs in another terminal
```

Hot-reload during UI work:

```bash
cd ui
npm install
npm run dev          # served on http://127.0.0.1:5173 by default
```

The dev server proxies `/api` to the local Compose stack — see `vite.config.ts`.

## Coding conventions

- **Go:** standard Go style enforced by `gofmt` + `golangci-lint`. Errors wrapped with `fmt.Errorf("...: %w", err)`. Tests live alongside the code (`foo_test.go` next to `foo.go`).
- **TypeScript:** Vite + React 18. Strict mode on. Prefer functional components with hooks. State management via TanStack Query for server state and Zustand for ephemeral UI state.
- **Terraform:** HCL formatted by `terraform fmt`. Variables documented in `terraform/variables.tf`. No defaults that depend on a specific operator's network.
- **Comments explain *why*, not *what*.** Names and types should already explain what.

## Reporting a security issue

See [SECURITY.md](SECURITY.md). Do not file public issues for vulnerabilities.
