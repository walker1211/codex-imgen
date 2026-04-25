# codex-imgen

[入口页](./README.md) | [English](./README.en.md)

## 安装

#### 方式一：从源码构建

需要 Go 环境，以及已登录可用的 Codex CLI。

```bash
cp configs/config.example.yaml configs/config.yaml
# 可选：仅在启用邮件 secret 时需要
cp .example.env .env
bash ./build.sh
./imgen --help
```

在使用服务模式或自定义 backend 前，请先填写 `configs/config.yaml`。敏感信息只放 `.env`。邮件配置目前是维护通知钩子的预留配置，生产二进制还没有内置真实 SMTP dialer。

说明：程序默认从当前工作目录读取 `configs/config.yaml`。即使把二进制加入 `PATH`，也仍然需要在包含 `configs/config.yaml` 的工作目录中运行。

## 配置

仓库内配置布局：

- `configs/config.example.yaml`：结构化配置模板，提交到版本库
- `configs/config.yaml`：本地真实结构化配置，不提交到版本库
- `.example.env`：敏感信息模板，提交到版本库
- `.env`：本地真实敏感信息，不提交到版本库

原则：

- `.env` 只放敏感值。
- YAML 只放结构化配置。
- `EMAIL_SMTP_AUTH_CODE` 是邮件 SMTP 授权码占位；当前项目只对齐配置加载约定，真实 SMTP dialer 后续单独实现。

初始化方式：

```bash
cp configs/config.example.yaml configs/config.yaml
cp .example.env .env
```

然后按需编辑 `configs/config.yaml`：

```yaml
server:
  listen: 127.0.0.1:18080
  read_timeout: 5s
  write_timeout: 30s

storage:
  data_dir: ""
  sqlite_path: ""

scheduler:
  global_max_concurrency: 10
  default_job_concurrency: 2
  max_job_concurrency: 10
  max_count_per_job: 10
  maintenance_interval: 5m
  task_lease_timeout: 30m
  max_attempts: 3

backend:
  type: built_in_codex
  command: codex
  model: gpt-5.4
  cwd: ""
  timeout: 90s
  prompt:
    prefix: "$imagegen"
    prelude: |
      使用内置 imagegen 技能。
      输出单张图片。
      默认适合网页或品牌资产场景。

email:
  enabled: false
  smtp_host: smtp.example.com
  smtp_port: 465
  from: from@example.com
  to: to@example.com
  timeout: 3s
  retry_times: 3
  retry_wait_time: 500ms
  use_proxy: false
```

常用字段：

- `server.listen`：本地服务监听地址。
- `storage.data_dir`：异步服务默认数据目录；为空时使用用户目录下的默认数据路径。
- `storage.sqlite_path`：SQLite 数据库路径；为空时使用 `data_dir/imgen.db`。
- `scheduler.default_job_concurrency`：异步任务默认并发数。
- `scheduler.max_job_concurrency`：单个任务最大并发数。
- `scheduler.max_count_per_job`：单个任务最大图片数量。
- `backend.command`：Codex CLI 命令，默认 `codex`。
- `backend.model`：传给 Codex CLI 的模型名。
- `backend.cwd`：Codex 执行工作目录，可为空。
- `backend.timeout`：单次 Codex 调用超时。
- `backend.prompt.prefix`：默认 `$imagegen` 前缀。
- `email.enabled`：预留的邮件维护通知开关；生产二进制暂不执行真实 SMTP 发送。

## 同步文生图

```bash
./imgen "生成一张 3D 风格的小龙吉祥物，适合网页 hero 区域，背景干净，单图"
./imgen --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen --count 4 --concurrency 2 --json "五更琉璃，穿着女仆装在咖啡馆"
```

默认 text 模式会输出图片路径；`--json` 会输出结构化结果。

## 同步图生图

使用本机图片路径作为参考图：

```bash
./imgen --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
./imgen --json --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
```

多张参考图可以重复 `--image`：

```bash
./imgen --image ./1.png --image ./2.png "参考这些图片的主体特征，生成一张统一风格的高质量视觉图，单图"
```

说明：

- 当前只支持本机文件路径，不支持 URL 或上传文件。
- 同步 `run` 与异步 `submit` 的 `--image` 参数语义一致。
- backend 调用 Codex CLI 时会生成 `codex exec --json --image ... -- '<prompt>'`。
- `--` 分隔符对原生 Codex CLI 是必需的，否则 variadic `--image` 会吞掉 prompt。

直接使用 Codex CLI 验证底层能力：

```bash
codex exec --json -- '$imagegen 生成一张 Q 版小龙吉祥物，白底，单图'
codex exec --json --image ./1.png -- '$imagegen 保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图'
```

## 服务模式

先启动本地服务：

```bash
./imgen serve
```

再在另一个终端提交和查询：

```bash
./imgen submit --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen submit --json --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆"
./imgen submit --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
./imgen status <job-id>
./imgen get <job-id>
./imgen list
./imgen cancel <job-id>
```

## WebSocket

服务暴露 `/ws?job_id=<job-id>`，支持按 job 订阅事件。当前支持的事件包括：

- `job.created`
- `job.started`
- `image.started`
- `image.completed`
- `image.failed`
- `image.cancelled`
- `job.completed`
- `job.partial_success`
- `job.failed`
- `job.cancelled`

当前 WebSocket 能力仍偏最小：已有连接升级、按 `job_id` 订阅和事件推送；历史回放与断线恢复后续再完善。

## 输出

- 默认 text 模式输出图片路径。
- `--json` 输出结构化结果。
- 多图同步模式下一行一个路径。
- 服务模式通过 `job_id` 查询状态与图片路径。
- maintenance ticker 已接入 `serve`，会做最小巡检、失败状态推进和最终失败通知。
- 维护通知钩子已接入 maintenance 路径；真实 SMTP dialer、失败分类与即时通知后续再完善。

## 开发 / 测试

```bash
go test ./...
bash ./build.sh
```

如修改了命令行参数、配置加载或 README，建议同时验证：

```bash
./imgen --help
./imgen --json "生成一张 Q 版小龙吉祥物，白底，单图"
```
