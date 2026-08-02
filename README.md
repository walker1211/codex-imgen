[中文](./README.zh-CN.md)

# codex-imgen

`codex-imgen` is a local-first Go CLI and async job service for Codex CLI `$imagegen`. It turns Codex image generation into a scriptable workflow for text-to-image, image-to-image, batch-style generation, job tracking, and agent/service integration.

`codex-imgen` is an independent community tool and is not affiliated with OpenAI.

## Why codex-imgen?

Codex CLI already has image generation. `codex-imgen` focuses on the engineering layer around it:

- Run text-to-image and image-to-image from a simple local CLI
- Use local reference images with repeated `--image` flags
- Submit async jobs and query them later with `status`, `get`, `list`, and `cancel`
- Control candidate generation with `--count` and `--concurrency`
- Subscribe to job-scoped WebSocket events from local tools and agents
- Check OpenClaw/TG original-file delivery contracts and skill sync drift
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

## Demo

OpenClaw/TG delivery can send generated images as original files, with captions preserved for each result.

![OpenClaw Telegram original-file delivery demo](./docs/assets/openclaw-telegram-delivery-demo.png)

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

`.claude/skills/imgen/` is the skill source; `.openclaw/skills/imgen/` is the repository OpenClaw mirror; `~/.claude/skills/imgen/`, `~/.openclaw/workspace/skills/imgen/`, and `~/.codex/skills/imgen/` are local install artifacts.

Check whether local installs match the repository sources:

```bash
go run ./cmd/skill-sync --check
```

Copy repository sources into local Claude, OpenClaw, and Codex installs, and update the repository OpenClaw mirror:

```bash
go run ./cmd/skill-sync --apply
```

You can also build first with `bash ./build.sh` and then use the local binary:

```bash
./skill-sync --check
./skill-sync --apply
```

The default behavior is drift checking only; local skill install directories are overwritten only when `--apply` is passed explicitly.

## OpenClaw doctor

Check whether the local OpenClaw setup satisfies the imgen / Telegram original-file delivery contract:

```bash
./imgen doctor openclaw
```

This read-only command checks the `image_generate` deny rules, main agent `message` exposure, Telegram direct `NO_REPLY` silence, OpenClaw `message send --force-document` support, OpenClaw imgen skill installation/sync state, the `IMGEN_DELIVERY_DIR` / `forceDocument` / `asDocument` call contract, the synchronous CLI JSON success contract, and whether local `backend.delivery_dir` is under an OpenClaw-sendable root. WARN lines do not block; FAIL lines are actionable and mean the configuration, OpenClaw CLI capability, or skill sync needs fixing.

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
  listen: 127.0.0.1:18080 # Service listen address; local-only by default
  read_timeout: 5s # HTTP request read timeout
  write_timeout: 30s # HTTP response write timeout

storage:
  data_dir: "" # Service data directory; empty uses the OS user data directory
  sqlite_path: "" # SQLite database path; empty uses data_dir/imgen.db

scheduler:
  global_max_concurrency: 10 # Shared serve-mode generation queue cap for submit async and WebSocket realtime
  default_job_concurrency: 2 # Default per-job concurrency when submit omits --concurrency
  max_job_concurrency: 10 # Maximum per-job concurrency for submit async jobs
  max_count_per_job: 10 # Maximum image count for one submit async job
  maintenance_interval: 5m # Background maintenance interval
  task_lease_timeout: 30m # Background task lease timeout
  max_attempts: 3 # Maximum retry attempts per image in submit async jobs

