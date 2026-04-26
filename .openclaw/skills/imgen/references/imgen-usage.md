# imgen Usage Reference

This reference is platform-neutral. OpenClaw or another local agent can use it to decide how to call `imgen`.

## What imgen does

`imgen` is a Go CLI for Codex CLI's built-in `$imagegen` workflow. It supports synchronous image generation and a local asynchronous service mode.

## Required local setup

From the repository root:

```bash
cp configs/config.example.yaml configs/config.yaml
cp .example.env .env
bash ./build.sh
./imgen --help
```

The `.env` file is only needed when email secrets are enabled. The CLI reads `configs/config.yaml` from the current working directory.

## Synchronous text-to-image

Use this when the caller wants image paths directly:

```bash
./imgen "生成一张 3D 风格的小龙吉祥物，适合网页 hero 区域，背景干净，单图"
./imgen --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen --count 4 --concurrency 2 --json "五更琉璃，穿着女仆装在咖啡馆"
```

Default text mode prints image paths. `--json` prints structured output.

## Synchronous image-to-image

Use local file paths with repeated `--image` flags:

```bash
./imgen --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
./imgen --json --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
./imgen --image ./1.png --image ./2.png "参考这些图片的主体特征，生成一张统一风格的高质量视觉图，单图"
```

Current image input is local file paths only. Do not pass image URLs.

## Native Codex CLI verification

Use these commands to verify the underlying `$imagegen` capability outside `imgen`:

```bash
codex exec --json -- '$imagegen 生成一张 Q 版小龙吉祥物，白底，单图'
codex exec --json --image ./1.png -- '$imagegen 保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图'
codex exec --json --image ./1.png ./2.png -- '$imagegen 以第一张为主体，采用第二张的风格和材质表现，输出单图'
```

Keep the `--` separator before the prompt.

## Service mode

Start the local service in the foreground:

```bash
./imgen serve
```

Or use repository scripts for background service management:

```bash
./start.sh
./stop.sh
./restart.sh
```

Submit and inspect jobs:

```bash
./imgen submit --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen submit --json --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen submit --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
./imgen status <job-id>
./imgen get <job-id>
./imgen list
./imgen cancel <job-id>
```

The service also exposes `/ws?job_id=<job-id>` for job-scoped WebSocket events.

## Configuration boundaries

Use these files:

- `configs/config.example.yaml` — committed structured config template.
- `configs/config.yaml` — local structured config, not committed.
- `.example.env` — committed secret template.
- `.env` — local secret values, not committed.

Important config fields:

- `server.listen` controls service binding. Use `127.0.0.1:18080` for local-only access.
- `storage.data_dir` and `storage.sqlite_path` control service data and SQLite paths.
- `scheduler.default_job_concurrency`, `scheduler.max_job_concurrency`, and `scheduler.max_count_per_job` control job fan-out limits.
- `backend.command` defaults to `codex`.
- `backend.model` is passed to Codex CLI when set.
- `backend.prompt.prefix` normally remains `$imagegen`.
- `email.enabled` enables maintenance failure email notification.
- `.env` key `EMAIL_SMTP_AUTH_CODE` supplies SMTP auth when email is enabled.

## OpenClaw calling contract

For CLI usage, OpenClaw should provide:

- Working directory containing `configs/config.yaml`.
- Prompt string.
- Optional local image paths.
- Optional `count`, `concurrency`, and `json` output choice.

For service usage, OpenClaw should:

1. Ensure `./imgen serve` is running locally.
2. Submit with `./imgen submit` or the local API exposed by the service.
3. Store the returned job id.
4. Poll status or subscribe to `/ws?job_id=<job-id>`.
5. Read final image paths from the completed job result.

OpenClaw should not assume URL image input, public network binding, or access to secrets inside prompts.
