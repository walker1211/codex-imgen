# codex-imgen

`imgen` 是一个调用原生 Codex CLI built-in `$imagegen` 路径的 Go 工具，支持同步生图，并提供本地服务模式来提交和查询异步任务。

## 构建

```bash
bash ./build.sh
```

## 同步模式（已可用）

```bash
./imgen "生成一张 3D 风格的小龙吉祥物，适合网页 hero 区域"
./imgen --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen --count 4 --concurrency 2 --json "五更琉璃，穿着女仆装在咖啡馆"
./imgen --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
```

## 图生图说明

- 使用重复 `--image` 传入本机图片路径。
- 同步 `run` 与异步 `submit` 参数语义一致。
- 当前方案 A 只支持本机文件路径，不支持 URL 和上传文件。
- backend 调用 Codex CLI 时会生成 `codex exec --json --image ... -- '<prompt>'`。
- `--` 分隔符是必需的，否则 Codex CLI 的 variadic `--image` 会吞掉 prompt。

## 服务模式（当前最小闭环已可用）

先启动本地服务：

```bash
imgen serve
```

再在另一个终端提交和查询：

```bash
imgen submit --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
imgen submit --json --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
imgen status <job-id>
imgen get <job-id>
imgen list
imgen cancel <job-id>
```

## WebSocket（最小推送链已接通）

服务暴露 `/ws?job_id=<job-id>`，当前支持建立真实 WebSocket 连接，并把 job/image 事件从 service 发布到 WebSocket hub。当前能力仍偏最小：

- 已有真实连接升级
- 已有按 `job_id` 的订阅模型
- 已有 `job.created` / `job.started` / `image.completed` / `job.completed` 等事件发布
- 更丰富的协议、历史回放和断线恢复仍未实现

## 配置文件

默认路径：`~/.config/codex-imgen/config.yaml`

```yaml
server:
  listen: 127.0.0.1:18080

scheduler:
  global_max_concurrency: 10
  default_job_concurrency: 2
  maintenance_interval: 5m
  max_attempts: 3

backend:
  type: built_in_codex
  command: codex
  model: gpt-5.4
  timeout: 90s
  prompt:
    prefix: "$imagegen"
    prelude: |
      使用内置 imagegen 技能。
      输出单张图片。
      默认适合网页或品牌资产场景。

email:
  enabled: true
  smtp_host: smtp.example.com
  smtp_port: 465
  from: from@example.com
  to: to@example.com
  timeout: 3s
  retry_times: 3
  retry_wait_time: 500ms
  use_proxy: false
```

## 输出

- 默认 text 模式输出图片路径
- `--json` 输出结构化结果
- 多图同步模式下一行一个路径
- 服务模式支持 `job_id` 查询状态与图片路径
- maintenance ticker 已接入 `serve`，会做最小巡检、失败状态推进和最终失败通知
- 邮件通知当前通过 maintenance 路径触发；更精细的失败分类与即时通知仍可继续完善
