# Contributing to Syncret

Thanks for your interest in Syncret. Bug fixes, documentation improvements, and new cloud provider support are all welcome.

For anything larger than a small fix, please open an issue first so we can agree on the approach before you spend time on it.

Syncret handles sensitive data: never include secret values, tokens, or credentials in issues, pull requests, logs, or test fixtures. To report a vulnerability, follow the [security policy](SECURITY.md) instead of opening a public issue.

## Getting started

You need Go 1.27.3 or later and [golangci-lint](https://golangci-lint.run/) v2.15.0, the version CI uses.

```bash
make build   # build the binary to bin/syncret
make test    # run the tests with the race detector
make lint    # run the linters and the gofmt check
```

Tests never call real cloud services, so you do not need an AWS account to run them.

To preview documentation changes, install the dependencies from `docs/requirements.txt` and run `make docs-serve`.

## Making a change

1. Create a branch from `main` named `<type>/<what-it-does>`, for example `fix/reject-empty-secret-keys`. Types are `feat`, `fix`, `chore`, `docs`, and `refactor`.
2. Keep the change focused on one topic. For bug fixes, start with a test that fails without your fix.
3. Update `docs/` if you change behavior or configuration, and describe the effect for operators in `CHANGELOG.md`.
4. Use commit titles like `fix: reject empty secret keys`.
5. Open a pull request against `main` and fill in the template. CI must pass before merging.

## Versions and releases

Merging to `main` publishes a Docker image with the tag in the `VERSION` file, unless the pull request only touches `.github/`, `docs/`, `Makefile`, `LICENSE`, `.dockerignore`, or Markdown files in the repository root.

Any other pull request needs a new version, so that an image that is already published is not overwritten. If you are not sure which version to use, leave it as it is and a maintainer will take care of it before merging.

## Adding a cloud provider

Azure and GCP support is planned. Please open a feature request first. As a starting point:

- Implement the `SecretsProvider` and `ComputeProvider` interfaces from `internal/handler/handler.go`.
- Add the provider to the accepted values in `internal/config/config.go`.
- Prefix provider-specific environment variables with the provider name, as in `SYNCRET_AWS_*`.

## Code of conduct

By taking part in this project, you agree to follow its [Code of Conduct](CODE_OF_CONDUCT.md).
