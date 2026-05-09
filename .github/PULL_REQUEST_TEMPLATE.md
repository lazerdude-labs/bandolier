## Summary

<!-- One paragraph: what does this PR change and why? -->

## Type of change

<!-- Check all that apply -->

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing behavior to change)
- [ ] Documentation only
- [ ] CI / tooling
- [ ] Refactor (no functional change)

## Test plan

<!-- How did you verify this works? CI alone is rarely sufficient for feature work. -->

- [ ] Existing tests pass (`go test ./... && cd ui && npm run lint && npm run build`)
- [ ] Added test coverage for the change (link to tests below if applicable)
- [ ] Manually exercised the affected flow (describe what you did)

## Linked issues

<!-- "Closes #N" auto-closes the issue on merge. "Refs #N" leaves it open. -->

## Notes for the reviewer

<!-- Anything subtle the reviewer should know? Tradeoffs, alternatives considered, follow-ups deferred? -->

## Checklist

- [ ] Conventional Commit style on the title (`type(scope): summary`)
- [ ] Updated `CHANGELOG.md` under `[Unreleased]` if user-visible
- [ ] No secrets, no internal hostnames, no real IPs in the diff
- [ ] If this touches `deploy/`, `terraform/`, or `ansible/`, the smoke test still runs
