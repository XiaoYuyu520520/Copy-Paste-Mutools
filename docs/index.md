<div align="center">

# 提示词复合工具箱 · 文档站

**Copy-Paste-Mutools** 的完整文档 —— 从这里开始。

[![GitHub Pages](https://img.shields.io/badge/docs-GitHub%20Pages-success?logo=github)](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/)
[![Release](https://img.shields.io/badge/Release-v1.0.0-blue?logo=github)](https://github.com/XiaoYuyu520520/Copy-Paste-Mutools/releases)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](https://github.com/XiaoYuyu520520/Copy-Paste-Mutools/blob/main/LICENSE)

</div>

---

## 🧭 快速导航

| # | 文档 | 内容 |
| --- | --- | --- |
| 1 | **[快速上手](快速上手.md)** | 5 分钟跑起来：下载、编译、第一个命令 |
| 2 | **[工具箱](工具箱.md)** | `toolbox` 全部 11 个子命令的用法与参数 |
| 3 | **[桌面应用](桌面应用.md)** | `merger` 的界面、功能、配置说明 |
| 4 | **[迁移对照表](迁移对照表.md)** | 旧 Python 工具 → 新 Go 工具的逐条对应 |
| 5 | **[项目结构](项目结构.md)** | 目录职责、包划分、设计取舍 |
| 6 | **[构建发布](构建发布.md)** | 本地编译、CI 流程、Release 产物 |
| 7 | **[画板](画板.md)** | `paint` 独立画板工具 |
| 8 | **[Android App](安卓方案.md)** | 手机端功能取舍、构建与安装 |
| 9 | [历史教程](tutorials/) | 共享磁盘、自动扩容、昇腾模型转换等备忘 |
| 10 | [ROS2 相关](ros2/) | ROS2 调试助手与 RDK 工具箱原始说明 |
| 11 | [旧版 Python 源码](../legacy/python/) | 归档参考，不参与构建 |

---

## 🧪 在线工具

| 工具 | 说明 |
| --- | --- |
| **[Base64 编解码](tools/base64.html)** | UTF-8 文本 / URL-safe / 文件转 Base64，纯本地处理 |
| **[二维码生成器](tools/qrcode.html)** | 文本 / 网址即时生成，支持下载 PNG、SVG |
| **[浏览器编解码检测](tools/codec.html)** | 检测 H.264 / HEVC / AV1 / VP9 / AAC / Opus，含 WebRTC 硬件编码 |
| **[Markdown 实时预览](tools/markdown.html)** | GitHub 风格渲染、代码高亮、亮暗主题、左右对照 |
| **[2D 激光地图](tools/mapview.html)** | 打开 .pgm + .yaml：比例尺 / 测距 / 擦除 / 导出；仅 YAML 时作为编辑器 |

---

## 🎯 按场景找答案

| 我想… | 去这里 |
| --- | --- |
| 合并提示词和语句，生成一段话 | [桌面应用](桌面应用.md) |
| 把文字「真实敲」进另一个程序 | [桌面应用](桌面应用.md) → 模拟键盘输入 |
| 找出局域网里有哪些设备 | [工具箱](工具箱.md) → `netscan` |
| 测一下家里网速 | [工具箱](工具箱.md) → `netspeed` |
| 把文件分享给同网段的人 | [工具箱](工具箱.md) → `share` |
| 看开发板 CPU / 内存 / 磁盘占用 | [工具箱](工具箱.md) → `monitor` |
| 跑分对比两台机器 | [工具箱](工具箱.md) → `cpubench` / `diskbench` |
| 把项目目录整理成文档 | [工具箱](工具箱.md) → `tree` |
| 查硬件配置 / 写进文档 | [工具箱](工具箱.md) → `sysinfo` |
| 列出 USB 设备 | [工具箱](工具箱.md) → `usb` |
| 随手画个图 | [画板](画板.md) |
| 在手机上用这套工具 | [Android App](安卓方案.md) |

---

## 📦 一分钟了解项目

本仓库是一个 **Go 多模块工程**，产出两个可执行程序：

- **`merger`** —— 基于 Fyne 的原生桌面应用，把「提示词 + 语句」按规则合并成一段话，
  并附带模拟键盘输入、目录树、Markdown 预览、激光地图查看、局域网扫描、AI 聊天等工具。
- **`toolbox`** —— 统一命令行工具箱，一个 exe 承载 11 个小工具。

原来的 Python 工具链（tkinter / Flask / FastAPI / psutil）已**全部改写为 Go**，
旧源码归档在 [`legacy/python/`](../legacy/python/)，逐条对应关系见[迁移对照表](迁移对照表.md)。

---

## 🔗 快速链接

- 📦 [Releases 下载](https://github.com/XiaoYuyu520520/Copy-Paste-Mutools/releases)
- 🐛 [Issues](https://github.com/XiaoYuyu520520/Copy-Paste-Mutools/issues)
- 💬 [Discussions](https://github.com/XiaoYuyu520520/Copy-Paste-Mutools/discussions)
- 👤 [作者 GitHub](https://github.com/XiaoYuyu520520)
- 🌐 [在线文档站](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/)

---

<div align="center">

**[⬆ 回到顶部](#提示词复合工具箱--文档站)**

</div>
