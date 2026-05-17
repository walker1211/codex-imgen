# codex-imgen

Local-first Go CLI and async job service for Codex CLI `$imagegen`.

[中文](./README.zh-CN.md) | [English](./README.en.md)

`codex-imgen` turns Codex image generation into a scriptable local workflow: text-to-image, image-to-image with local references, async jobs, status tracking, cancellation, WebSocket events, and local configuration.

`codex-imgen` is an independent community tool and is not affiliated with OpenAI.

If you only need one image once, native `codex exec` is enough. If you need repeatable generation, batching, job tracking, or agent/service integration, use `codex-imgen`.

## Highlights

- Simple synchronous CLI for text-to-image and image-to-image
- Repeated `--image` flags for local reference images
- Async local service with `submit`, `status`, `get`, `list`, and `cancel`
- `--count` and `--concurrency` controls for batch-style workflows
- Job-scoped WebSocket events for local integrations
- OpenClaw/TG integration checks for original-file image delivery
- YAML configuration for structured settings and `.env` for secrets
- Go-first implementation that builds into local binaries

## Quick Start

Download the archive for your OS/arch from [GitHub Releases](https://github.com/walker1211/codex-imgen/releases), then run:

```bash
tar -xzf codex-imgen_<tag>_<os>_<arch>.tar.gz
cd codex-imgen_<tag>_<os>_<arch>
cp configs/config.example.yaml configs/config.yaml
# Optional, only needed when email secrets are enabled:
cp .example.env .env
./imgen --help
```

On Windows, run `imgen.exe --help`.

Requires a logged-in Codex CLI. To build from source instead, see [English](./README.en.md#installation) or [中文](./README.zh-CN.md#安装).

## Common Commands

```bash
./imgen "生成一张 Q 版小龙吉祥物，白底，单图"
./imgen --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
./imgen serve
./imgen submit --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen status <job-id>
./imgen get <job-id>
./imgen list
./imgen cancel <job-id>
```
