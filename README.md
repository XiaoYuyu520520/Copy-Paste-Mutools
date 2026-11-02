<div align="center">

# 提示词复合工具箱 · Copy-Paste-Mutools

**多合一桌面工具箱**：提示词合并、局域网扫描、网速测试、文件共享、性能监视、系统信息、画板……

[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Fyne](https://img.shields.io/badge/Fyne-v2.8-8F4FFF?logo=appveyor&logoColor=white)](https://fyne.io/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Release](https://img.shields.io/badge/Release-v1.0.0-blue?logo=github)](https://github.com/XiaoYuyu520520/Copy-Paste-Mutools/releases)
[![Pages](https://img.shields.io/badge/Docs-GitHub%20Pages-success?logo=github)](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/)

</div>

---

## 🌐 在线工具（GitHub Pages · 免安装）

与桌面 / 命令行工具同属一个项目的网页版，打开即用，**全部在浏览器本地处理，不上传数据**：

| 工具 | 说明 |
| --- | --- |
| **[Base64 编解码](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/base64.html)** | UTF-8 / URL-safe / 文件转 Base64，本地处理 |
| **[二维码生成器](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/qrcode.html)** | 文本 / 网址即时生成，可下载 PNG、SVG |
| **[浏览器编解码检测](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/codec.html)** | H.264 / HEVC / AV1 / VP9 / AAC / Opus，含 WebRTC 实时通信与硬件编码检测 |
| **[Markdown 实时预览](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/markdown.html)** | GitHub 风格渲染、代码高亮、亮暗主题、左右对照 |
| **[2D 激光地图查看器](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/mapview.html)** | 打开 `.pgm + .yaml`：比例尺 / 测距 / 橡皮 / 导出 |

> 📘 文档站首页：<https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/>

---

## 📖 文档导航

| 想看什么 | 入口 |
| --- | --- |
| 📘 **完整文档站** | **[docs/index.md](docs/index.md)** · [在线版](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/) |
| 🧪 **在线工具（网页版）** | [Base64](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/base64.html) / [二维码](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/qrcode.html) / [编解码检测](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/codec.html) / [Markdown](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/markdown.html) / [激光地图](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/mapview.html) |
| 🚀 **快速上手（5 分钟）** | [docs/快速上手.md](docs/快速上手.md) |
| 🧰 **工具箱命令手册** | [docs/工具箱.md](docs/工具箱.md) |
| 🖥️ **桌面应用说明** | [docs/桌面应用.md](docs/桌面应用.md) |
| 🔀 **Python → Go 迁移对照表** | [docs/迁移对照表.md](docs/迁移对照表.md) |
| 🏗️ **构建与发布** | [docs/构建发布.md](docs/构建发布.md) |
| 🧭 **项目结构** | [docs/项目结构.md](docs/项目结构.md) |
| 📱 **Android App** | [docs/安卓方案.md](docs/安卓方案.md) |
| 📜 **历史教程（备忘）** | [docs/tutorials/](docs/tutorials/) |
| 🦊 **ROS2 相关** | [docs/ros2/](docs/ros2/) |
| 🗂️ **旧版 Python 源码（归档）** | [legacy/python/](legacy/python/) |

---

## 📦 这个仓库里有什么

本仓库是一个 **Go 多模块工程**，包含两个可执行程序：

### 1. `merger` — 提示词 + 语句自动合并（桌面应用）

基于 Go + [Fyne](https://fyne.io) 的原生桌面程序，无浏览器内核。

- **多标签**：顶部标签栏可任意增删，每个标签独立保存提示词 / 语句
- **实时合并**：勾选与文本即时联动，底部只读框实时预览结果（200 ms 去抖）
- **模拟键盘输入**：把合并结果逐字「真实键入」到目标程序，绕过粘贴检测
- **文件夹树**：按 `tree` 风格输出目录结构，可屏蔽构建/依赖目录
- **Markdown 渲染**：本地起服务 + 系统浏览器打开，GitHub 风格实时预览、代码高亮（[在线版](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/markdown.html)）
- **2D 激光地图**：打开 `.pgm + .yaml` 即进入地图查看器（比例尺 / 测量 / 橡皮）（[在线版](https://xiaoyuyu520520.github.io/Copy-Paste-Mutools/tools/mapview.html)）
- **局域网扫描**：自动枚举网卡与子网，ICMP + TCP 回退，表格实时刷新、导出 CSV
- **AI 聊天**：OpenAI 兼容 `/chat/completions` 流式对话，可接 OpenAI / DeepSeek / Qwen / Ollama
- **亮/暗主题**：一键切换并记忆

### 2. `toolbox` — 统一命令行工具箱

一个 exe 承载全部小工具，避免「一个工具一个 exe」的碎片化。

| 子命令 | 功能 | 迁移自 |
| --- | --- | --- |
| `tree` | 打印目录树（自动忽略构建/依赖目录） | `文件树/file_tree.py`、`tree_交互式输入路径.py` |
| `cpubench` | CPU 单核性能基准测试 | `CPU 单核基准测试脚本.py` |
| `diskbench` | 磁盘顺序读写吞吐测试 | `磁盘 IO 速度测试.py` |
| `diskm` | 实时磁盘 I/O 吞吐监控 | `磁盘工具/disk_monitor.py` |
| `netscan` | 局域网设备扫描（ICMP/TCP/MAC/主机名） | `network_windows.py`、`network_linux_v2.py`、`wlan_scan/` |
| `sysinfo` | 系统硬件与资源信息（支持 `--json`） | `读取设备信息.py` |
| `shortcut` | 生成桌面网址快捷方式 | `桌面网址快捷方式生成器.py` |
| `usb` | 列出 USB 设备（可导出 Markdown 快照） | `USB_scan/main.py` |
| `share` | 局域网文件共享服务（浏览器访问） | `lan-share-server/`（FastAPI） |
| `monitor` | 开发板性能监视器（SSE 实时推送） | `Linux开发板性能监视器/`（Flask） |
| `netspeed` | 局域网网速测试服务 | `局域网测速工具/`（本就是 Go，已并入主模块） |

另有独立 GUI 程序：

| 程序 | 功能 | 迁移自 |
| --- | --- | --- |
| `paint` | 简易画板（自由笔画 / 选色 / 调粗细 / 导出 PNG） | `最终优化版画板.py`（tkinter） |

### 3. `android` — 手机端 App

同一个代码库的 Android 版本（底部 5 个导航页）。只保留手机上真正可用的功能：

| 页 | 功能 | 说明 |
| --- | --- | --- |
| 编辑 | 提示词 + 语句合并 | 多标签、实时预览、一键复制 |
| 聊天 | AI 对话 | 流式输出，配置 OpenAI 兼容接口 |
| 扫描 | 局域网扫描 | Android 无 ICMP 权限，**强制 TCP 探测** |
| 画板 | 手指绘画 | 复用桌面版画板控件，支持导出 PNG |
| 更多 | AI 设置 / 本地服务 / 系统信息 | 文件共享、性能监视、网速测试 |

桌面端独有的「模拟键盘输入 / USB 枚举 / 桌面快捷方式 / 磁盘基准」在手机上不提供。

---

## 🚀 快速开始

### 下载即用

前往 [**Releases**](https://github.com/XiaoYuyu520520/Copy-Paste-Mutools/releases) 下载对应平台压缩包，解压后双击运行。

| 平台 | 产物 |
| --- | --- |
| Windows x64 | `merger-windows-amd64.zip`、`toolbox-windows-amd64.zip`、`paint-windows-amd64.zip` |
| Ubuntu x86_64 | `merger-linux-amd64.tar.gz`、`toolbox-linux-amd64.tar.gz`、`paint-linux-amd64.tar.gz` |
| Ubuntu ARM64 | `merger-linux-arm64.tar.gz`、`toolbox-linux-arm64.tar.gz`、`paint-linux-arm64.tar.gz` |
| **Android (arm64-v8a)** | `提示词工具箱.apk`（Debug 签名，直接安装） |

> Android 版由独立的 [`android.yml`](.github/workflows/android.yml) 工作流构建，
> 每次合并到 `main` 都会产出 APK artifact。详见 [docs/安卓方案.md](docs/安卓方案.md)。

### 自己编译

```bash
git clone https://github.com/XiaoYuyu520520/Copy-Paste-Mutools.git
cd Copy-Paste-Mutools

# 需要 Go 1.25+；Windows 上还需要 MinGW（MSYS2 UCRT64）以支持 CGO
go build -o merger.exe    ./cmd/merger
go build -o toolbox.exe   ./cmd/toolbox
go build -o paint.exe     ./cmd/paint

# Windows GUI 程序（双击不弹黑框）
go build -ldflags "-H windowsgui" -o merger.exe ./cmd/merger
```

跑一遍测试：

```bash
go test ./...
```

---

## 🖼️ 命令行示例

```bash
# 打印当前目录的树（忽略 node_modules/.git/build 等）
toolbox tree

# 限制深度、额外忽略某些目录
toolbox tree -depth 2 -ignore "dist,tmp" -keyword "test"

# CPU 单核跑分
toolbox cpubench

# 磁盘顺序读写（默认 1 GiB）
toolbox diskbench -size 512 -block 32

# 局域网扫描，自动推断网段，结果导出 CSV
toolbox netscan -timeout 800 -out hosts.csv

# 系统信息（JSON）
toolbox sysinfo --json --target all

# 生成桌面快捷方式
toolbox shortcut -name "GitHub" -url "https://github.com" -dir "D:\Desktop"

# 启动局域网文件共享（浏览器打开 http://<IP>:8000）
toolbox share -dir D:\共享 -addr 0.0.0.0:8000

# 启动开发板性能监视器
toolbox monitor -addr 0.0.0.0:8888
```

---

## 🧱 项目结构

```
Copy-Paste-Mutools/
├── cmd/
│   ├── merger/          # 桌面应用入口（Fyne）
│   ├── toolbox/         # 统一命令行入口
│   ├── paint/           # 画板入口
│   └── checktest/ mkicon/ screenshot/   # 开发辅助工具
├── internal/
│   ├── data/            # 合并规则、配置读写、目录树生成（纯逻辑，有单测）
│   ├── ui/              # Fyne 界面层
│   ├── render/          # Markdown / PGM 地图渲染服务
│   ├── chat/            # OpenAI 兼容流式聊天
│   ├── netscan/         # 局域网探测引擎（ICMP/TCP/MAC/主机名）
│   ├── lanscan/         # 浏览器版局域网扫描服务
│   ├── netspeed/        # 网速测试服务（含内嵌前端）
│   └── toolbox/         # 命令行各子命令实现
│       ├── bench/ sysinfo/ shortcut/ usb/
│       ├── share/ monitor/ diskm/ netscan/ paint/
│       └── webassets/   # 前端静态资源（go:embed）
├── docs/                # 文档站（GitHub Pages 源）
├── legacy/python/       # 旧版 Python 源码归档（只读参考，不参与构建）
├── .github/workflows/   # CI：构建 + 发布
└── go.mod
```

---

## 🔧 技术栈

- **Go 1.25+**（`go.mod` 声明 `go 1.25`）
- **Fyne v2.8** — 原生桌面 GUI（OpenGL）
- **gopsutil v4** — 跨平台系统信息采集（替代 Python 的 psutil + WMI + GPUtil）
- **gorilla/websocket** — 网速测试的实时通道
- **纯标准库** — HTTP 服务、SSE、ZIP 打包、目录遍历

---

## 📄 许可证

[MIT](LICENSE)

---

<div align="center">

**[⬆ 回到顶部](#提示词复合工具箱--copy-paste-mutools)**

Made with ❤️ by [XiaoYuyu520520](https://github.com/XiaoYuyu520520)

</div>
