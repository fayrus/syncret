# Deployment Guide

Each use case requires its own function instance and event trigger. Deploy one instance per use case, with different environment variables and event rules for each.

---

## AWS

Syncret runs as a Lambda function triggered by EventBridge CloudTrail rules.

### Prerequisites

- AWS CLI configured with sufficient permissions
- Docker with `buildx` support
- An ECR repository for the Syncret image

### Step 1 — Create the IAM execution role

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

### Step 2 — Build and push the image to ECR

Lambda does not support OCI manifest lists, so push a single-arch image. ARM64 (Graviton) is preferred for cost and performance.

```bash
ECR=123456789012.dkr.ecr.us-east-1.amazonaws.com/syncret
VERSION=$(cat VERSION)

aws ecr get-login-password --region us-east-1 \
  | docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com

docker buildx build \
  --platform linux/arm64 \
  --provenance=false \
  --build-arg VERSION=$VERSION \
  -t $ECR:$VERSION \
  --push .
```

### Step 3 — Create the Lambda function

```bash
aws lambda create-function \
  --function-name syncret-rds \
  --package-type Image \
  --code ImageUri=123456789012.dkr.ecr.us-east-1.amazonaws.com/syncret:v0.1.0 \
  --architectures arm64 \
  --role arn:aws:iam::123456789012:role/syncret-execution-role \
  --timeout 60 \
  --region us-east-1 \
  --environment 'Variables={
    SYNCRET_SECRET_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-00000000-0000-0000-0000-000000000000-AbCdEf,
    SYNCRET_AWS_REGION=us-east-1,
    SYNCRET_TARGET_SECRET_ARN=arn:aws:secretsmanager:us-east-1:123456789012:secret:my-app-secret-XyZaBc,
    SYNCRET_TARGET_SECRET_KEYS=password,
    SYNCRET_ECS_FORCE_DEPLOY=true,
    SYNCRET_ECS_CLUSTER=my-cluster,
    SYNCRET_ECS_SERVICES=backend,worker
  }'
```

### Step 4 — Create the EventBridge rule

Each use case uses a different CloudTrail event shape and requires its own rule.

#### RDS rotation rule

Triggers on `RotationSucceeded`. This event arrives as an AWS Service Event — `requestParameters` is `null`, so the ARN filter is applied inside Syncret via `SYNCRET_SECRET_ARN`.

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

#### App settings rule

Triggers on `PutSecretValue`. This event arrives as an API Call via CloudTrail and includes `requestParameters.secretId`, so the rule can filter by ARN prefix to narrow it to a specific secret.

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

### Verify

Invoke the Lambda directly and check CloudWatch Logs:

```bash
aws lambda invoke \
  --function-name syncret-rds \
  --region us-east-1 \
  --payload '{}' \
  response.json && cat response.json
```

For a more realistic test, trigger a manual rotation with `aws secretsmanager rotate-secret` and confirm the Lambda executes via CloudWatch Logs.

---

## Azure

Coming soon. Planned implementation: Azure Functions + Event Grid + Key Vault + Container Apps.

---

## GCP

Coming soon. Planned implementation: Cloud Functions + Pub/Sub + Secret Manager + Cloud Run.
