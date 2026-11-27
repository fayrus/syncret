# Syncret

Syncret reacts to AWS Secrets Manager rotation and change events. It copies selected fields into a dependent secret, forces ECS services to redeploy with the new values, or both, and can notify Google Chat or Slack after every run. It runs as an AWS Lambda container function triggered by EventBridge.

## Quick reference

- **Documentation:** [syncret.fayr.us](https://syncret.fayr.us)
- **Source and issues:** [github.com/fayrus/syncret](https://github.com/fayrus/syncret)
- **Changelog:** [CHANGELOG.md](https://github.com/fayrus/syncret/blob/main/CHANGELOG.md)
- **Architectures:** `linux/arm64` (recommended for Lambda), `linux/amd64`

## Use with AWS Lambda

Lambda cannot pull images from Docker Hub, and it needs a single-architecture image. Copy the image to your ECR repository for the architecture your function uses:

```sh
ECR=123456789012.dkr.ecr.us-east-1.amazonaws.com/syncret
VERSION=v0.5.0   # pick a release from the Tags page

aws ecr get-login-password --region us-east-1 \
  | docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com

docker pull --platform linux/arm64 fayrus/syncret:$VERSION   # or linux/amd64
docker tag fayrus/syncret:$VERSION $ECR:$VERSION
docker push $ECR:$VERSION
```

The platform you copy must match the function's `--architectures` value. The [AWS deployment guide](https://syncret.fayr.us/deployment/aws/) covers the IAM role, the function, and the EventBridge rule.

## Which tag should I use?

Every release publishes four tags that point to the same image:

| Tag | Example | Moves when | Use it for |
| --- | --- | --- | --- |
| `vX.Y.Z` | `v0.5.0` | Never | Production. Reproducible copies to ECR; upgrade deliberately. |
| `vX.Y` | `v0.5` | Each patch release | Picking up the latest fixes within a minor version when you copy. |
| `vX` | `v0` | Each minor and patch release | Picking up fixes and new features without breaking changes when you copy. |
| `latest` | | Every release, including major versions | Trying Syncret out. Avoid in production. |

Your function runs the image you copied to ECR, so a moving tag only changes what you get the next time you copy it. All tags are on the [Tags](https://hub.docker.com/r/fayrus/syncret/tags) page, and each release is described in the [GitHub Releases](https://github.com/fayrus/syncret/releases).

## Configuration

Set environment variables on the Lambda function. At least one action is required:

```sh
SYNCRET_PROVIDER=aws
SYNCRET_AWS_REGION=us-east-1
SYNCRET_AWS_SECRET_ARN=<source secret ARN>      # the secret whose events trigger Syncret

# Action 1: copy fields into a dependent secret
SYNCRET_AWS_TARGET_SECRET_ARN=<target secret ARN>
SYNCRET_TARGET_SECRET_KEYS=password             # or source_key:target_key, comma-separated

# Action 2: redeploy ECS services
SYNCRET_AWS_ECS_FORCE_DEPLOY=true
SYNCRET_AWS_ECS_CLUSTER=my-cluster
SYNCRET_AWS_ECS_SERVICES=backend,worker

# Optional notifications (webhook URLs contain credentials; treat them as secrets)
SYNCRET_GCHAT_WEBHOOK=<Google Chat incoming webhook URL>
SYNCRET_SLACK_WEBHOOK=<Slack incoming webhook URL>
```

Invalid configuration fails at startup with a message naming the variable. See the [configuration reference](https://syncret.fayr.us/reference/) for every option.

## Image security

- Runtime image: [Chainguard `static`](https://images.chainguard.dev/directory/image/static/overview), containing only the statically linked `syncret` binary; no shell or package manager.
- Each release is built once, and every platform of that exact image is scanned with Trivy before it is pushed; a release with fixable HIGH or CRITICAL vulnerabilities is not published.
- Go code, modules, and the toolchain are checked with `govulncheck` on every pull request and daily.
- Report vulnerabilities privately as described in the [security policy](https://github.com/fayrus/syncret/security/policy).

## License

Apache License 2.0. See [LICENSE](https://github.com/fayrus/syncret/blob/main/LICENSE).
