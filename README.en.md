# codex-imgen

[中文](./README.zh-CN.md) | [English](./README.en.md)

`codex-imgen` is a local-first Go CLI and async job service for Codex CLI `$imagegen`. It turns Codex image generation into a scriptable workflow for text-to-image, image-to-image, batch-style generation, job tracking, and agent/service integration.

`codex-imgen` is an independent community tool and is not affiliated with OpenAI.

## Why codex-imgen?

Codex CLI already has image generation. `codex-imgen` focuses on the engineering layer around it:

- Run text-to-image and image-to-image from a simple local CLI
- Use local reference images with repeated `--image` flags
- Submit async jobs and query them later with `status`, `get`, `list`, and `cancel`
- Control candidate generation with `--count` and `--concurrency`
- Subscribe to job-scoped WebSocket events from local tools and agents
- Keep structured settings in `configs/config.yaml` and secrets in `.env`

If you only need one image once, native `codex exec` is enough. If you need repeatable generation, batching, job tracking, or local integration, use `codex-imgen`.

## Comparison

| Need | Native `codex exec` | `codex-imgen` |
|---|---:|---:|
| One-off prompt | yes | yes |
| Simple CLI UX | limited | yes |
| Local reference images | manual | yes |
| Batch count/concurrency | manual | yes |
| Async job queue | no | yes |
| Status/list/cancel | no | yes |
| WebSocket events | no | yes |
| Agent/service integration | manual | yes |
| Local YAML config | no | yes |

## Installation

#### Option 1: Download a release archive

