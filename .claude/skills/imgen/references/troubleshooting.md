# imgen Troubleshooting Reference

Use this order when a service job is slow, failed, stuck, retried, or missing images.

## 1. Check job status

```bash
./imgen status <job-id>
./imgen get <job-id>
./imgen list
```

Use `cancel` only when the user wants to stop the job:

```bash
./imgen cancel <job-id>
```

## 2. Inspect image attempts

When using the default local SQLite database path from the README examples:

```bash
sqlite3 .data/imgen.db \
  "select job_id,image_index,attempt,status,duration_ms,path,last_error from job_image_attempts where job_id='<job-id>' order by image_index,attempt;"
```

Use this to see which image index retried, failed, completed, and which path or error was recorded.

## 3. Inspect phase timings

```bash
sqlite3 .data/imgen.db \
  "select image_index,attempt,phase,elapsed_ms,detail from job_image_attempt_phases where job_id='<job-id>' order by image_index,attempt,occurred_at_ms;"
```

Interpret common phases this way:

- `process.started` is late: Codex CLI startup or system scheduling is slow.
- `stdout.thread_started` is late: Codex CLI initialization, network, or conversation creation is slow.
- `stdout.turn_started` to `image.file_detected` is long: image generation or file writing is the main cost.
- `image.file_detected` to `stdout.turn_completed` is long: the file appeared before Codex finished final response work.
- `stdout.turn_completed` to `process.exited` is long: Codex turn completed but the CLI process exited slowly.
- No `stdout.turn_completed` and `image.file_detected` to `process.exited` is long: image appeared but Codex CLI cleanup was slow.
- No `image.file_detected` and `stdout.turn_started` to `stdout.saved_to` or `process.exited` is long: model or imagegen tool execution is still the likely bottleneck.
- `process.exited` to `parser.completed` is long: local parsing or generated image discovery is slow.

## 4. Check service logs

When service mode was started with repository scripts, inspect:

```bash
logs/out.log
```

Look for startup errors, config load errors, backend command failures, scheduler messages, and email notification failures.

## 5. Verify backend compatibility

First verify through the stable `imgen` contract:

```bash
./imgen --json '生成一张 Q 版小龙吉祥物，白底，单图'
```

Treat the run as successful only when JSON contains `ok: true` and non-empty `images[].path` values. Exit code 0 without image paths is not enough.

Only run native Codex CLI checks when the executable supports `exec --json`:

```bash
codex exec --help
codex exec --json -- '$imagegen 生成一张 Q 版小龙吉祥物，白底，单图'
codex exec --json --image ./1.png -- '$imagegen 保留主体构图和姿态，改成高质量 3D 手办渲染风格，单图'
```

Keep the `--` separator before the prompt.

If the local tool is `ccs codex` and `ccs codex exec --json` reports `unknown option '--json'`, do not use it as an `imgen` backend. Current `imgen` calls `<backend.command> exec --json ...`; a wrapper that lacks this interface needs a dedicated adapter before it can be used reliably.

Native output must expose a parseable image path, such as `Saved to: file://...`, or a `thread.started` event that lets `imgen` discover files under `~/.codex/generated_images/<thread_id>`. Otherwise the expected failure is `image path not found in codex output`.

## 6. Configuration checks

Check these config values when failures mention config, storage, backend, scheduler, or email:

- `server.listen`
- `storage.data_dir`
- `storage.sqlite_path`
- `scheduler.default_job_concurrency`
- `scheduler.max_job_concurrency`
- `scheduler.max_count_per_job`
- `scheduler.task_lease_timeout`
- `scheduler.max_attempts`
- `backend.command`
- `backend.model`
- `backend.cwd`
- `backend.timeout`
- `email.enabled`
- `.env` key `EMAIL_SMTP_AUTH_CODE`

Do not ask the user to paste secret values.
