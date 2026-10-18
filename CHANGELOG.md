# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.4.4] - 2026-10-18

### Fixed

- **Docs dependencies upgraded** in `docs/requirements.txt` — `urllib3` 2.8.0 resolves three Dependabot alerts (unbounded chunk-size buffering and ignored HTTPS proxy TLS configuration, both high; infinite loop in chunked deflate streaming, medium) and `mkdocs-material` 9.7.7 resolves a DOM XSS in search suggestions (medium). Docs-only dependencies; no runtime impact.
- **Published images now report their real version** — the publish workflow passes `VERSION` as a Docker build arg; previously images were built with the default `dev`, so the startup log showed `version=dev`.
- **`docker/setup-qemu-action` upgraded to v4.4.0** in the publish workflow — removes the spurious `Failed to save: Unable to reserve cache` warning caused by a minification bug in v4.1.0 ([docker/setup-qemu-action#306](https://github.com/docker/setup-qemu-action/issues/306)).

### Changed

- **GitHub Actions upgraded** (SHA-pinned) — `actions/checkout` v7.0.1, `actions/setup-go` v7, `actions/setup-python` v7, `docker/build-push-action` v7.4.0, `docker/login-action` v4.6.0, `docker/metadata-action` v6.2.0, `docker/setup-buildx-action` v4.4.1, `github/codeql-action/upload-sarif` v4.39.0, `golangci/golangci-lint-action` v9.3.0, `SonarSource/sonarqube-scan-action` v8.3.0, `SonarSource/sonarqube-quality-gate-action` v1.2.1.
- **CI runners pinned to `ubuntu-24.04`** instead of `ubuntu-latest`, so runner image upgrades (e.g., Ubuntu 26.04) are adopted deliberately.
- **golangci-lint pinned to v2.15.0** instead of `latest`, so new linter releases cannot break CI unexpectedly.
- **Workflow, job, and step names refreshed with icons**; the `CI` workflow is now `🧪 PR Checks`. Jobs used as required status checks (`Lint (Actions)`, `Tests (Go)`, `Lint (Go)`) keep their names.

## [0.4.3] - 2026-10-18

### Fixed

- **Go upgraded to 1.27.2** — resolves 8 reachable stdlib vulnerabilities reported by `govulncheck` in `net/http`, `net/http/internal/http2`, `net/textproto`, and `crypto/tls` (GO-2026-6603, GO-2026-6605, GO-2026-6607, GO-2026-6608, GO-2026-6610, GO-2026-6611, GO-2026-6613, GO-2026-6617). Builder image pinned to `golang:1.27.2-alpine` so cached or stale `1.27` tags cannot produce a binary built with an older toolchain.

### Changed

- **Go module dependencies upgraded** — `aws-lambda-go` 1.55.1, `aws-sdk-go-v2` 1.47.3, `config` 1.33.9, `service/ecs` 1.100.3, `service/secretsmanager` 1.50.4, plus related indirect AWS SDK and `smithy-go` modules.

## [0.4.2] - 2026-08-23

### Fixed

- **`pymdown-extensions` upgraded to 11.0.2** in `docs/requirements.txt` — resolves two Dependabot alerts: a ReDoS in inline processors (high) and a path-traversal in the `b64` extension (medium). Docs-only dependency; no runtime impact.

## [0.4.1] - 2026-08-23

### Fixed

- **Go upgraded to 1.27.0** — resolves stdlib CVEs flagged by Docker Hub image scanning; base builder image bumped to `golang:1.27-alpine` to match the Chainguard toolchain version.

## [0.4.0] - 2026-06-30

### Added

- **`SYNCRET_INSTANCE_NAME`** — optional human-readable label for the deployment, shown in notification headers (e.g., `*Syncret — Production*`). Must be a single line of at most 64 characters.
- **`SYNCRET_TIMEZONE`** — optional IANA timezone for notification timestamps (e.g., `America/Lima`). Defaults to `UTC`. Invalid zones fail at startup. Timezone data is embedded in the binary via `time/tzdata` for Lambda environments.
- **Enriched GChat notifications** — messages now include AWS account ID, region, source and target secret names (extracted from ARNs), ECS cluster and services, Lambda request ID, and a localized timestamp. Each pipeline stage (target secret update, ECS force-deployment) is reported individually with its outcome (succeeded, failed, skipped, or not attempted). Errors shown in notifications are sanitized; raw details remain in CloudWatch Logs.

## [0.3.0] - 2026-06-19

### Added

- **Google Chat notifications** — optional `SYNCRET_GCHAT_WEBHOOK` variable. When set, Syncret sends a notification after every execution (success and failure) with the event type, secret ARN, actions taken, status, and error message on failure. Notification failures are logged as warnings and do not affect the Lambda return value.

## [0.2.0] - 2026-06-19

### Added

- **`SYNCRET_PROVIDER`** — required variable that selects the cloud provider. Currently only `aws` is supported; unsupported values fail at startup.

### Changed

- **Breaking: environment variable renames** — all provider-specific variables now carry an explicit cloud prefix to clearly separate provider namespaces:
  - `SYNCRET_SECRET_ARN` → `SYNCRET_AWS_SECRET_ARN`
  - `SYNCRET_TARGET_SECRET_ARN` → `SYNCRET_AWS_TARGET_SECRET_ARN`
  - `SYNCRET_ECS_FORCE_DEPLOY` → `SYNCRET_AWS_ECS_FORCE_DEPLOY`
  - `SYNCRET_ECS_CLUSTER` → `SYNCRET_AWS_ECS_CLUSTER`
  - `SYNCRET_ECS_SERVICES` → `SYNCRET_AWS_ECS_SERVICES`
  - `SYNCRET_TARGET_SECRET_KEYS`, `SYNCRET_AWS_REGION`, `SYNCRET_LOG_LEVEL`, `SYNCRET_LOG_FORMAT` — unchanged
- **Documentation restructure** — docs are now cloud-agnostic at the top level; provider-specific content lives under `Deployment — AWS`; `Configuration` page renamed to `Reference` with sections per provider

## [0.1.0] - 2026-06-15

### Added

- **Lambda handler** triggered by EventBridge CloudTrail rules scoped to `aws.secretsmanager` events
- **Event parsing** for three event names, with ARN extraction per event type:
  - `RotationSucceeded` — ARN read from `detail.additionalEventData.SecretId`
  - `PutSecretValue` — ARN read from `detail.requestParameters.secretId`
  - `RotationFailed` — logs a warning and exits cleanly; no AWS calls are made
- **Event source guard** — rejects payloads with an unexpected `source` or `eventSource` before any AWS API call
- **ARN guard** — rejects events whose secret ARN does not match `SYNCRET_SECRET_ARN`
- **Target secret update** — reads the source secret and merges selected fields into a target secret (`SYNCRET_TARGET_SECRET_ARN`, `SYNCRET_TARGET_SECRET_KEYS`); key specs support `src:dst` remapping (e.g., `password:DB_PASS`); optional when ECS force deployment is configured
- **Idempotent secret writes** — `ClientRequestToken` is a UUID derived via SHA-256 from the target secret's current `VersionId`, so concurrent invocations processing the same rotation event produce the same token and the second write is a no-op
- **ECS force-new-deployment** — calls `UpdateService` with `ForceNewDeployment=true` for each service in `SYNCRET_ECS_SERVICES`; `DescribeServices` is batched in groups of 10 to respect the AWS API limit; inactive services are skipped with a warning; services not found in the cluster return an error; optional when target secret update is configured
- **Startup validation** — fails fast if `SYNCRET_SECRET_ARN` or `SYNCRET_AWS_REGION` are missing, if `SYNCRET_TARGET_SECRET_ARN` is set without `SYNCRET_TARGET_SECRET_KEYS`, or if neither flow is configured
- **Structured logging** via `log/slog`; `request_id` (Lambda `AwsRequestID`) is attached to every log line; level and format are configurable via `SYNCRET_LOG_LEVEL` (debug/info/warn/error, default: info) and `SYNCRET_LOG_FORMAT` (json/text, default: json); invalid values fail at startup
- **Chainguard static base image** — `cgr.dev/chainguard/static`; no shell, no package manager, minimal attack surface
- **Multi-arch container** — supports `linux/amd64` and `linux/arm64`; ARM64 (Graviton) preferred for lower cost and better performance-per-watt
- **Build-time version embedding** — version string injected via `-ldflags` and logged at startup

[Unreleased]: https://github.com/fayrus/syncret/compare/v0.4.2...HEAD
[0.4.2]: https://github.com/fayrus/syncret/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/fayrus/syncret/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/fayrus/syncret/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/fayrus/syncret/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/fayrus/syncret/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/fayrus/syncret/releases/tag/v0.1.0
