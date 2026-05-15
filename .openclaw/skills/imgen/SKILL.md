---
name: imgen
description: Use this skill whenever the user wants to use codex-imgen, imgen, Codex CLI $imagegen, text-to-image, image-to-image, local asynchronous image generation jobs, image job troubleshooting, or OpenClaw integration for local image generation. This skill helps choose the right imgen command or service flow, ask only necessary questions, keep secrets and config in the right files, and produce safe platform-neutral CLI/service guidance when another agent such as OpenClaw will call imgen.
---

# imgen

Use this skill to help users operate the `codex-imgen` project through its synchronous `imgen` CLI, local service jobs, and realtime WebSocket mode.

## First decide the scenario

Classify the user's request into one of these scenarios:

1. **Synchronous text-to-image** — user has a prompt and wants image paths directly.
2. **Synchronous image-to-image** — user supplies one or more local reference image paths and wants image paths directly.
3. **Asynchronous service job** — user wants `serve`, `submit`, `status`, `get`, `list`, `cancel`, polling, recovery, or server-side job management.
4. **Realtime WebSocket** — user explicitly wants streaming per-item generation events over `/v1/realtime/generate/ws`.
5. **Troubleshooting** — user reports slowness, failed jobs, retries, missing images, or confusing output.
6. **Configuration** — user asks about `configs/config.yaml`, `.env`, backend, scheduler, storage, or email settings.
7. **OpenClaw or another agent** — user wants platform-neutral instructions for another agent or tool to call imgen.

Read only the reference file that matches the scenario:

- Usage and OpenClaw calling contract: `references/imgen-usage.md`
- Prompt wording patterns: `references/prompt-patterns.md`
- Job and generation troubleshooting: `references/troubleshooting.md`

## Ask only necessary questions

If the user gave enough information, provide the command or steps directly.

Ask one question when a required value is missing:

- Missing image prompt for generation.
- Missing local image path for image-to-image.
- Missing job id for `status`, `get`, `cancel`, or job-specific troubleshooting.
- Missing decision between synchronous CLI, service job, or realtime WebSocket only when the user's goal requires choosing one.

Do not ask for preferences that are not needed to produce a safe minimal command.

## Command rules

Use the shortest command that satisfies the request. Default one-shot generation, including normal OpenClaw image generation, to synchronous `./imgen --json ...`; do not require `./imgen serve` unless the user needs job management or realtime streaming.

Before giving local execution instructions for OpenClaw or another agent, do not assume `<repo-root>` is known. Tell the caller to discover a config cwd in this order: `IMGEN_REPO_ROOT`, upward search for `configs/config.yaml` plus `./imgen`/`build.sh`/`go.mod`, explicit user install paths, then `command -v imgen` paired with a discovered config cwd. If no executable or config cwd is found, return a clear "imgen is not installed or not discoverable" error instead of running from an arbitrary cwd.

For OpenClaw or another local agent, resolve a repo/config root before running image requests. Check explicit common checkout paths such as `$HOME/Projects/codex-imgen` and `$HOME/codex-imgen` before falling back to `command -v imgen`; a PATH executable still needs a discovered config cwd. When a checkout root is found, run from that cwd. For normal text-to-image requests, use the synchronous CLI and run `./imgen --json --count N --concurrency 1 "<single-image prompt>"`; this does not require `./imgen serve`. For image-to-image requests, add repeated `--image <local-path>` flags before the prompt. Use `submit` only when the caller needs server-side job management, polling, cancellation, or recovery. Use `/v1/realtime/generate/ws` only when `imgen serve` is running and the caller needs live streaming events for a WebSocket session. For Telegram multi-image requests where distinct themes are useful, launch N independent `./imgen --json --count 1 --concurrency 1 "<single-image prompt>"` commands concurrently when the execution tool supports background sessions, then poll all sessions and deliver each successful `images[].path` immediately as it completes. Do not serialize these independent theme generations unless the tool cannot run them concurrently. Do not wait for all requested images before sending earlier successes. Do not call OpenClaw's built-in `image_generate` tool. Do not fall back to direct `codex exec --json -- '$imagegen ...'`. Do not reuse old generated image paths unless the user explicitly asks for existing files.

For multiple candidates, use `--count N` and `--concurrency M` to control quantity. Keep the prompt phrased as a single-image request such as `生成 1 张...` or `单图`; do not also ask for `生成 N 张` or `输出 N 张不同构图` inside the prompt.

Common commands:

```bash
./imgen "生成一张 Q 版小龙吉祥物，白底，单图"
./imgen --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
./imgen --image ./1.png --image ./2.png "以第一张为主体，采用第二张的风格和材质表现，输出单图"
./imgen serve
./imgen submit --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen status <job-id>
./imgen get <job-id>
./imgen list
./imgen cancel <job-id>
```