Download the archive for your OS/arch from [GitHub Releases](https://github.com/walker1211/codex-imgen/releases), then unpack it:

```bash
tar -xzf codex-imgen_<tag>_<os>_<arch>.tar.gz
cd codex-imgen_<tag>_<os>_<arch>
cp configs/config.example.yaml configs/config.yaml
# Optional, only needed when email secrets are enabled:
cp .example.env .env
./imgen --help
```

On Windows, run `imgen.exe --help`.

Release archives include the `imgen` and `skill-sync` binaries, `configs/config.example.yaml`, `.example.env`, README files, and `LICENSE`.

#### Option 2: Build from source

Requires Go and a logged-in Codex CLI.

```bash
git clone https://github.com/walker1211/codex-imgen.git
cd codex-imgen
cp configs/config.example.yaml configs/config.yaml
# Optional, only needed when email secrets are enabled:
cp .example.env .env
bash ./build.sh
./imgen --help
```

The source-build commands above assume a Unix-like shell. On Windows, built binaries are named `imgen.exe` and `skill-sync.exe`.

Fill in `configs/config.yaml` before using service mode, custom backend settings, or email notifications. Put SMTP auth secrets in `.env` only.

Note: the binary reads `configs/config.yaml` from the current working directory. Adding the binary to `PATH` does not remove that requirement.

## Skill Sync

Repository directories `.claude/skills/imgen/` and `.openclaw/skills/imgen/` are the skill sources; `~/.claude/skills/imgen/` and `~/.openclaw/workspace/skills/imgen/` are local install artifacts.

Check whether local installs match the repository sources:

```bash
go run ./cmd/skill-sync --check
```

Copy repository sources into the local Claude and OpenClaw installs:

```bash
go run ./cmd/skill-sync --apply
```

You can also build first with `bash ./build.sh` and then use the local binary:

```bash
./skill-sync --check
./skill-sync --apply
```

The default behavior is drift checking only; local skill install directories are overwritten only when `--apply` is passed explicitly.

## Configuration

Repository config layout:

- `configs/config.example.yaml`: structured config template committed to git
- `configs/config.yaml`: real local structured config, not committed to git
- `.example.env`: secret template committed to git
- `.env`: real local secrets, not committed to git

Rules:

- Put only sensitive values in `.env`.
- Keep structured configuration in YAML.
- `EMAIL_SMTP_AUTH_CODE` is the SMTP auth code used for email delivery.

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

Configuration fields:

- `server.listen`: service-mode listen address. `127.0.0.1:18080` allows local access only; use `0.0.0.0:18080` only when you intentionally expose it to the network.
- `server.read_timeout`: HTTP request read timeout.
- `server.write_timeout`: HTTP response write timeout.
- `storage.data_dir`: async service data directory. If empty, the user data directory is used. For local development, `./.data` is a good choice.
- `storage.sqlite_path`: SQLite database path. If empty, `data_dir/imgen.db` is used. For local development, `./.data/imgen.db` is a good choice.
- `scheduler.global_max_concurrency`: global concurrency cap reserved for scheduler expansion.
- `scheduler.default_job_concurrency`: default concurrency for async jobs when `--concurrency` is not provided.
- `scheduler.max_job_concurrency`: maximum concurrency for one job; larger user input is clamped to this value.
- `scheduler.max_count_per_job`: maximum image count for one job; larger `--count` input is clamped to this value.
- `scheduler.maintenance_interval`: service-mode maintenance interval for checks, failure progression, and failure notification.
- `scheduler.task_lease_timeout`: running-task lease timeout used to detect expired work.
- `scheduler.max_attempts`: maximum generation attempts per image.
- `backend.type`: generation backend type. Currently use `built_in_codex`.
- `backend.command`: Codex CLI command. Defaults to `codex`.
- `backend.model`: model name passed to Codex CLI. If empty, Codex CLI chooses its default model.
- `backend.cwd`: Codex CLI working directory. If empty, the current process working directory is used. `~/` is expanded.
- `backend.timeout`: timeout for one Codex/imagegen invocation. Increase it if generation frequently times out.
- `backend.prompt.prefix`: prefix automatically prepended to prompts, usually `$imagegen`.
- `backend.prompt.prelude`: fixed prompt prelude for default style and output constraints.
- `email.enabled`: whether to enable maintenance failure email notification.
- `email.smtp_host`: SMTP server host.
- `email.smtp_port`: SMTP server port. Port `465` uses implicit TLS; other ports use a timeout-controlled standard SMTP connection.
- `email.from`: sender email address and SMTP login identity.
- `email.to`: recipient email address.
- `email.timeout`: timeout for one SMTP connection/send attempt.
- `email.retry_times`: maximum email send attempts.
- `email.retry_wait_time`: wait duration between failed email attempts.
- `email.use_proxy`: email proxy switch. SMTP proxying is not supported yet; setting this to `true` returns a config error.
- `.env` `EMAIL_SMTP_AUTH_CODE`: SMTP auth code or password. Required when email is enabled.

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

Start the local service in the foreground:

```bash
./imgen serve
```

Or use the repository scripts to run it in the background:

```bash
./start.sh
./stop.sh
./restart.sh
```

The background script uses `nohup ./imgen serve` and writes logs to `logs/out.log`.

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

To inspect whether a job retried, query the SQLite attempt history:

```bash
sqlite3 .data/imgen.db \
  "select job_id,image_index,attempt,status,duration_ms,path,last_error from job_image_attempts where job_id='<job-id>' order by image_index,attempt;"
```

To locate which part of one Codex CLI invocation is slow, inspect phase details:

```bash
sqlite3 .data/imgen.db \
  "select image_index,attempt,phase,elapsed_ms,detail from job_image_attempt_phases where job_id='<job-id>' order by image_index,attempt,occurred_at_ms;"
```

Common interpretation:

- Late `process.started`: Codex CLI startup or OS scheduling is slow.
- Late `stdout.thread_started`: Codex CLI initialization, network, or session creation is slow.
- Long gap from `stdout.turn_started` to `image.file_detected`: most time is waiting for image generation or file availability.
- Long gap from `image.file_detected` to `stdout.turn_completed`: the image file is already present, but Codex is still completing its final response or internal turn cleanup.
- Long gap from `stdout.turn_completed` to `process.exited`: the Codex turn is complete, but the CLI process exit is slow.
- If `stdout.turn_completed` is missing, a long gap from `image.file_detected` to `process.exited` means the image file is already present, but Codex CLI cleanup/exit is slow.
- If `image.file_detected` is missing, a long gap from `stdout.turn_started` to `stdout.saved_to` / `process.exited` still points to the model or imagegen tool execution chain.
- Long gap from `process.exited` to `parser.completed`: local parsing or generated_images directory lookup is slow.

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
- Failure email notification is wired into the maintenance path; richer failure classification and immediate notification are future work.

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
