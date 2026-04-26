# imgen Prompt Patterns

Use these patterns to help users produce stable `imgen` prompts. Keep prompts explicit about subject, style, composition, and output count.

## Text-to-image pattern

Template:

```text
生成一张[主体]，[风格/材质]，[场景/背景]，[构图/用途]，单图
```

Example:

```bash
./imgen "生成一张 Q 版小龙吉祥物，白底，适合网页 hero 区域，单图"
```

## Multiple output pattern

Use CLI flags for count and concurrency. Keep the prompt focused on one image concept.

```bash
./imgen --count 4 --concurrency 2 "五更琉璃，穿着女仆装在咖啡馆，背景干净，单图"
```

## Single-reference image-to-image pattern

Template:

```text
保留主体构图和姿态，把这张图改成[目标风格]，背景更干净，单图
```

Example:

```bash
./imgen --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
```

## Two-reference pattern

Use the first image for subject and the second image for style when the user describes that intent.

```bash
./imgen --image ./1.png --image ./2.png "以第一张为主体，采用第二张的风格和材质表现，输出单图"
```

## Useful constraints

Add one or two constraints when they are useful:

- `单图`
- `白底`
- `背景干净`
- `保留主体构图和姿态`
- `统一风格`
- `适合网页 hero 区域`
- `适合品牌资产`
- `高质量 3D 手办渲染风格`

Do not overload the prompt with many unrelated styles. If the user asks for a broad idea, help them choose one coherent style direction.

## OpenClaw prompt guidance

When generating prompts for OpenClaw, return plain text and CLI-ready strings. Do not include Claude-only instructions.

Good OpenClaw-ready prompt:

```text
保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图
```

Good OpenClaw-ready command:

```bash
./imgen --image ./1.png "保留主体构图和姿态，把这张图改成高质量 3D 手办渲染风格，背景更干净，单图"
```
