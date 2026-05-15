# Platform Prod Deploy Runbook

This runbook standardizes Platform backend production deploys and runtime smoke checks.

## Safety defaults

- The deploy flow preserves remote `/root/gk/platform-backend/config.prod.yaml` by default.
- Repository `config.prod.yaml` may contain placeholders; it is uploaded only when `UPLOAD_PROD_CONFIG=1` or `--upload-prod-config` is explicitly set and placeholder scan passes.
- Scripts must not print secrets, provider keys, JWTs, DB passwords, or internal service secrets. Secret comparisons use booleans/hash prefixes only.
- `/healthz` is not sufficient for runtime-capable deploys. Run drift and runtime smoke.

## Topology manifest

Non-secret prod topology lives in:

```text
ops/topology/prod.env
```

It defines prod/dev container names, expected ports, Docker network, provider expectations, and storage binding expectations.

## Common commands

Read-only dry run:

```bash
./tools/prod/platform-drift-check.sh --env prod --dry-run --fail-on-critical
./tools/prod/platform-runtime-smoke.sh --env prod --dry-run --auto-route
./tools/prod/platform-deploy.sh all --env prod --dry-run
```

Read-only prod drift gate:

```bash
./tools/prod/platform-drift-check.sh --env prod --fail-on-critical
```

Callback secret alignment / rotation:

```bash
# Plan only; no mutation
./tools/prod/platform-callback-secret-sync.sh --env prod

# Rotate Ecommerce inbound callback secret and align Platform endpoint secret
./tools/prod/platform-callback-secret-sync.sh --env prod --mode rotate --apply --restart-ecommerce

# Or, if Ecommerce already has the correct real secret, copy it into the Platform endpoint
./tools/prod/platform-callback-secret-sync.sh --env prod --mode sync-existing --apply --restart-ecommerce
```

Runtime smoke:

```bash
./tools/prod/platform-runtime-smoke.sh --env prod --explicit-provider kimi_coding_text
./tools/prod/platform-runtime-smoke.sh --env prod --auto-route
```

Full deploy after merge:

```bash
git checkout main
git pull --ff-only origin main
./tools/prod/platform-deploy.sh all --env prod
```

Legacy wrapper remains available:

```bash
./build.sh prod
```

## Phased/resumable deploy

```bash
./tools/prod/platform-deploy.sh build --env prod
./tools/prod/platform-deploy.sh upload --env prod
./tools/prod/platform-deploy.sh restart --env prod
./tools/prod/platform-deploy.sh drift-check --env prod
./tools/prod/platform-deploy.sh smoke --env prod
./tools/prod/platform-deploy.sh evidence --env prod
```

## Build behavior

`platform-deploy.sh build` first attempts the normal multi-stage Docker build. If Docker build fails due network/APK/module fetch, it automatically falls back to:

1. local `GOOS=linux GOARCH=amd64` Go binary build;
2. `Dockerfile.local-binary` runtime image;
3. `docker save | gzip` artifact.

## Drift layers

`platform-drift-check.sh` checks:

- prod containers exist and report image/health;
- `bootstrap.sync_enabled=true` and `auto_migrate_enabled=false`;
- Kimi provider config/key presence;
- runtime product endpoints do not point to dev containers;
- endpoint secrets are non-empty and non-placeholder;
- Ecommerce callback endpoint secret hash matches Ecommerce `security.service_secret_key`;
- provider definitions/bindings exist for text runtime;
- storage bindings exist for expected output categories.

## Smoke layers

`platform-runtime-smoke.sh` creates a synthetic `regression_smoke` text runtime job, polls until terminal, and asserts:

- `status=completed`;
- provider resolved correctly;
- output manifest is present.

Use explicit Kimi smoke first, then auto-route smoke.

## Callback secret rotation

`platform-callback-secret-sync.sh` is the safe remediation when drift check reports `endpoint product=ecommerce endpoint_secret_placeholder=true`, `ecommerce callback_secret_placeholder=true`, or `callback_secret_hash_matches_backend=false`. It creates a remote config backup, rotates or syncs Ecommerce inbound `security.service_secret_key`, updates `runtime_product_endpoints.ecommerce.secret`, restores Ecommerce outbound `platform.internal_service_secret` from Platform `security.internal_service_secret`, optionally restarts Ecommerce, and prints only booleans/hash prefixes.

Follow-up gate:

```bash
./tools/prod/platform-drift-check.sh --env prod --fail-on-critical
```

## Known warnings

- Minimax may be configured and bound but still unavailable due account/quota limits. Treat as WARN unless default routing depends on it.
- Dev containers may run next to prod containers; this is allowed only if prod DB endpoints point to prod container URLs.