backend:
  type: built_in_codex # Use local Codex CLI with the built-in $imagegen skill
  command: codex # Codex CLI command name or executable path
  model: "" # Empty uses the Codex CLI default model; set this only when pinning a model
  reasoning_effort: "" # Empty inherits Codex CLI config; set a level supported by the selected model
  cwd: "" # Codex CLI working directory; empty uses the current process directory
  timeout: 90s # Timeout for one Codex/imagegen invocation
  delivery_dir: "" # Optional: copy generated images there; OpenClaw/TG can point this at an allowed workspace/media directory
  delivery_max_files: 200 # Max files to retain in delivery_dir when set; 0 disables automatic cleanup
  cleanup_source_thread_dir: false # When delivery_dir is set and copying succeeds, delete the source Codex generated_images thread directory for this image
  prompt:
    prefix: "$imagegen" # Prefix prepended to every prompt
    prelude: | # Fixed prompt prelude for default style/output constraints
      Use the built-in imagegen skill.
      Output a single image.
      Default to web or brand asset scenarios.

realtime:
  enabled: true # Whether to enable the WebSocket realtime generation endpoint
  max_sessions: 4 # Maximum active WebSocket generation sessions
  max_items_per_session: 8 # Maximum items in one WebSocket generate.start frame
  max_count_per_item: 1 # Maximum image count for one realtime item
  item_timeout: 300s # Default timeout for one realtime item
  max_item_timeout: 300s # Maximum client timeout_ms; usually keep this equal to item_timeout

email:
  enabled: false # Whether to enable maintenance failure email notification
  smtp_host: smtp.example.com # SMTP server host
  smtp_port: 465 # SMTP port; 465 uses implicit TLS
  from: from@example.com # Sender email and SMTP login identity
  to: to@example.com # Recipient email
  timeout: 3s # Timeout for one SMTP connection/send attempt
  retry_times: 3 # Maximum email send attempts
  retry_wait_time: 500ms # Wait duration between failed email attempts
  use_proxy: false # SMTP proxying is not supported yet; keep false
