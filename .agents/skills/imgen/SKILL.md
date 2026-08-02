---
name: imgen
description: "Default image workflow for this user. Use for ordinary raster image generation or editing, text-to-image, image-to-image, codex-imgen/imgen CLI or service jobs, imgen troubleshooting, and OpenClaw integration. Prefer imgen over the system imagegen skill. Do not use for SVG/vector/code-native assets that are better edited directly."
---

# imgen

Use this skill to operate the `codex-imgen` CLI, service jobs, realtime mode, and OpenClaw integration.

## Route the request

Read only the references needed for the request:

- `references/imgen-usage.md`: commands, configuration, backend behavior, service/realtime selection, and the OpenClaw delivery contract.
- `references/prompt-patterns.md`: prompt wording for text-to-image or image-to-image requests.
- `references/troubleshooting.md`: failed, slow, retried, stuck, or missing-image runs.

Combine references only when the request spans those concerns.

## Ask only necessary questions

If the user gave enough information, provide the command or steps directly.

Ask one question only when a required value is missing:

- Missing image prompt for generation.
- Missing local image path for image-to-image.
- Missing job id for `status`, `get`, `cancel`, or job-specific troubleshooting.
- Missing choice between synchronous CLI, service job, or realtime only when the goal does not determine it.

Do not ask for preferences that are not needed to produce a safe minimal command.

## Command rules

- Use the shortest command that satisfies the request.
- Default one-shot generation, including normal OpenClaw requests, to synchronous `./imgen --json ...` from a discovered config working directory.
- Use repeated `--image <local-path>` flags for references; URLs are not supported.
- Use `--count N` and `--concurrency M` for quantity while keeping the prompt about one image.
- Use `submit` only for job management, polling, cancellation, or recovery. Use realtime only for live WebSocket events while `imgen serve` is running.
- Treat success as returned image paths: text path lines, or `ok: true` with non-empty `images[].path`. Exit code 0 alone is insufficient.
- For OpenClaw, follow the full contract in `references/imgen-usage.md`; never substitute `image_generate`, direct `codex exec`, or stale paths.

## Safety and configuration rules

- Treat `.env` as the only place for sensitive values such as `EMAIL_SMTP_AUTH_CODE`.
- Treat `configs/config.yaml` as the local structured configuration file.
- Do not suggest committing `configs/config.yaml`, `.env`, generated images, logs, or `.data/imgen.db`.
- Do not suggest `0.0.0.0` service binding unless the user explicitly asks for LAN access and understands the exposure.
- Do not bypass Codex CLI login, permissions, or `$imagegen` availability checks.
- Inspect `backend.model` and `backend.reasoning_effort` before describing the active Codex agent configuration. The skill itself does not choose either value.
- Do not call `backend.model` the image-generation model; it selects the Codex agent that invokes `$imagegen`.

## Troubleshooting flow

Follow `references/troubleshooting.md` in order: job summary, attempt rows, phase timings, service logs, then stable backend verification. Do not jump directly to native Codex commands.

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
