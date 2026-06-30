# Deployment — AWS

Syncret runs as a Lambda container function triggered by EventBridge CloudTrail rules. Each use case requires its own function instance and event rule — one function per monitored secret.

---

## Prerequisites

- AWS CLI configured with sufficient permissions
- Docker with `buildx` support
- An ECR repository for the Syncret image

---

## Supported events

| Event | CloudTrail source | Default action |
|---|---|---|
| `RotationSucceeded` | AWS Service Event via CloudTrail | Update target secret → optionally redeploy services |
| `PutSecretValue` | AWS API Call via CloudTrail | Redeploy services → optionally update target secret |
| `RotationFailed` | AWS Service Event via CloudTrail | Log warning, no action |

### Event shapes

**`RotationSucceeded`** — emitted by AWS as a Service Event. `requestParameters` is `null`; the secret ARN lives in `detail.additionalEventData.SecretId` (capital S).

**`PutSecretValue`** — emitted as an API Call via CloudTrail. The secret ARN is in `detail.requestParameters.secretId` (lowercase s).

Syncret resolves the ARN based on `eventName`, not `detail-type`, since `detail-type` differs between the two event shapes.

### ARN validation

After parsing, Syncret rejects any event whose secret ARN does not match `SYNCRET_AWS_SECRET_ARN`. This is the primary guard against the RDS rotation EventBridge rule, which fires on every `RotationSucceeded` in the account regardless of which secret rotated.

---

## Step 1 — Create the IAM execution role

The Lambda function needs a role that allows it to read the source secret, optionally update the target secret, and optionally trigger ECS redeployments.

```bash
aws iam create-role \
  --role-name syncret-execution-role \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": { "Service": "lambda.amazonaws.com" },
      "Action": "sts:AssumeRole"
    }]
  }'

aws iam attach-role-policy \
  --role-name syncret-execution-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
```

Create `syncret-policy.json` with the minimum required permissions — remove sections that don't apply to your use case:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["secretsmanager:GetSecretValue"],
      "Resource": "arn:aws:secretsmanager:<region>:<account>:secret:<source-secret-name>-*"
    },
    {
      "Effect": "Allow",
      "Action": ["secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue"],
      "Resource": "arn:aws:secretsmanager:<region>:<account>:secret:<target-secret-name>-*"
    },
    {
      "Effect": "Allow",
      "Action": ["ecs:UpdateService", "ecs:DescribeServices"],
      "Resource": "*"
    }
  ]
}
```

```bash
aws iam put-role-policy \
  --role-name syncret-execution-role \
  --policy-name syncret-policy \
  --policy-document file://syncret-policy.json
```

---

## Step 2 — Push the image to ECR

Lambda requires images to be stored in ECR — it cannot pull from Docker Hub or other public registries directly. The public Syncret image is multi-arch (`linux/amd64` and `linux/arm64`); Lambda requires a single-arch image, so you must specify the platform when pulling.

ARM64 (Graviton) is recommended for lower cost and better performance. The architecture you push must match the `--architectures` value in Step 3.

First, log in to ECR:

```bash
ECR=123456789012.dkr.ecr.us-east-1.amazonaws.com/syncret
VERSION=latest

aws ecr get-login-password --region us-east-1 \
  | docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com
```

### Option A — Use the pre-built image from Docker Hub

```bash
# ARM64 (recommended)
docker pull --platform linux/arm64 fayrus/syncret:$VERSION
docker tag fayrus/syncret:$VERSION $ECR:$VERSION
docker push $ECR:$VERSION

# x86_64
docker pull --platform linux/amd64 fayrus/syncret:$VERSION
docker tag fayrus/syncret:$VERSION $ECR:$VERSION
docker push $ECR:$VERSION
```

### Option B — Build from source

```bash
docker buildx build \
  --platform linux/arm64 \
  --provenance=false \
  --build-arg VERSION=$VERSION \
  -t $ECR:$VERSION \
  --push .
```

Replace `linux/arm64` with `linux/amd64` to target x86_64.

---

## Step 3 — Create the Lambda function

```bash
aws lambda create-function \
  --function-name syncret-rds \
  --package-type Image \
  --code ImageUri=$ECR:$VERSION \
  --architectures arm64 \
  --role arn:aws:iam::123456789012:role/syncret-execution-role \
  --timeout 60 \
  --region us-east-1 \
  --environment 'Variables={
    SYNCRET_PROVIDER=aws,
    SYNCRET_AWS_SECRET_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-00000000-0000-0000-0000-000000000000-AbCdEf,
    SYNCRET_AWS_REGION=us-east-1,
    SYNCRET_AWS_TARGET_SECRET_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:my-app-secret-XyZaBc,
    SYNCRET_TARGET_SECRET_KEYS=password,
    SYNCRET_AWS_ECS_FORCE_DEPLOY=true,
    SYNCRET_AWS_ECS_CLUSTER=my-cluster,
    SYNCRET_AWS_ECS_SERVICES=backend,
    SYNCRET_INSTANCE_NAME=Production,
    SYNCRET_TIMEZONE=America/Lima
  }'
