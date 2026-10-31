# Reference

All configuration is via environment variables. Syncret validates all required variables at startup and exits immediately with a clear error message if any are missing or invalid.

At least one action must be configured: target secret update, service redeployment, or both.

---

## Core

These variables apply regardless of cloud provider.

| Variable | Required | Default | Description |
|---|---|---|---|
| `SYNCRET_PROVIDER` | Yes | — | Cloud provider to use. Supported values: `aws`. |
| `SYNCRET_TARGET_SECRET_KEYS` | No* | — | Comma-separated list of fields to copy from source to target secret. Required when `SYNCRET_AWS_TARGET_SECRET_ARN` is set. Use `key` to copy under the same name, or `source_key:target_key` to remap. |
| `SYNCRET_LOG_LEVEL` | No | `info` | Log verbosity: `debug`, `info`, `warn`, `error`. Invalid values fail at startup. |
| `SYNCRET_LOG_FORMAT` | No | `json` | Log format: `json` for production, `text` for local development. Invalid values fail at startup. |
| `SYNCRET_INSTANCE_NAME` | No | — | Human-readable name for this Syncret deployment, shown in notifications. Must be a single line of at most 64 characters. |
| `SYNCRET_TIMEZONE` | No | `UTC` | IANA timezone used in notifications, such as `America/Lima` or `America/New_York`. Invalid zones fail at startup. |

**`SYNCRET_TARGET_SECRET_KEYS` examples:**

```bash
# Same field name in source and target
SYNCRET_TARGET_SECRET_KEYS=password

# Remap to a different field name
SYNCRET_TARGET_SECRET_KEYS=password:MB_DB_PASS

# Copy multiple fields
SYNCRET_TARGET_SECRET_KEYS=username,password
```

---

## AWS

| Variable | Required | Default | Description |
|---|---|---|---|
| `SYNCRET_AWS_SECRET_ARN` | Yes | — | ARN of the source secret to monitor. Must match the secret ARN in the incoming event — mismatches are rejected. |
| `SYNCRET_AWS_REGION` | Yes | — | AWS region where all resources live. |
| `SYNCRET_AWS_TARGET_SECRET_ARN` | No* | — | ARN of the secret to update with selected fields from the source secret. Must refer to a different secret than `SYNCRET_AWS_SECRET_ARN` (also checked against the source's partial ARN and name); otherwise Syncret refuses to start, since writing to the source would re-trigger itself. |
| `SYNCRET_AWS_ECS_FORCE_DEPLOY` | No* | `false` | Enable ECS force-new-deployment. |
| `SYNCRET_AWS_ECS_CLUSTER` | If ECS enabled | — | Name of the ECS cluster. |
| `SYNCRET_AWS_ECS_SERVICES` | If ECS enabled | — | Comma-separated list of ECS service names to redeploy. Spaces around names are trimmed. |

\* At least one of `SYNCRET_AWS_TARGET_SECRET_ARN` or `SYNCRET_AWS_ECS_FORCE_DEPLOY=true` must be set.

→ See [Deployment — AWS](deployment/aws.md) for step-by-step setup and full configuration examples.

---

## Google Chat

| Variable | Required | Default | Description |
|---|---|---|---|
| `SYNCRET_GCHAT_WEBHOOK` | No | — | Incoming webhook URL. When set, Syncret sends a notification after every execution — success and failure. |

**Message format:**

```
*Syncret — Production*

*Event:* RotationSucceeded
*Account:* 123456789012
*Region:* us-east-1
*Source secret:* source-secret
*Target secret:* target-secret
*ECS cluster:* production
*ECS services:* backend
*Date:* 2026-06-20 13:06:17 -05:00 (America/Lima)
*Request ID:* 00000000-0000-0000-0000-000000000000
*Actions:*
  • ✅ Target secret updated
  • ✅ ECS force-deployment request accepted
*Status:*  ✅ Success
```

Optional resource fields are omitted when they are not configured. `ECS force-deployment request accepted` means ECS accepted `UpdateService` with `ForceNewDeployment=true`; Syncret does not wait for the deployment to stabilize.

On failure, each action is reported as failed, skipped, or not attempted. The notification contains a sanitized error and the Lambda request ID; technical details remain in CloudWatch Logs. If the notification itself fails, Syncret logs a warning and preserves the Lambda's original result.
