# Git Hooks

Install hooks for this repository:

```bash
bash scripts/install-git-hooks.sh
```

## Pre-commit checks

- `gofmt` on staged Go files
- quick Go tests via `scripts/test-quick.sh`
- full Go tests via `scripts/test-all.sh`
