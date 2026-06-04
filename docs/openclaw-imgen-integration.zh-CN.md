# OpenClaw imgen 集成

[English](./openclaw-imgen-integration.md)

本文档说明在 OpenClaw 中调用 `codex-imgen` 的仓库级契约，重点是 Telegram 图片投递。

`codex-imgen` 负责生成本机图片文件并返回本机路径。OpenClaw 或其他 agent 负责读取这些文件，并把文件内容上传到 Telegram 或其他渠道。

## 前置条件

从全新 checkout 开始：

```bash
cp configs/config.example.yaml configs/config.yaml
# 可选：仅在启用邮件通知时需要
cp .example.env .env
bash ./build.sh
./imgen --help
```

本地配置和 secret 只保留在本地：

- 不要提交 `configs/config.yaml`。
- 不要提交 `.env`。
- 不要提交 OpenClaw 运行时配置、bot token、chat id、日志、数据库或生成图片。

## 本地验证 imgen

在仓库/配置根目录运行一次同步 JSON 请求：

```bash
./imgen --json --count 1 --concurrency 1 "生成一张简单测试图，背景干净，单图"
```

只有当 JSON 输出包含 `ok: true`，并且期望数量的 `images[].path` 非空时，才应视为生成成功。退出码为 0 但没有图片路径，不算成功。

## 安装或同步 imgen skill

仓库中的标准 skill 源目录是 `.claude/skills/imgen`。`.openclaw/skills/imgen` 是仓库内 OpenClaw 镜像，本机运行时安装目录由 `skill-sync` 填充。

```bash
go run ./cmd/skill-sync --check
go run ./cmd/skill-sync --apply
go run ./cmd/skill-sync --check
```

执行 `--apply` 后，本机 skill 安装会从仓库标准源更新。该命令不会把 OpenClaw secret 或运行时配置写入本仓库。

## OpenClaw 必须遵守的行为

OpenClaw 处理图片请求时应遵守以下清单：

- 运行命令前先定位 `codex-imgen` 仓库/配置根目录。
- 从发现的根目录运行 `./imgen --json`。
- 不要为这些请求调用 OpenClaw 内置 `image_generate` 工具。
- 不要在 OpenClaw 图片请求中回退到直接调用 `codex exec --json -- '$imagegen ...'`。
- 声称成功前，必须确认 `ok: true` 且 `images[].path` 非空。
- 除非用户明确要求使用已有文件，否则不要复用旧生成图片路径。

Telegram 投递要求：

- 优先使用 OpenClaw `message` 工具。
- 发送 `images[].path` 返回的确切 `path` 或 `filePath`。
- 对 PNG 壁纸或任何需要原图质量的图片，使用 `forceDocument: true` 或 `asDocument: true`。
- 简短 caption 或有用的状态说明可以保留。
- 直接完成文件投递后，最终返回必须是 `NO_REPLY`。
- Telegram direct chat 要配置为让 `NO_REPLY` 保持静默，不要被改写成 `No extra answer from me.` 这类可见 fallback 文本。

如果 `message` 工具不可用，fallback 是为每张完成图片返回一行 `MEDIA:/absolute/path/to/image.png`。

## 投递目录

OpenClaw/TG 集成建议把同步 CLI 的返回路径限制在 OpenClaw 可发送的本地目录下。可以在每次调用时设置 `IMGEN_DELIVERY_DIR`，也可以在本地 `configs/config.yaml` 中配置 `backend.delivery_dir`。

```bash
IMGEN_DELIVERY_DIR="${OPENCLAW_STATE_DIR:-$HOME/.openclaw}/workspace/imgen" ./imgen --json --count 1 --concurrency 1 "主题，单图"
```

如果使用本地配置，建议同时设置 `backend.delivery_max_files` 控制投递目录保留文件数。默认保留 200 个普通文件；设为 0 可关闭自动清理。只有在也希望删除已复制图片对应的 Codex `generated_images/<thread-id>` 原始目录时，才设置 `backend.cleanup_source_thread_dir: true`。

## 执行契约

平台无关的同步调用形态如下：

```text
cwd: <repo-root>
command: ./imgen --json --count 1 --concurrency 1 "<single-image prompt>"
success: ok=true and images[0].path is non-empty
delivery: message action=send with path/filePath=<images[0].path> and forceDocument/asDocument=true
final reply: NO_REPLY
```

图生图场景中，在 prompt 前重复传入 `--image <local-path>` 本机参考图。URL 图片输入不属于当前 `imgen` 契约。

## Telegram 多图请求

当 Telegram 请求多张图且适合拆成不同主题时，OpenClaw 应创建多个单图 prompt，并在执行工具支持后台 session 时并发运行独立命令：

```bash
./imgen --json --count 1 --concurrency 1 "主题 A，单图"
./imgen --json --count 1 --concurrency 1 "主题 B，单图"
./imgen --json --count 1 --concurrency 1 "主题 C，单图"
```

轮询所有 session；某个 session 成功后，立刻发送对应的 `images[].path`。不要等所有图片都完成后才发送早已成功的图片；除非执行工具不能并发后台 session，否则不要串行执行独立主题生成。

## 排障

- 如果 OpenClaw trace 中出现 `image_generate`，说明选错了工具路径。
- 如果 OpenClaw trace 中出现直接 `codex exec --json -- '$imagegen ...'`，应改回 `./imgen --json` 契约。
- 如果 `imgen` 成功退出但没有返回 `images[].path`，应把请求视为失败并检查 backend 输出。
- 如果 Telegram 返回 `Media failed`，从 OpenClaw 运行环境检查生成路径是否存在、可读，并确认文件系统共享或文件已复制到可访问位置。
- 如果 Telegram 下载的是 JPG/photo 预览而不是 PNG 原图，确认 `message` 调用使用了 `forceDocument: true` 或 `asDocument: true`。
- 如果文件已发送后又出现 `No extra answer from me.` 之类最终可见消息，应允许 OpenClaw direct `NO_REPLY` 静默，并确认最终 assistant 回复严格等于 `NO_REPLY`。
- 如果没有返回图片路径，检查 `configs/config.yaml` 中的 `backend.command`、`backend.model` 和 `backend.cwd`，并确认配置的 backend 支持 Codex `exec --json`。

## OpenClaw active-memory 嵌入运行 warning

某些 OpenClaw 版本可能从 active-memory 嵌入运行中打印类似 `No callable tools remain` 的 warning。这是 OpenClaw 工具策略问题，不是 `imgen` 生图或 Telegram 投递问题。

常见形态是：

- active-memory 嵌入 lane 有局部运行策略，例如 `toolsAllow: memory_*`。
- 交互式 main agent 有显式工具 allow/alsoAllow，例如用于 Telegram 文件投递的 `message`。
- 如果 OpenClaw 把 main agent 的 allowlist 继承或合并进嵌入 memory lane，再用交集计算可调用工具，结果可能为空：memory 工具满足嵌入 lane，`message` 满足交互 lane，但没有工具同时满足两边。

OpenClaw 侧修复应保持工具策略按运行类型隔离：

- 不要把 main agent 的显式 allowlist 继承到 active-memory 嵌入 lane。
- 给 active-memory 嵌入运行保留 lane-local 策略，确保所需 `memory_*` 工具仍可用。
- 把 `message` 等 surface 投递工具限制在交互/surface/main 运行中。
- 或提供更细粒度配置，例如 interactive-only 的 `agents.<id>.tools.alsoAllow`，再配合独立的 `plugins.active-memory.tools` 策略。

只要这个 warning 没有影响实际 memory 行为，就可以和上面的图片生成契约分开处理。