```

Configuration fields:

- `server.listen`: service-mode listen address. `127.0.0.1:18080` allows local access only; use `0.0.0.0:18080` only when you intentionally expose it to the network.
- `server.read_timeout`: HTTP request read timeout.
- `server.write_timeout`: HTTP response write timeout.
- `storage.data_dir`: async service data directory. If empty, the user data directory is used. For local development, `./.data` is a good choice.
- `storage.sqlite_path`: SQLite database path. If empty, `data_dir/imgen.db` is used. For local development, `./.data/imgen.db` is a good choice.
- `scheduler.global_max_concurrency`: serve-mode bottom generation queue cap shared by async `submit` jobs and WebSocket realtime; it does not affect local sync shorthand generation with `imgen "prompt"`.
- `scheduler.default_job_concurrency`: default async `submit` job concurrency when `--concurrency` is omitted.
- `scheduler.max_job_concurrency`: maximum async `submit` job concurrency.
- `scheduler.max_count_per_job`: maximum image count for one async job; larger `--count` input is clamped to this value.
- `scheduler.maintenance_interval`: service-mode maintenance interval for checks, failure progression, and failure notification.
- `scheduler.task_lease_timeout`: running-task lease timeout used to detect expired work.
- `scheduler.max_attempts`: maximum generation attempts per image in async jobs.
- `realtime.enabled`: whether to enable the WebSocket realtime generation endpoint.
- `realtime.max_sessions`: maximum active WebSocket generation sessions at the same time.
- `realtime.max_items_per_session`: maximum items in one WebSocket `generate.start` frame.
- `realtime.max_count_per_item`: maximum image count per realtime item.
- `realtime.item_timeout`: default timeout for one realtime item; realtime no longer has its own backend global queue.
- `realtime.max_item_timeout`: maximum client `timeout_ms`; usually keep it equal to `item_timeout`.
- `backend.type`: generation backend type. Currently use `built_in_codex`.
- `backend.command`: Codex CLI command. Defaults to `codex`; the built-in backend currently requires this command to support `exec --json`.
- `backend.model`: model name passed to Codex CLI. If empty, the configured Codex backend chooses its default model.
- `backend.reasoning_effort`: optional Codex reasoning effort passed as a per-invocation config override. If empty, the Codex CLI configuration is inherited; supported values depend on the selected model.
- `backend.cwd`: Codex CLI working directory. If empty, the current process working directory is used. `~/` is expanded.
- `backend.timeout`: timeout for one Codex/imagegen invocation. Increase it if generation frequently times out.
- `backend.delivery_dir`: optional delivery directory. When set, generated images are copied there and the copied path is returned. OpenClaw/TG can point this at an allowed workspace/media directory.
- `backend.delivery_max_files`: maximum retained files in `delivery_dir` when set. Defaults to `200`; set `0` to disable automatic cleanup.
- `backend.cleanup_source_thread_dir`: defaults to `false`; when set to `true`, deletes the source Codex `generated_images/<thread-id>` directory for this image only after `delivery_dir` copying succeeds.
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

Text mode prints one image path per line. `--json` prints structured output. Automation should treat `ok: true` plus non-empty `images[].path` values as success, not exit code alone.

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
- The backend invokes Codex CLI as `<backend.command> exec --json [--config model_reasoning_effort=...] --image ... -- '<prompt>'`.
- The `--` separator is required for the native Codex CLI command because variadic `--image` would otherwise consume the prompt.
- Wrappers such as `ccs codex` are not automatically compatible; if `ccs codex exec --json` reports `unknown option '--json'`, the current built-in backend cannot use it directly.

When verifying native Codex CLI behavior, first confirm that the executable supports `exec --json`:

```bash
codex exec --help
codex exec --json -- '$imagegen Generate a cute baby dragon mascot, white background, single image'
codex exec --json --image ./1.png -- '$imagegen Keep the subject composition and pose, convert this image to a high-quality 3D figure render style, cleaner background, single image'
```

## Agent / OpenClaw / Telegram integration

This project only generates images and returns local file paths. Telegram, OpenClaw, or another agent must read the file pointed to by `images[].path` and upload the file bytes; a local path is not an image URL or a remote file id.

Integrations should follow this minimal contract:

1. Resolve the config working directory before calling the CLI: prefer `IMGEN_REPO_ROOT`; otherwise walk upward from the current directory until finding `configs/config.yaml` plus `./imgen`, `build.sh`, or `go.mod`; then try explicit user-provided install paths. Do not scan the whole filesystem.
2. Run `./imgen --json ...` from that directory, or use `./imgen get --json <job-id>` in service mode.
3. For OpenClaw/TG, use `IMGEN_DELIVERY_DIR` or `backend.delivery_dir` to copy images into an OpenClaw-sendable workspace/media directory, and use `delivery_max_files` to cap retained delivery files; explicitly enable `cleanup_source_thread_dir` only when you also want to remove the Codex source thread directory.
4. Synchronous success requires `ok: true` and non-empty `images[].path` values for the expected images; service jobs expose final files through `images[].path` after completion.
5. If Telegram reports something like `Media failed`, first check from the Telegram/OpenClaw runtime that `images[].path` exists, is readable, has a valid image format, and is on a shared or copied filesystem.

For Telegram multi-image requests that need distinct themes, OpenClaw should run independent `./imgen --json --count 1 --concurrency 1` commands concurrently, send each completed `images[].path` immediately with the `message` tool, use `forceDocument` or `asDocument` for original PNG delivery, and return exactly `NO_REPLY` after direct delivery.

### OpenClaw + Telegram quick start

1. Run `./skill-sync --apply` to sync the imgen skill, then restart OpenClaw.
2. Run `./imgen doctor openclaw` and confirm `message send supports --force-document` is OK and there are no FAIL lines.
3. Send a Telegram test message, for example: `Generate 3 cat Mac wallpapers with different moods`.
4. Expect 3 image files/documents. Brief status text and captions are fine, but the literal `NO_REPLY` should not be visible.

`NO_REPLY` is the silent completion signal for OpenClaw: after files have been delivered directly to Telegram, the agent should not add a final text reply.

For the full OpenClaw reproduction and configuration checklist, see [OpenClaw imgen Integration](./docs/openclaw-imgen-integration.md).

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
./imgen get --json <job-id>
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

- Text mode prints one image path per line.
- `--json` prints structured output; automation should read `images[].path`.
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
