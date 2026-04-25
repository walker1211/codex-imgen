# codex-imgen

`imgen` is a Go CLI that calls the native Codex CLI built-in `$imagegen` workflow for text-to-image and image-to-image generation, with an optional local service mode for asynchronous jobs.

[中文文档](./README.zh-CN.md) | [English Documentation](./README.en.md)

## Features

- Generate images synchronously from a prompt
- Use local image files as references with repeated `--image` flags
- Submit and query asynchronous jobs through a local service
- Query job status, list jobs, cancel jobs, and receive WebSocket events
- Use a local `configs/config.yaml` file from the current working directory

## Quick Start

```bash
cp configs/config.example.yaml configs/config.yaml
# Optional, only needed when email secrets are enabled:
cp .example.env .env
bash ./build.sh
./imgen --help
```

Fill in `configs/config.yaml` before using service mode or custom backend settings. Put secrets in `.env` only. Email settings are reserved for the maintenance notification hook; the production binary does not include a real SMTP dialer yet. Run commands from the repository or another working directory containing `configs/config.yaml`.

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
