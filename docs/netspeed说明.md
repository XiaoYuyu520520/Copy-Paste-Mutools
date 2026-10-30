# NetSpeed — 局域网测速工具

自托管网页版网络测速工具，Go + Vue 3 构建，单文件部署，跨平台运行。

## 快速开始

```bash
# 运行预编译二进制
./bin/netspeed

# 或构建后运行
cd src && bash build.sh && ./build/netspeed

# 打开浏览器访问 http://localhost:7777
```

## 功能特性

- **三种测速模式**：下载测速 / 上传测速 / 双向同时测速
- **多线程传输**：1/2/4/8 线程并行，更准确压榨带宽
- **实时仪表盘**：200ms 窗口瞬时速度显示
- **测速时长可选**：5s / 10s / 30s
- **历史记录**：保存最近 50 条结果，刷新不丢失
- **单文件部署**：前后端打包为一个可执行文件，零依赖

## 预编译二进制

| 文件 | 平台 |
|------|------|
| `bin/netspeed-windows-amd64.exe` | Windows x86_64 |
| `bin/netspeed-linux-amd64` | Linux x86_64 |
| `bin/netspeed-linux-arm64` | Linux ARM64（树莓派 / Jetson） |
| `bin/netspeed` | 当前编译平台 |

## 技术栈

- **后端**：Go + Gorilla WebSocket
- **前端**：Vue 3 + Vite
- **传输**：WebSocket（JSON 控制消息 + 二进制测速载荷）

## 详细文档

完整的构建说明、架构设计、API 协议见 [`src/README.md`](src/README.md)。
