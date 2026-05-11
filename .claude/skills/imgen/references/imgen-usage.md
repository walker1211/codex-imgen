# imgen Usage Reference

This reference is platform-neutral. OpenClaw or another local agent can use it to decide how to call `imgen`.

## What imgen does

`imgen` is a Go CLI for Codex CLI's built-in `$imagegen` workflow. It supports synchronous image generation and a local asynchronous service mode.

## Required local setup

`imgen` has two local requirements:

1. An `imgen` executable.
2. A config working directory containing `configs/config.yaml`.

From a fresh repository checkout:

```bash
cp configs/config.example.yaml configs/config.yaml
cp .example.env .env
bash ./build.sh
./imgen --help
```

The `.env` file is only needed when email secrets are enabled. The CLI reads `configs/config.yaml` from the current working directory, so a globally installed `imgen` binary is not enough by itself; the caller still needs a config cwd.

## Finding repo-root or config cwd

Do not assume `<repo-root>` is known. If a user or agent wants to run local commands, discover the working directory in this order:

1. Use `IMGEN_REPO_ROOT` when it is set and contains `configs/config.yaml` plus either `./imgen`, `build.sh`, or `go.mod`.
2. If the current directory is inside a checkout, walk upward until finding `configs/config.yaml` plus either `./imgen`, `build.sh`, or `go.mod`.
3. Check explicit user-provided install paths or common local checkout paths such as `$HOME/Projects/codex-imgen` and `$HOME/codex-imgen`; do not scan the whole filesystem.
4. Use `command -v imgen` only to find an executable. Pair it with a config cwd found above; if no config cwd exists, report that the CLI is not fully installed/configured.

If discovery fails, return a clear error instead of running from an arbitrary cwd:

```text
imgen is not installed or not discoverable. Set IMGEN_REPO_ROOT to the codex-imgen checkout, or install/build imgen and create configs/config.yaml.
```

When an agent uses shell or exec tools, prefer setting `cwd` to the discovered repo/config root before running `./imgen`; if `cwd` cannot be set, prefix the command with `cd <repo-root> &&`.

## Synchronous text-to-image

Use this when the caller wants image paths directly:

```bash
./imgen "生成一张 3D 风格的小龙吉祥物，适合网页 hero 区域，背景干净，单图"
./imgen --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen --count 4 --concurrency 2 --json "五更琉璃，穿着女仆装在咖啡馆"
```

Default text mode prints one image path per completed image, one path per line. `--json` prints structured output; treat success as `ok: true` plus non-empty `images[].path` values for the expected completed images. An exit code of 0 without image paths is not enough to declare generation successful.

For multi-candidate generation, use `--count` / `--concurrency` for quantity. Keep the prompt phrased as a single-image request, such as `生成 1 张...` or `单图`; do not also ask for `生成 N 张` or `输出 N 张不同构图` inside the prompt.

## Synchronous image-to-image

Use local file paths with repeated `--image` flags:

```bash
./imgen --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
./imgen --json --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
./imgen --image ./1.png --image ./2.png "参考这些图片的主体特征，生成一张统一风格的高质量视觉图，单图"
```

Current image input is local file paths only. Do not pass image URLs.

## Native backend verification

Prefer `./imgen --json ...` as the stable local contract. It verifies the same backend path that OpenClaw should call and returns image paths in a known shape.

Only use native Codex CLI verification when the available executable supports `exec --json`:

```bash
codex exec --help
codex exec --json -- '$imagegen 生成一张 Q 版小龙吉祥物，白底，单图'
codex exec --json --image ./1.png -- '$imagegen 保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图'
codex exec --json --image ./1.png --image ./2.png -- '$imagegen 以第一张为主体，采用第二张的风格和材质表现，输出单图'
```

Keep the `--` separator before the prompt.

`ccs codex` is not automatically compatible with `imgen`. Current `imgen` invokes `<backend.command> exec --json ...`; if `ccs codex exec --json` reports `unknown option '--json'`, do not use it as the backend command and do not treat `ccs codex '$imagegen ...'` exit code 0 as image-generation success. Use `./imgen --json` or service mode as the caller contract, and report that the native ccs wrapper lacks the required `exec --json` interface unless the project adds a dedicated adapter.

Native Codex output is successful only when it exposes an image path that `imgen` can parse, such as `Saved to: file://...`, or when a `thread.started` event lets `imgen` discover files under `~/.codex/generated_images/<thread_id>`. Otherwise `imgen` reports `image path not found in codex output`.

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
./imgen get --json <job-id>
./imgen list
./imgen cancel <job-id>
```

`submit --json` returns a job id and status, not final image paths. Poll `get --json <job-id>` until the job is complete, then read final paths from `images[].path`.

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
- `backend.command` defaults to `codex` and must be a single executable that accepts `exec --json`.
- `backend.model` is passed to Codex CLI when set; when empty, the actual model is whatever the backend executable uses by default.
- `backend.cwd` is passed to Codex CLI as `--cd` when set.
- `backend.prompt.prefix` normally remains `$imagegen` and is prepended to the prompt text.
- `email.enabled` enables maintenance failure email notification.
- `.env` key `EMAIL_SMTP_AUTH_CODE` supplies SMTP auth when email is enabled.

To confirm the current backend and model, inspect the local config from the discovered repo/config root:

```bash
grep -nE '^[[:space:]]*(command|model|cwd|prefix):' configs/config.yaml
```

If `configs/config.yaml` is unavailable, do not invent a model. Say the model depends on the local `backend.model` setting or the default model of the configured Codex backend.

## OpenClaw calling contract

OpenClaw should first resolve a config cwd using the discovery rules above. Then use one of these stable contracts.

For CLI usage, OpenClaw should provide:

- `cwd`: the discovered repo/config root.
- `command`: `./imgen --json ...` when using a repository checkout, or an absolute `imgen` executable path only when paired with the discovered config cwd.
- Prompt string describing one image or one candidate.
- Optional local image paths via repeated `--image` flags.
- Optional `count`, `concurrency`, and `json` output choice.

Expected synchronous JSON result:

```json
{
  "ok": true,
  "count": 1,
  "completed": 1,
  "failed": 0,
  "images": [
    {"index": 0, "status": "completed", "path": "/local/path.png", "uri": "file:///local/path.png"}
  ]
}
```

Treat the call as successful only when `ok` is true and the expected completed images have non-empty `path` values. In text mode, treat non-empty path lines as the result.

If an execution tool cannot set `cwd`, use `cd <repo-root> && ./imgen ...` as the shell command.

For service usage, OpenClaw should:

1. Ensure `./imgen serve` is running locally from the discovered repo/config root.
2. Submit with `./imgen submit --json` or the local API exposed by the service.
3. Store the returned job id.
4. Poll `./imgen get --json <job-id>` or subscribe to `/ws?job_id=<job-id>`.
5. Read final image paths from `images[].path` after completion.

OpenClaw should not assume URL image input, public network binding, public filesystem scans, ccs Codex compatibility, or access to secrets inside prompts.
