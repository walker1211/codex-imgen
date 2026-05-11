---
name: imgen
description: Use this skill whenever the user wants to use codex-imgen, imgen, Codex CLI $imagegen, text-to-image, image-to-image, local asynchronous image generation jobs, image job troubleshooting, or OpenClaw integration for local image generation. This skill helps choose the right imgen command or service flow, ask only necessary questions, keep secrets and config in the right files, and produce safe platform-neutral CLI/service guidance when another agent such as OpenClaw will call imgen.
---

# imgen

Use this skill to help users operate the `codex-imgen` project through its `imgen` CLI and local service mode.

## First decide the scenario

Classify the user's request into one of these scenarios:

1. **Synchronous text-to-image** — user has a prompt and wants image paths immediately.
2. **Synchronous image-to-image** — user supplies one or more local reference image paths.
3. **Asynchronous service job** — user wants `serve`, `submit`, `status`, `get`, `list`, or `cancel`.
4. **Troubleshooting** — user reports slowness, failed jobs, retries, missing images, or confusing output.
5. **Configuration** — user asks about `configs/config.yaml`, `.env`, backend, scheduler, storage, or email settings.
6. **OpenClaw or another agent** — user wants platform-neutral instructions for another agent or tool to call imgen.

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
- Missing decision between synchronous CLI and service mode when the user's goal requires choosing one.

Do not ask for preferences that are not needed to produce a safe minimal command.

## Command rules

Use the shortest command that satisfies the request.

Before giving local execution instructions for OpenClaw or another agent, do not assume `<repo-root>` is known. Tell the caller to discover a config cwd in this order: `IMGEN_REPO_ROOT`, upward search for `configs/config.yaml` plus `./imgen`/`build.sh`/`go.mod`, explicit user install paths, then `command -v imgen` paired with a discovered config cwd. If no executable or config cwd is found, return a clear "imgen is not installed or not discoverable" error instead of running from an arbitrary cwd.

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

Prefer `./imgen --json ...` as the stable verification and OpenClaw contract. Native Codex CLI examples are only valid when the available executable supports `exec --json`; preserve the `--` separator so repeated `--image` flags do not consume the prompt:

```bash
codex exec --help
codex exec --json -- '$imagegen 生成一张 Q 版小龙吉祥物，白底，单图'
codex exec --json --image ./1.png -- '$imagegen 保留主体构图和姿态，改成高质量 3D 手办渲染风格，单图'
```

If the local tool is `ccs codex` and `ccs codex exec --json` fails, do not reuse these native examples. Current `imgen` expects `backend.command` to accept `exec --json`; a plain `ccs codex '$imagegen ...'` exit code 0 without image paths is not a successful imgen-compatible result.

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

- A CLI contract that says how to discover repo/config root, which `./imgen --json` command OpenClaw should run, what arguments it should supply, what output shape to expect, and how to report missing installation/configuration.
- A local service contract that says to start `./imgen serve`, submit a job, poll or subscribe for completion, then read result paths.

Mention these constraints:

- Image references are local file paths.
- `--count` / `--concurrency` control output quantity; prompt text should describe one image and its style constraints.
- Synchronous success means path lines in text mode, or `ok: true` plus non-empty `images[].path` in JSON mode.
- `submit --json` returns a job id first; final paths come from `get --json <job-id>` after completion.
- Secrets remain outside the calling prompt and stay in `.env`.
- The caller should not assume public network exposure, ccs Codex compatibility, or a known repo-root.

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