Prefer `./imgen --json ...` as the stable verification and OpenClaw contract. Direct native Codex CLI usage is only a diagnostic path for humans who explicitly ask for it; it is not an OpenClaw fallback. For OpenClaw image requests, do not call `codex exec --json -- '$imagegen ...'` directly; use the discovered repo/config root with `./imgen --json ...` instead.

If the local tool is `ccs codex` and `ccs codex exec --json` fails, do not reuse native Codex commands. Current `imgen` expects `backend.command` to accept `exec --json`; a plain `ccs codex '$imagegen ...'` exit code 0 without image paths is not a successful imgen-compatible result.

## Safety and configuration rules

- Treat `.env` as the only place for sensitive values such as `EMAIL_SMTP_AUTH_CODE`.
- Treat `configs/config.yaml` as the local structured configuration file.
- Do not suggest committing `configs/config.yaml`, `.env`, generated images, logs, or `.data/imgen.db`.
- Do not assume URL image input is supported; current image input is local file paths only.
- Do not suggest `0.0.0.0` service binding unless the user explicitly asks for LAN access and understands the exposure.
- Do not bypass Codex CLI login, permissions, or `$imagegen` availability checks.
- Do not claim generation succeeded from exit code alone; require text path lines or JSON `images[].path` values.
- Do not invent the image model; inspect `configs/config.yaml` `backend.model` when available, otherwise say it depends on the configured backend default.

## Troubleshooting flow

For slow or failed service jobs, follow this order:

1. Check the job summary with `./imgen status <job-id>` and `./imgen get <job-id>`.
2. Check image attempts in SQLite.
3. Check phase timings in SQLite.
4. Check `logs/out.log` when the service was started by repository scripts.
5. Verify with `./imgen --json`; only verify the underlying Codex command directly when it supports `exec --json`.

Use `references/troubleshooting.md` for exact SQL and phase interpretation.

## OpenClaw guidance

When the user wants OpenClaw support, produce platform-neutral instructions instead of Claude-specific tool steps.

Prefer one of these outputs:

- A synchronous CLI contract for normal image generation. It should say how to discover repo/config root, which `./imgen --json` command OpenClaw should run, what arguments it should supply, what output shape to expect, and how to report missing installation/configuration. This is the default OpenClaw contract and does not require `./imgen serve`.
- A local service job contract only when the caller needs server-side job management, polling, cancellation, or recovery. It should say to start `./imgen serve`, submit a job, poll or subscribe for completion, then read result paths.
- A realtime WebSocket contract only when the caller needs live streaming events. It should say to start `./imgen serve`, connect to `/v1/realtime/generate/ws`, send one `generate.start` frame, consume streamed events, and read image paths from `image.completed` events.

Mention these constraints:

- Image references are local file paths.
- `--count` / `--concurrency` control output quantity; prompt text should describe one image and its style constraints.
- Synchronous success means path lines in text mode, or `ok: true` plus non-empty `images[].path` in JSON mode.
- For Telegram delivery after synchronous CLI success, use the OpenClaw `message` tool when available: use `action="send"` to the current/original chat and attach the generated local file with the exact `path` or `filePath` returned by imgen. For PNG wallpapers or any image where original quality matters, set `forceDocument: true` or `asDocument: true` so Telegram sends the original file instead of compressing it as a photo. A concise caption/status message on the delivered image is acceptable when useful. After direct delivery, reply only `NO_REPLY`; OpenClaw Telegram direct chats should allow this silent reply instead of rewriting it into visible fallback text such as `No extra answer from me.` For multi-image requests with distinct themes, start the independent `./imgen --json --count 1 --concurrency 1` commands concurrently, poll them all, and send each completed `images[].path` immediately when that session finishes. If the `message` tool is unavailable, reply immediately with one `MEDIA:/absolute/path/to/image.png` line for each completed image.
- `submit --json` returns a job id first; final paths come from `get --json <job-id>` after completion. Use it only for service-job workflows, not normal one-shot OpenClaw generation.
- Realtime WebSocket requires `imgen serve`, streams `session.started`, `item.started`, `image.completed`, `item.failed`, and terminal session events, and does not create submit jobs or store rows.
- Secrets remain outside the calling prompt and stay in `.env`.
- For OpenClaw, use the discovered repo/config root, preferring `$HOME/Projects/codex-imgen` or `$HOME/codex-imgen` when present; otherwise the caller should not assume public network exposure, ccs Codex compatibility, `imgen serve` for normal one-shot generation, or a known repo-root.

## Before saying the work is ready

If you edited skill files, run from the skill directory and verify at minimum:

```bash
python3 -m json.tool evals/evals.json >/dev/null
python3 - <<'PY'
from pathlib import Path
markers = ['T' + 'BD', 'TO' + 'DO', 'PLACE' + 'HOLDER']
problems = []
for path in Path('.').rglob('*'):
    if path.is_file():
        text = path.read_text()
        for marker in markers:
            if marker in text:
                problems.append(f'{path}: {marker}')
if problems:
    raise SystemExit('\n'.join(problems))
PY
```

The marker scan should print no unresolved markers.
