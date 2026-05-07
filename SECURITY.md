# Security Policy

## Reporting a vulnerability

Please do not publish vulnerability details, exploit steps, tokens, credentials, private configuration, or generated private data in a public issue, pull request, discussion, or commit.

This project does not currently publish a dedicated security email address. If you need to report a security issue, use GitHub's private vulnerability reporting for this repository if it is available from the repository Security tab.

If private vulnerability reporting is unavailable, open a minimal public GitHub issue so a maintainer can arrange private follow-up. Keep the public issue limited to:

* The affected area at a high level.
* A statement that you can share details privately with a maintainer.
* No secrets, private prompts, private images, database files, `.env` values, `configs/config.yaml`, or generated data.
* No step-by-step exploit instructions or weaponized proof-of-concept details.

## Supported scope

Security fixes are generally handled for the current `main` branch and the latest released version when releases are available. Older unreleased snapshots or local forks may not receive separate fixes.

## Project security boundaries

`codex-imgen` is a local-first Go CLI and optional local async job service. It wraps Codex CLI image generation and stores local configuration, local job state, logs, and generated artifacts according to your configuration.

Important boundaries and assumptions:

* The service is intended to listen on `127.0.0.1` by default for local-only access. Only bind to `0.0.0.0` or another network-facing address if you intentionally want to expose it and understand the risks.
* Put sensitive values in `.env`, not in YAML or command examples.
* Keep structured local configuration in `configs/config.yaml`; commit only `configs/config.example.yaml`.
* Do not commit `.env`, `configs/config.yaml`, local SQLite databases, `.data/`, generated images, logs, or other private runtime data.
* Treat prompts, reference images, generated images, job records, and logs as potentially private.
* Review any issue, pull request, log excerpt, screenshot, or shell output for secrets before publishing it.

## Secret handling

Before contributing, run the local checks and secret scanner when possible:

```bash
bash ./scripts/ci-local.sh
bash ./scripts/secret-scan.sh --current --history
```

If you accidentally commit or publish a secret, rotate or revoke it immediately. Removing it from a later commit is not enough once it has been exposed.
