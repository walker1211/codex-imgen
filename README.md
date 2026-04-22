# codex-imgen

`imgen` 是一个调用原生 Codex CLI `$imagegen` 路径的 Go 命令行工具。

## 构建

```bash
bash ./build.sh
```

## 用法

```bash
./imgen "生成一张 3D 风格的小龙吉祥物，适合网页 hero 区域"
./imgen --json "生成一张极简风格产品插画"
./imgen --model gpt-5.4 --cwd ~/Projects/codex-helper "生成一张浅色品牌横幅"
```

## 配置文件

默认路径：`~/.config/codex-imgen/config.yaml`

```yaml
codex:
  command: codex
  cwd: ~/Projects/codex-helper
  model: gpt-5.4
  timeout: 90s
  extra_flags: []

prompt:
  prefix: "$imagegen"
  prelude: |
    使用内置 imagegen 技能。
    输出单张图片。
    默认适合网页或品牌资产场景。

output:
  format: text
```

## 输出

- 默认输出最终图片本地路径
- `--json` 输出结构化结果
- `--raw` 把 Codex 原始输出打印到 stderr，便于排查解析问题
- `--verbose` 打印实际调用摘要和工作目录