```

---

## Step 4 — Create the EventBridge rule

Each use case uses a different CloudTrail event shape and requires its own rule.

### Database rotation rule

Triggers on `RotationSucceeded`. The ARN filter is applied inside Syncret via `SYNCRET_AWS_SECRET_ARN` since `requestParameters` is null for this event.

```bash
aws events put-rule \
  --name syncret-rds-rotation \
  --event-pattern '{
    "source": ["aws.secretsmanager"],
    "detail-type": ["AWS Service Event via CloudTrail"],
    "detail": {
      "eventSource": ["secretsmanager.amazonaws.com"],
      "eventName": ["RotationSucceeded"]
    }
  }' \
  --region us-east-1

aws lambda add-permission \
  --function-name syncret-rds \
  --statement-id syncret-rds-eventbridge \
  --action lambda:InvokeFunction \
  --principal events.amazonaws.com \
  --source-arn arn:aws:events:us-east-1:123456789012:rule/syncret-rds-rotation

aws events put-targets \
  --rule syncret-rds-rotation \
  --targets 'Id=syncret-rds,Arn=arn:aws:lambda:us-east-1:123456789012:function:syncret-rds' \
  --region us-east-1
```

### App settings rule

Triggers on `PutSecretValue`. The rule can filter by ARN prefix since `requestParameters.secretId` is available for this event.

```bash
aws events put-rule \
  --name syncret-app-settings \
  --event-pattern '{
    "source": ["aws.secretsmanager"],
    "detail-type": ["AWS API Call via CloudTrail"],
    "detail": {
      "eventSource": ["secretsmanager.amazonaws.com"],
      "eventName": ["PutSecretValue"],
      "requestParameters": {
        "secretId": [{"prefix": "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-app-settings"}]
      }
    }
  }' \
  --region us-east-1

aws lambda add-permission \
  --function-name syncret-app \
  --statement-id syncret-app-eventbridge \
  --action lambda:InvokeFunction \
  --principal events.amazonaws.com \
  --source-arn arn:aws:events:us-east-1:123456789012:rule/syncret-app-settings

aws events put-targets \
  --rule syncret-app-settings \
  --targets 'Id=syncret-app,Arn=arn:aws:lambda:us-east-1:123456789012:function:syncret-app' \
  --region us-east-1
```

---

## ECS task definition

When service redeployment is enabled, Syncret only triggers new task launches — it does not inject secrets into containers. Your ECS task definition must already reference the target secret so ECS fetches the latest value at container startup:

```json
{
  "containerDefinitions": [{
    "secrets": [{
      "name": "DB_PASSWORD",
      "valueFrom": "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-app-secret-XyZaBc:password::"
    }]
  }]
}
```

---

## Configuration examples

### Database rotation

Copies the rotated database password into an application secret and restarts services.

```bash
SYNCRET_PROVIDER=aws
SYNCRET_AWS_SECRET_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-0a0aa000-0a00-0a00-aaa0-0aa0a000a00a-AbdosSu
SYNCRET_AWS_REGION=us-east-1
SYNCRET_AWS_TARGET_SECRET_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:my-app-secret-XyZaBc
SYNCRET_TARGET_SECRET_KEYS=password
SYNCRET_AWS_ECS_FORCE_DEPLOY=true
SYNCRET_AWS_ECS_CLUSTER=my-cluster
SYNCRET_AWS_ECS_SERVICES=backend
SYNCRET_INSTANCE_NAME="Production"
SYNCRET_TIMEZONE=America/Lima
SYNCRET_LOG_LEVEL=info
SYNCRET_LOG_FORMAT=json
```

### App settings

Restarts services when an application secret changes. No target secret update needed — the app reads the source secret directly.

```bash
SYNCRET_PROVIDER=aws
SYNCRET_AWS_SECRET_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:my-app-settings-XyZaBc
SYNCRET_AWS_REGION=us-east-1
SYNCRET_AWS_ECS_FORCE_DEPLOY=true
SYNCRET_AWS_ECS_CLUSTER=my-cluster
SYNCRET_AWS_ECS_SERVICES=app
SYNCRET_LOG_LEVEL=info
SYNCRET_LOG_FORMAT=json
```

---

## Verify

Invoke the Lambda directly and check CloudWatch Logs:

```bash
aws lambda invoke \
  --function-name syncret-rds \
  --region us-east-1 \
  --payload '{}' \
  response.json && cat response.json
```

For a more realistic test, trigger a manual rotation with `aws secretsmanager rotate-secret` and confirm the Lambda executes via CloudWatch Logs.
