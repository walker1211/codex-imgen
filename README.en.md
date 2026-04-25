# codex-imgen

[Landing Page](./README.md) | [中文](./README.zh-CN.md)

## Installation

#### Option 1: Build from source

Requires Go and a logged-in Codex CLI.

```bash
cp configs/config.example.yaml configs/config.yaml
# Optional, only needed when email secrets are enabled:
cp .example.env .env
bash ./build.sh
./imgen --help
```

Fill in `configs/config.yaml` before using service mode or custom backend settings. Put secrets in `.env` only. Email settings are reserved for the maintenance notification hook; the production binary does not include a real SMTP dialer yet.

Note: the binary reads `configs/config.yaml` from the current working directory. Adding the binary to `PATH` does not remove that requirement.

## Configuration

Repository config layout:

- `configs/config.example.yaml`: structured config template committed to git
- `configs/config.yaml`: real local structured config, not committed to git
- `.example.env`: secret template committed to git
- `.env`: real local secrets, not committed to git

Rules:

- Put only sensitive values in `.env`.
- Keep structured configuration in YAML.
- `EMAIL_SMTP_AUTH_CODE` is the SMTP auth-code placeholder for email. This project currently standardizes config loading conventions; a real SMTP dialer should be implemented separately.

Initialization:

```bash
cp configs/config.example.yaml configs/config.yaml
cp .example.env .env
```

Then edit `configs/config.yaml` as needed:

```yaml
server:
  listen: 127.0.0.1:18080
  read_timeout: 5s
  write_timeout: 30s

storage:
  data_dir: ""
  sqlite_path: ""

scheduler:
  global_max_concurrency: 10
  default_job_concurrency: 2
  max_job_concurrency: 10
  max_count_per_job: 10
  maintenance_interval: 5m
  task_lease_timeout: 30m
  max_attempts: 3

backend:
  type: built_in_codex
  command: codex
  model: gpt-5.4
  cwd: ""
  timeout: 90s
  prompt:
    prefix: "$imagegen"
    prelude: |
      Use the built-in imagegen skill.
      Output a single image.
      Default to web or brand asset scenarios.

email:
  enabled: false
  smtp_host: smtp.example.com
  smtp_port: 465
  from: from@example.com
  to: to@example.com
  timeout: 3s
  retry_times: 3
  retry_wait_time: 500ms
  use_proxy: false
```

Common fields:

- `server.listen`: local service address.
- `storage.data_dir`: default service data directory. If empty, the user data directory is used.
- `storage.sqlite_path`: SQLite database path. If empty, `data_dir/imgen.db` is used.
- `scheduler.default_job_concurrency`: default concurrency for async jobs.
- `scheduler.max_job_concurrency`: maximum concurrency for one job.
- `scheduler.max_count_per_job`: maximum image count for one job.
- `backend.command`: Codex CLI command. Defaults to `codex`.
- `backend.model`: model name passed to Codex CLI.
- `backend.cwd`: Codex execution working directory. Optional.
- `backend.timeout`: timeout for one Codex invocation.
- `backend.prompt.prefix`: default `$imagegen` prefix.
- `email.enabled`: reserved maintenance-notification switch; the production binary does not perform real SMTP delivery yet.

## Synchronous text-to-image

```bash
./imgen "Generate a 3D-style baby dragon mascot for a web hero section, clean background, single image"
./imgen --count 4 --concurrency 2 "Kuroneko wearing a maid outfit in a cafe"
./imgen --count 4 --concurrency 2 --json "Kuroneko wearing a maid outfit in a cafe"
```

Text mode prints image paths. `--json` prints structured output.

## Synchronous image-to-image

Use local image files as references:

```bash
./imgen --image ./1.png "Keep the subject composition and pose, convert this image to a high-quality 3D figure render style, cleaner background, single image"
./imgen --json --image ./1.png "Keep the subject composition and pose, convert this image to a high-quality 3D figure render style, cleaner background, single image"
```

Pass multiple reference images by repeating `--image`:

```bash
./imgen --image ./1.png --image ./2.png "Use these images as subject references and generate one consistent high-quality visual"
```

Notes:

- Only local file paths are supported in this version. URLs and uploads are not supported.
- Synchronous `run` and asynchronous `submit` use the same `--image` semantics.
- The backend invokes Codex CLI as `codex exec --json --image ... -- '<prompt>'`.
- The `--` separator is required for the native Codex CLI command because variadic `--image` would otherwise consume the prompt.

Native Codex CLI examples:

```bash
codex exec --json -- '$imagegen Generate a cute baby dragon mascot, white background, single image'
codex exec --json --image ./1.png -- '$imagegen Keep the subject composition and pose, convert this image to a high-quality 3D figure render style, cleaner background, single image'
```

## Service mode

Start the local service:

```bash
./imgen serve
```

Submit and query from another terminal:

```bash
./imgen submit --count 4 --concurrency 2 "Kuroneko wearing a maid outfit in a cafe"
./imgen submit --json --count 4 --concurrency 2 "Kuroneko wearing a maid outfit in a cafe"
./imgen submit --image ./1.png "Keep the subject composition and pose, convert this image to a high-quality 3D figure render style, cleaner background, single image"
./imgen status <job-id>
./imgen get <job-id>
./imgen list
./imgen cancel <job-id>
```

## WebSocket

The service exposes `/ws?job_id=<job-id>` for job-scoped event subscriptions. Current event types include:

- `job.created`
- `job.started`
- `image.started`
- `image.completed`
- `image.failed`
- `image.cancelled`
- `job.completed`
- `job.partial_success`
- `job.failed`
- `job.cancelled`

The WebSocket implementation is intentionally minimal: it supports connection upgrade, `job_id` subscriptions, and event pushes. Historical replay and reconnect recovery are future work.

## Output

- Text mode prints image paths.
- `--json` prints structured output.
- Multi-image sync mode prints one path per line.
- Service mode supports querying status and image paths by `job_id`.
- The maintenance ticker is wired into `serve` for minimal checks, failure progression, and final failure notification.
- The maintenance-notification hook is wired into the maintenance path; a real SMTP dialer, richer failure classification, and immediate notification are future work.

## Development / Testing

```bash
go test ./...
bash ./build.sh
```

When changing CLI flags, config loading, or README content, also verify:

```bash
./imgen --help
./imgen --json "Generate a cute baby dragon mascot, white background, single image"
```
