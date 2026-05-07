# Contributing

Thank you for considering a contribution to `codex-imgen`.

This project is a local-first Go CLI and optional local async job service for Codex CLI `$imagegen`. Please keep changes small, practical, and easy to review.

## Development environment

Recommended tools:

* Go version from `go.mod`
* Bash-compatible shell
* Git
* Codex CLI, logged in locally, for manual image generation checks

Install or verify Go dependencies with the normal Go toolchain:

```bash
go mod download
```

## Local configuration

Copy the example files before running service-mode or email-related features:

```bash
cp configs/config.example.yaml configs/config.yaml
cp .example.env .env
```

Keep secrets in `.env` only. Do not commit `.env`, `configs/config.yaml`, `.data/`, local databases, logs, or generated private assets.

The local service is intended to listen on `127.0.0.1` by default. Only expose it to a network intentionally.

## Build and run

Build the local binaries:

```bash
bash ./build.sh
```

Check the CLI help:

```bash
./imgen --help
```

## Tests and local CI

Run the standard local check before opening a pull request:

```bash
bash ./scripts/ci-local.sh
```

This runs the repository's local validation flow, including formatting, vetting, tests, build checks, and the configured secret scan.

You can also run focused Go checks while developing:

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
```

## Secret scanning

Before submitting changes, run:

```bash
bash ./scripts/secret-scan.sh --current --history
```

If the scanner reports a real secret, rotate or revoke the secret immediately. Do not include secret values in issue comments, pull request descriptions, screenshots, logs, or commits.

## Pull request flow

* Open an issue first for larger changes when the design or scope is unclear.
* Keep pull requests focused on one topic.
* Include a clear summary and test plan.
* Run `bash ./scripts/ci-local.sh` before requesting review.
* Update user-facing documentation or examples when behavior changes.
* Do not include generated private data, local config, local databases, or secrets.

## Commit messages

Use concise Conventional Commits when possible, for example:

```text
fix: handle empty async job output
feat: add service status endpoint
```

## Releases

Version tags, release builds, and publishing are performed by maintainers. Contributors should not create release tags unless a maintainer explicitly asks them to do so.
