# Configuration Reference

All configuration is via environment variables. Syncret validates all required variables at startup and exits immediately with a clear error message if any are missing or invalid.

At least one action must be configured: target secret update, ECS force deployment, or both.

---

## Core (required)

| Variable | Description |
|---|---|
| `SYNCRET_SECRET_ARN` | Full ARN of the source secret to monitor. Must match the ARN in the EventBridge event — mismatches are rejected. |
| `SYNCRET_AWS_REGION` | AWS region where all resources live. |

---

## Target secret update

Use this when another service needs the rotated credential under a different field name, or merged into an existing secret — for example, copying `password` from an RDS-managed secret into a Metabase config secret as `MB_DB_PASS`.

Syncret reads the source secret, copies the specified fields into the target secret, and writes it back. If ECS force deployment is also enabled, the target secret is updated first so restarting tasks pick up the latest values.

| Variable | Description |
|---|---|
| `SYNCRET_TARGET_SECRET_ARN` | ARN of the secret to update with selected fields from the source secret. Optional — if omitted, `SYNCRET_ECS_FORCE_DEPLOY=true` must be set. |
| `SYNCRET_TARGET_SECRET_KEYS` | Comma-separated list of fields to copy. Required when `SYNCRET_TARGET_SECRET_ARN` is set. Use `key` when source and target share the same field name (`password`), or `source_key:target_key` when they differ (`password:MB_DB_PASS`). |

**Examples:**

```bash
# Source and target use the same field name
SYNCRET_TARGET_SECRET_KEYS=password

# Source and target use different field names (e.g. Metabase)
SYNCRET_TARGET_SECRET_KEYS=password:MB_DB_PASS

# Copy multiple fields
SYNCRET_TARGET_SECRET_KEYS=username,password
```

---

## ECS force deployment

Use this when ECS services must restart after a secret changes to pick up the new values.

| Variable | Default | Description |
|---|---|---|
| `SYNCRET_ECS_FORCE_DEPLOY` | `false` | Enable ECS force-new-deployment. Optional — if omitted or `false`, `SYNCRET_TARGET_SECRET_ARN` must be set. |
| `SYNCRET_ECS_CLUSTER` | — | Name of the ECS cluster where the services run. Required when `SYNCRET_ECS_FORCE_DEPLOY=true`. |
| `SYNCRET_ECS_SERVICES` | — | Comma-separated list of ECS service names to redeploy. Required when `SYNCRET_ECS_FORCE_DEPLOY=true`. Spaces around names are trimmed. |

---

## Observability

| Variable | Default | Options | Description |
|---|---|---|---|
| `SYNCRET_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` | Log verbosity. Invalid values fail at startup. |
| `SYNCRET_LOG_FORMAT` | `json` | `json`, `text` | `json` for production (CloudWatch Logs). `text` for local development. Invalid values fail at startup. |

---

## Examples

### RDS rotation Lambda

Copies the rotated database password into an application secret and restarts ECS services.

```bash
SYNCRET_SECRET_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-0a0aa000-0a00-0a00-aaa0-0aa0a000a00a-AbdosSu
SYNCRET_AWS_REGION=us-east-1
SYNCRET_TARGET_SECRET_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:my-app-secret-XyZaBc
SYNCRET_TARGET_SECRET_KEYS=password
SYNCRET_ECS_FORCE_DEPLOY=true
SYNCRET_ECS_CLUSTER=my-cluster
SYNCRET_ECS_SERVICES=backend,worker
SYNCRET_LOG_LEVEL=info
SYNCRET_LOG_FORMAT=json
```

### App settings Lambda

Restarts ECS services when an application secret changes. No target secret update needed — the app reads the source secret directly.

```bash
SYNCRET_SECRET_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:my-app-settings-XyZaBc
SYNCRET_AWS_REGION=us-east-1
SYNCRET_ECS_FORCE_DEPLOY=true
SYNCRET_ECS_CLUSTER=my-cluster
SYNCRET_ECS_SERVICES=app
SYNCRET_LOG_LEVEL=info
SYNCRET_LOG_FORMAT=json
```
