# 图标素材

使用内置 image_gen 生成 PNG 原图，Rust 工具 `examples/prepare_icons.rs` 负责转换为多尺寸 RGBA ICO。运行状态为蓝青色，停止状态为石墨灰；EXE 使用运行状态图标。

图标原图为 `running.png` 和 `stopped.png`，项目根目录的 `app-enable.ico` 与 `app-disable.ico` 为实际嵌入资源。重建命令见项目 README。

## 生成提示词

运行状态：

> Use case: logo-brand. Create one polished Windows application icon for Clash Tray, a lightweight network proxy controller. A bold white flowing C-shaped route with two simple connection terminals, centered on a deep navy rounded square with restrained teal/cyan luminous gradient. Elegant crisp silhouette, strong contrast and thick simple geometry readable at 16px. Flat front view, very subtle depth, no tiny details, no text, no letters other than the abstract C form, no watermark, no surrounding mockup. Square 1024x1024. Rounded square fills 88 percent of canvas. Actual transparent background outside the rounded square. This is the enabled/running state icon. Save output for use in project.

停止状态（以上述运行状态 PNG 为编辑目标）：

> Edit target: attached enabled Clash Tray app icon. Create its stopped/off state. Preserve exact composition, rounded-square silhouette, C-shaped network route and two terminals, framing, padding and alpha transparency. Change only palette and glow: desaturate navy/cyan background into neutral graphite/slate gray, keep the white route clearly visible, remove cyan glow. No added text, no other design changes. Preserve transparent background outside icon.
