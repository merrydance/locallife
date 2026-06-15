# Release Readiness Target Evidence Template

Date: 2026-06-15
Risk theme: release configuration
Risk class: G3 - scheduler and worker convergence for payment, refund, withdrawal, order, checkout, and recovery flows
Status: template only; no target run recorded in this file

## Purpose

Use this template only for the target-environment run of:

```bash
cd locallife
ENVIRONMENT=production PAYMENT_FACT_APPLICATION_FIXTURE_ID=<id> PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID=<id> PATH="/usr/local/go/bin:$PATH" scripts/release_readiness_smoke.sh --target --format text
```

The fixture IDs must be disposable release-smoke rows prepared by the release
operator. Do not use static, dry-run, staging, or local output as production
release readiness evidence.

Validate a filled evidence file with:

```bash
cd locallife
make check-release-readiness-target-evidence evidence=../artifacts/production-risk-audit/flows/release-readiness-target-evidence-YYYY-MM-DD.md
```

## Target Environment

Target environment: record production.
Backend commit: record the deployed commit SHA.
Release operator: record the operator or release owner.

## Release Smoke Command

Smoke command: record the exact `scripts/release_readiness_smoke.sh --target --format text` command, including `ENVIRONMENT=production`, both fixture IDs, and the Go `PATH`.
Smoke output artifact: record the masked output path, ticket attachment, or evidence id.
Target smoke status: record pass only if the target command exited 0.

## Fixture Rows

PAYMENT_FACT_APPLICATION_FIXTURE_ID: record the positive integer fixture id.
PAYMENT_DOMAIN_OUTBOX_FIXTURE_ID: record the positive integer fixture id.
Fixture source: record how the disposable fixture rows were prepared and why they are safe to claim in rollback-only mode.

## Readiness Results

config:production_allowed_origins: fail
config:production_redis_address: fail
config:production_data_encryption_key: fail
config:production_payment_runtime: fail
redis:connection: fail
asynq:queue:critical: fail
asynq:queue:default: fail
provider:baofu:root: fail
provider:baofu:aggregate: fail
provider:baofu:account: fail
provider:baofu:merchant_report: fail
fixture:payment_fact_application: fail
fixture:payment_domain_outbox: fail
scheduler:dine-in-checkout-recovery: fail
worker:payment:process_fact_application: fail
worker:payment:process_domain_outbox: fail

## Alert Evidence

Dine-in recovery alert evidence: record the filled target-environment alert evidence file path or evidence id. If this is a local `.md` file, it must pass `npm run check:dine-in-recovery-alert-evidence -- <file>` and must not be this template or an alert template.

## Result

Verdict: fail

If the verdict is `pass`, this file must be a filled evidence file, not this
template. Every readiness row above must be `pass`, the smoke command must use
`--target` without `--dry-run`, and the environment must be production.
