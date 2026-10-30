***

# 📊 Hardware Monitor (Linux Edge)

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Python](https://img.shields.io/badge/python-3.10+-blue.svg)
![Vue](https://img.shields.io/badge/Vue-3.x-4fc08d.svg)
![ECharts](https://img.shields.io/badge/ECharts-5.x-e43961.svg)

基于 Python + Flask + Vue 3 的轻量级、实时硬件状态监视器。专为边缘计算设备和 Linux 开发板打造，**原生重点支持 NVIDIA Jetson 全系列**，并完美兼容 **树莓派 (Raspberry Pi)、RDK X5、常规 Linux 桌面/服务器** 以及 **桌面级 NVIDIA GPU**。

## 📸 屏幕截图

> ![Dashboard Screenshot](collector/display.png)

## ✨ 核心特性

- 🚀 **广泛的硬件兼容** — 自动识别桌面端与边缘端环境，无缝支持 Jetson、树莓派、RDK X5 及 x86/ARM 主机。
- 🧠 **CPU & 内存监控** — 实时追踪总占用率、单核负载、物理/逻辑核心数、运行频率，提供 60 秒历史趋势平滑折线图。
- 🎮 **智能 GPU 适配** — 自动检测并展示 GPU 占用率、频率、温度及显存（支持桌面 NVIDIA 与 Jetson 的底层数据采集）。
- 💿 **磁盘 & 操作系统** — 直观呈现各分区存储状态（进度条+柱状图），展示内核版本及系统架构。
- ⚡ **毫秒级实时通讯** — 采用 SSE (Server-Sent Events) 单向流技术，每秒低延迟推送，彻底告别轮询带来的性能损耗。
- 🌌 **现代化暗黑 UI** — 采用高级暗色玻璃态 (Glassmorphism) 设计语言，毛玻璃卡片搭配四色数据主题，结合 ECharts 丝滑动画渲染。

## 🚀 快速开始

确保你的设备已安装 Python 3.10 或更高版本。

```bash
# 1. 克隆项目

# 2. 一键启动 (推荐)
bash start.sh

# 3. 访问仪表盘
# 在浏览器中打开 http://localhost:8888 或 http://<设备IP>:8888
```

### 🛠 手动部署

```bash
# 安装依赖 (推荐使用国内镜像加速)
pip install -r requirements.txt -i https://pypi.tuna.tsinghua.edu.cn/simple

# 运行服务
python app.py
```

## ⚙️ 配置说明

为了保持代码整洁和易于维护，本项目遵循将**可配置参数统一放置在代码头文件或导入库下方**的规范。

如果你需要修改默认端口或刷新频率，可以直接编辑 `app.py` 顶部的全局变量，或者通过环境变量临时覆盖：

```bash
# 通过环境变量自定义端口启动
PORT=9999 bash start.sh
```

## 🖥️ 设备兼容性矩阵

本监控器内置了自适应降级逻辑，即使在没有独立 GPU 的开发板上也能稳定运行并展示基础系统信息。

| 硬件平台 | CPU/内存/磁盘 | GPU 支持依赖 | 自动检测逻辑 |
| :--- | :---: | :--- | :--- |
| **NVIDIA Jetson 系列** (JetPack) | ✅ | `jtop` / `sysfs` | 优先通过 jtop 获取，失败时回退读取 `/sys/devices/gpu.0/load` |
| **桌面级 NVIDIA 显卡** | ✅ | `pynvml` | 自动识别 NVIDIA 显卡及驱动环境 |
| **树莓派 / RDK X5 / 其他 Linux** | ✅ | — | 自动降级，GPU 模块优雅显示为「不适用」 |

## 📁 项目结构

```text
hardware-monitor/
├── app.py                  # Flask 主服务入口（API + SSE 路由分发）
├── requirements.txt        # Python 依赖清单
├── start.sh                # 快速启动脚本
├── collector/              # 数据采集模块
│   ├── __init__.py
│   ├── system_collector.py # 通用传感器采集（CPU / 内存 / 磁盘 / OS）
│   └── gpu_collector.py    # 智能 GPU 采集（桌面 NVIDIA + Jetson + 无GPU降级）
└── static/                 # 纯前端静态资源
    └── index.html          # Vue 3 响应式视图 + ECharts 渲染逻辑 (CDN 引入，无需 npm build)
```

## 🧰 技术栈

| 层级 | 核心技术 | 描述 |
| :--- | :--- | :--- |
| **后端** | `Python` / `Flask` / `psutil` | 提供轻量级的 Web 容器与基础系统状态采集 |
| **GPU 驱动** | `pynvml` / `jtop` | 分别用于对接 NVML API 与 Jetson 设备的 Tegra 状态 |
| **实时传输** | `SSE` (Server-Sent Events) | 建立轻量级服务端单向推送流 |
| **前端框架** | `Vue 3` (CDN) / `ECharts 5` | 无需构建工具的响应式数据绑定与可视化图表 |
| **视觉呈现** | `CSS3` Glassmorphism | 毛玻璃、渐变遮罩等现代化 UI 实现 |

## 🔌 API 接口

如果你希望将监控数据接入其他应用，可以直接调用以下接口：

- `GET /` — 渲染并返回前端 Dashboard 页面。
- `GET /api/status` — 获取当前硬件状态的 JSON 快照（适用于定时抓取）。
- `GET /api/stream` — 订阅 SSE 实时数据流（每秒推送最新 JSON 状态）。

## 📄 License

本项目基于 [MIT License](LICENSE) 开源。欢迎提交 PR 和 Issue！
