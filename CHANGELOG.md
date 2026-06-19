# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/fayrus/syncret/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/fayrus/syncret/releases/tag/v0.1.0
