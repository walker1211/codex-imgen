# OpenClaw imgen Integration

[中文](./openclaw-imgen-integration.zh-CN.md)

This guide documents the repository-supported contract for using `codex-imgen` from OpenClaw, especially for Telegram image delivery.

`codex-imgen` generates local image files and returns local paths. OpenClaw or another agent is responsible for reading those files and uploading the file bytes to Telegram or another channel.

## Prerequisites

From a fresh checkout:

```bash
cp configs/config.example.yaml configs/config.yaml
# Optional, only when email notifications are enabled:
cp .example.env .env
bash ./build.sh
./imgen --help
```

Keep local configuration and secrets local:

- Do not commit `configs/config.yaml`.
- Do not commit `.env`.
- Do not commit OpenClaw runtime config, bot tokens, chat ids, logs, databases, or generated images.

## Verify imgen locally

Run one synchronous JSON request from the repo/config root:

```bash
./imgen --json --count 1 --concurrency 1 "Generate one simple test image, clean background, single image"
```

Treat generation as successful only when the JSON output has `ok: true` and the expected `images[].path` values are non-empty. An exit code of 0 without image paths is not enough.

## Install or sync the imgen skill

The canonical skill source in this repository is `.claude/skills/imgen`. The `.openclaw/skills/imgen` tree is a repository mirror, and local runtime installs are populated by `skill-sync`.

```bash
go run ./cmd/skill-sync --check
go run ./cmd/skill-sync --apply
go run ./cmd/skill-sync --check
```

After `--apply`, local installs are updated from the canonical source. The command does not make OpenClaw secrets or runtime config part of this repository.

## Required OpenClaw behavior

OpenClaw should follow this checklist for image requests:

- Resolve the `codex-imgen` repo/config root before running commands.
- Run `./imgen --json` from the discovered root.
- Do not call OpenClaw's built-in `image_generate` tool for these requests.
- Do not fall back to direct `codex exec --json -- '$imagegen ...'` for OpenClaw image requests.
- Require `ok: true` and non-empty `images[].path` values before claiming success.
- Do not reuse old generated image paths unless the user explicitly asks for existing files.

For Telegram delivery:

- Prefer the OpenClaw `message` tool when available.
- Send the exact `path` or `filePath` returned by `images[].path`.
- Use `forceDocument: true` or `asDocument: true` for PNG wallpapers or any original-quality image delivery.
- Concise captions or status text on delivered images are acceptable when useful.
- After direct file delivery, return exactly `NO_REPLY`.
- Configure Telegram direct chats so `NO_REPLY` stays silent and is not rewritten into visible fallback text such as `No extra answer from me.`

If the `message` tool is unavailable, the fallback is to return one `MEDIA:/absolute/path/to/image.png` line for each completed image.

## Delivery directory

OpenClaw/TG integrations should make the synchronous CLI return paths under a local directory that OpenClaw is allowed to send from. Set `IMGEN_DELIVERY_DIR` for each call, or configure local `backend.delivery_dir` in `configs/config.yaml`.

```bash
IMGEN_DELIVERY_DIR="${OPENCLAW_STATE_DIR:-$HOME/.openclaw}/workspace/imgen" ./imgen --json --count 1 --concurrency 1 "Theme, single image"
```

When using local config, also consider `backend.delivery_max_files` to cap retained delivery files. The default keeps 200 regular files; set it to 0 to disable automatic cleanup.

## Execution contract

A platform-neutral synchronous call looks like this:

```text
cwd: <repo-root>
command: ./imgen --json --count 1 --concurrency 1 "<single-image prompt>"
success: ok=true and images[0].path is non-empty
delivery: message action=send with path/filePath=<images[0].path> and forceDocument/asDocument=true
final reply: NO_REPLY
```

For image-to-image, pass local reference images with repeated `--image <local-path>` flags before the prompt. URL image inputs are not part of the current `imgen` contract.

## Telegram multi-image requests

When a Telegram request asks for multiple images and distinct themes are useful, OpenClaw should create multiple single-image prompts and run independent commands concurrently when its execution tool supports background sessions:

```bash
./imgen --json --count 1 --concurrency 1 "Theme A, single image"
./imgen --json --count 1 --concurrency 1 "Theme B, single image"
./imgen --json --count 1 --concurrency 1 "Theme C, single image"
```

Poll all sessions and send each successful `images[].path` as soon as that session completes. Do not wait for all requested images before sending earlier successes, and do not serialize independent theme generations unless the execution tool cannot run concurrent sessions.

## Troubleshooting

- If OpenClaw traces show `image_generate`, the wrong tool path was selected.
- If OpenClaw traces show direct `codex exec --json -- '$imagegen ...'`, replace that path with the `./imgen --json` contract.
- If `imgen` exits successfully but returns no `images[].path`, treat the request as failed and inspect the backend output.
- If Telegram says `Media failed`, check from the OpenClaw runtime that the generated path exists, is readable, and is on a shared or copied filesystem.
- If Telegram downloads a JPG/photo preview instead of the PNG original, verify that the `message` call used `forceDocument: true` or `asDocument: true`.
- If a final visible message like `No extra answer from me.` appears after files are delivered, allow direct `NO_REPLY` silence in OpenClaw and ensure the final assistant reply is exactly `NO_REPLY`.
- If no image paths are returned, inspect `configs/config.yaml` for `backend.command`, `backend.model`, and `backend.cwd`, and confirm the configured backend supports Codex `exec --json`.

## OpenClaw active-memory embedded run warning

Some OpenClaw versions can show a warning like `No callable tools remain` from an active-memory embedded run. This is an OpenClaw tool-policy issue, not an `imgen` generation or Telegram delivery issue.

The likely shape is:

- The active-memory embedded lane has a lane-local runtime policy such as `toolsAllow: memory_*`.
- The interactive main agent has explicit tool allow/alsoAllow entries such as `message` for Telegram file delivery.
- If OpenClaw inherits or combines the main agent allowlist into the embedded memory lane and computes callable tools by intersection, the result can be empty: memory tools satisfy the embedded lane, while `message` satisfies the interactive lane, and no tool satisfies both.

OpenClaw-side fixes should keep tool policy scoped by run type:

- Do not inherit the main agent's explicit allowlist into active-memory embedded lanes.
- Give active-memory embedded runs a lane-local policy that preserves the required `memory_*` tools.
- Scope `message` and other surface-delivery tools to interactive/surface/main runs only.
- Alternatively, provide finer-grained configuration such as interactive-only `agents.<id>.tools.alsoAllow` plus a separate `plugins.active-memory.tools` policy.

Until that warning affects actual memory behavior, it can be treated separately from the image-generation contract above.
