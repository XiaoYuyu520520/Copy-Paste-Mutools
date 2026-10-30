***

# 🤖 RDK 小车开发常用工具箱

![OS](https://img.shields.io/badge/OS-Linux%20%7C%20Windows-blue)
![Python](https://img.shields.io/badge/Python-3.8+-green.svg)
![ROS2](https://img.shields.io/badge/ROS2-Jazzy-orange.svg)

> 本项目是针对 **RDK (Robot Development Kit) 小车** 开发场景收集整理的专属工具集。涵盖了底层性能评估、系统硬件监控、外设状态扫描、网络探测以及 ROS2 节点诊断等常用功能，旨在提升机器人开发与调试效率。

---

## 📑 目录

- [📁 项目结构](#-项目结构)
- [🧮 CPU 与性能基准测试](#-cpu-与性能基准测试)
- [💾 磁盘与 I/O 工具](#-磁盘与-io-工具)
- [🦊 ROS2 调试助手](#-ros2-调试助手)
- [🔌 USB 与外设扫描](#-usb-与外设扫描)
- [🌐 局域网网络扫描](#-局域网网络扫描)
- [📡 激光雷达调试](#-激光雷达调试)
- [🧰 实用小脚本](#-实用小脚本)
- [⚙️ 开发与配置规范](#️-开发与配置规范)

---

## 📁 项目结构

```text
RDK小车开发常用工具/
├── CPU测试/
│   └── cpu-benchmark/         # C++ CPU 性能基准测试 (整数/浮点/核间延迟)
├── CPU&磁盘基准测试/
│   ├── CPU单核基准测试.py       # Python 单核跑分脚本
│   └── 磁盘IO速度测试.py        # 顺序读写吞吐量测试
├── 磁盘工具/
│   └── disk_monitor.py        # 实时磁盘 I/O 监控 (%util, 读写速度)
├── 局域网设备扫描/
│   ├── network_linux.py       # Linux 基础 Ping 扫描
│   ├── network_linux_v2.py    # Linux 增强版扫描 (含 MAC、主机名)
│   ├── network_windows.py     # Windows 局域网扫描
│   └── scan_results.txt
├── wlan_scan/                 # 跨平台高级网络扫描框架 (ARP/TCP/多线程)
├── USB_scan/                  # USB 设备/雷达串口扫描与数据解析
├── ros2_helper/               # ROS2 辅助调试工具 (话题解析、消息导出)
└── 文件夹结构打印工具/
    └── tree_交互式输入路径.py   # 自定义目录树打印 (自动忽略特定文件夹)
```

---

## 🧮 CPU 与性能基准测试

评估 RDK 开发板核心算力与调度性能。

### 1. 高性能基准测试 (C++)
位于 `CPU测试/cpu-benchmark/`，通过编译型语言深度压榨硬件性能。
*   **测试项**：整数运算、单/双精度浮点运算 (FMA)、多核间通信延迟。
*   **输出**：终端报告与热力图 PNG。
```bash
cd "CPU测试/cpu-benchmark"
./cpu-benchmark
```

### 2. 轻量级单核跑分 (Python)
位于 `CPU&磁盘基准测试/CPU单核基准测试.py`。
*   **测试项**：通过质数查找与三角函数/平方根运算评估单核极限性能。
*   **输出**：类 Geekbench 评分。
```bash
python "CPU&磁盘基准测试/CPU单核基准测试.py"
```

---

## 💾 磁盘与 I/O 工具

### 1. 吞吐量极限测试
位于 `CPU&磁盘基准测试/磁盘IO速度测试.py`。创建临时文件，执行顺序写入 → 缓存清理 → 顺序读取，输出 B/KB/MB/GB/s 级别报告。
```bash
sudo python "CPU&磁盘基准测试/磁盘IO速度测试.py"
```

### 2. 实时 I/O 监控器
位于 `磁盘工具/disk_monitor.py`。基于 `iostat -x` 实现，实时刷新目标挂载点的读写速率与磁盘繁忙率（`%util`）。
```bash
python 磁盘工具/disk_monitor.py
```

---

## 🦊 ROS2 调试助手

位于 `ros2_helper/`。一个完整的 ROS2 话题交互式扫描与诊断终端工具。

*   **特性**：
    *   一键扫描所有活跃节点与话题。
    *   深度解析 QoS 配置（可靠性、深度、持久性等）。
    *   支持捕获并解析复杂消息（图像、IMU、雷达、里程计）。
    *   支持将节点拓扑与消息快照导出为 JSON/YAML。
*   **安装与运行**：
```bash
cd ros2_helper
pip install -r requirements.txt
pip install -e .
ros2-helper
```

---

## 🔌 USB 与外设扫描

位于 `USB_scan/`。跨平台的 USB 插拔监控与记录工具。

*   **特性**：Linux 基于 `pyudev`，Windows 基于 `WMI`。支持生成 Markdown 格式的设备快照日志。
*   **运行模式**：
```bash
cd USB_scan
# 生成当前设备快照
python main.py scan

# 实时监听设备插拔事件
python main.py monitor
```

---

## 🌐 局域网网络扫描

探测局域网内的其他设备（如上位机、从机、工控机等）。

### 1. 高级网络扫描框架 (`wlan_scan/`)
*   **特性**：子网与网关自动检测、高并发 ARP Ping、TCP 全端口/常用端口扫描、OUI 厂商识别。支持交互式菜单及 CSV/JSON 导出。
```bash
cd wlan_scan
pip install -r requirements.txt
python main.py --interactive
```

### 2. 轻量级扫描脚本 (`局域网设备扫描/`)
免去繁杂依赖，开箱即用的单文件脚本（区分 Linux 与 Windows 版本）。
```bash
python "局域网设备扫描/network_linux_v2.py"
```

---

## 📡 激光雷达调试

位于 `USB_scan/code_秦嘉伟_20260317/雷达/`。提供从底层串口读取到上层可视化的完整链路。

| 工具脚本 | 功能描述 |
| :--- | :--- |
| `COM3_link.py` | 建立串口连接，实时读取并打印原始十六进制数据。 |
| `decode.py` | 雷达底层协议解码，处理校验和，提取真实距离与角度。 |
| `display.py` | 基于 Pygame 的 2D 极坐标系，**实时 360° 渲染雷达点云**。 |

```bash
# 启动雷达实时可视化
python "USB_scan/code_秦嘉伟_20260317/雷达/display.py"
```

---

## 🧰 实用小脚本

### 目录结构净化打印工具
位于 `文件夹结构打印工具/tree_交互式输入路径.py`。
*   **特性**：替代原生 `tree` 命令。自动过滤开发过程中的噪音文件夹（如 `venv`, `__pycache__`, `node_modules`, `.git`, `build` 等），支持限定扫描深度。
```bash
python "文件夹结构打印工具/tree_交互式输入路径.py"
```

---

## ⚙️ 开发与配置规范

为了保持工具集的易用性和代码整洁，后续在修改或新增 Python 工具脚本时，请遵循以下规范：
*   **配置项前置**：脚本中如有需要用户修改的变量（如：雷达串口号 `COM3`/`/dev/ttyUSB0`、网络扫描超时时间、基准测试循环次数等），请统一将其作为**可配置参数提取到代码文件顶部，紧跟在导入库（`import`）的下方**。避免让使用者在代码逻辑深处寻找修改点。

## 💻 系统要求

- **核心运行环境**：Linux (Ubuntu / RDK 官方系统优先)
- **语言依赖**：Python 3.8+
- **中间件**：ROS2 Jazzy (仅 `ros2_helper` 强依赖)
- **编译工具**：`gcc/clang`, `make`, `zlib` (针对 C++ 基准测试模块)

## 📄 许可

本项目内置工具脚本主要供 RDK 小车及相关边缘计算设备的内部开发与调试使用。
