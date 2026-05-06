---
name: imgen
description: Use when the user wants OpenClaw to call the local codex-imgen/imgen CLI or service for text-to-image, image-to-image, multi-reference image generation, asynchronous image jobs, job status checks, troubleshooting, or prompt shaping for local image generation. Prefer synchronous CLI for one-off generation and service mode only when the user needs background jobs, polling, WebSocket updates, cancellation, or long-running orchestration.
---

# imgen for OpenClaw

Use this skill to operate the local `codex-imgen` project through its `imgen` CLI or local service mode.

## Scenario selection

Choose the smallest workflow that satisfies the user:

1. **One-off text-to-image** — use synchronous CLI.
2. **One-off image-to-image** — use synchronous CLI with local `--image` paths.
3. **Multiple reference images** — use repeated `--image` flags.
4. **Background or long-running job** — use service mode and job polling.
5. **Troubleshooting** — inspect job state, SQLite attempts, phase timings, logs, and native Codex CLI behavior.
6. **Prompt shaping** — produce concise image prompts and CLI-ready commands.

Default to synchronous CLI unless the user explicitly needs background execution, job status tracking, cancellation, WebSocket events, or service integration.

## Read only the relevant reference

- CLI/service usage and calling contract: `references/imgen-usage.md`
- Prompt patterns: `references/prompt-patterns.md`
- Troubleshooting: `references/troubleshooting.md`

## Required working directory

When using shell or exec tools, set `cwd` to `<repo-root>` and run `./imgen` from there:

```text
cwd: <repo-root>
command: ./imgen submit "生成一张 Q 版小龙吉祥物，白底，单图"
```

If the tool cannot set `cwd`, prefix the shell command instead:

```bash
cd <repo-root> && ./imgen submit "生成一张 Q 版小龙吉祥物，白底，单图"
```

Do not put secrets in chat, prompts, or generated commands. Secret values such as `EMAIL_SMTP_AUTH_CODE` belong in `.env`.

## CLI command patterns

Text-to-image:

```bash
./imgen "生成一张 Q 版小龙吉祥物，白底，单图"
```

Multiple outputs:

```bash
./imgen --count 4 --concurrency 2 "生成 1 张五更琉璃穿着女仆装在咖啡馆的图，背景干净，单图"
```

For multiple candidates, use `--count N` and `--concurrency M` to control quantity. Keep the prompt phrased as a single-image request; do not also ask for `生成 N 张` or `输出 N 张不同构图` inside the prompt.

Image-to-image:

```bash
./imgen --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
```

Two references:

```bash
./imgen --image ./1.png --image ./2.png "以第一张为主体，采用第二张的风格和材质表现，输出单图"
```

Structured output for automation:

```bash
./imgen --json "生成一张 Q 版小龙吉祥物，白底，单图"
```

## Service mode patterns

Start local service when needed:

```bash
./imgen serve
```

Submit and inspect jobs:

```bash
./imgen submit --json --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen status <job-id>
./imgen get <job-id>
./imgen cancel <job-id>
```

Use `/ws?job_id=<job-id>` only when the caller needs job-scoped WebSocket events.

## Safety rules

- Use local image paths only; do not assume image URLs are supported.
- Keep `.env` for secrets and `configs/config.yaml` for structured config.
- Do not suggest committing `.env`, `configs/config.yaml`, generated images, logs, or `.data/imgen.db`.
- Do not suggest public binding such as `0.0.0.0` unless the user explicitly asks for LAN exposure and accepts the risk.
- Do not bypass Codex CLI login, permissions, or `$imagegen` availability checks.

## Troubleshooting order

For slow or failed jobs:

1. `./imgen status <job-id>`
2. `./imgen get <job-id>`
3. SQLite `job_image_attempts`
4. SQLite `job_image_attempt_phases`
5. `logs/out.log` if repository scripts started the service
6. Native `codex exec --json -- '$imagegen ...'` verification

Use `references/troubleshooting.md` for exact SQL and phase interpretation.
