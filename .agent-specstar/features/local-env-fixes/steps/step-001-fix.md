# Fix-Step for step-001: Commit deliverable and revert forbidden changes

## Metadata
- Feature: local-env-fixes
- Fixes: step-001
- Status: pending
- Last Updated: 2026-04-19

## Objective

Resolve the two violations found during step-001 review so the step can be re-reviewed and approved.

## Required Fixes

### Fix 1: Commit `.golangci.yml`

The file exists in the working tree but is untracked. Stage and commit it as the sole deliverable of step-001.

```bash
git add .golangci.yml
git commit -m "feat(local-env-fixes): add repository-owned .golangci.yml"
```

### Fix 2: Revert `.github/agents/AGENTS.md` changes

The working tree contains substantial modifications to `.github/agents/AGENTS.md` (325 lines of diff). This file is a **forbidden path** for step-001. AGENTS.md verification and correction belongs to step-004.

Restore `AGENTS.md` to its committed state:

```bash
git checkout -- .github/agents/AGENTS.md
```

If any of these AGENTS.md changes are genuinely needed, they must be applied during step-004 execution, not step-001.

## Allowed Write Paths

- `.golangci.yml` (commit only — no content changes needed)

## Forbidden Paths

- Everything except `.golangci.yml` and review/fix-step workflow artifacts.

## Verification After Fix

1. `git status --short` shows no modified or untracked production files.
2. `.golangci.yml` appears in the git log as a committed file.
3. `.github/agents/AGENTS.md` matches its last committed state.
4. `golangci-lint run ./...` still exits 0 (re-run to confirm).

## Scope

Narrow. No content changes to `.golangci.yml`. Only version-control hygiene and scope restoration.
