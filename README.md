# 提示词+语句自动合并工具 · Pro (Native)

基于原 Web 版的 Go + [Fyne](https://fyne.io) 原生桌面重写版。无浏览器内核、纯原生窗口、支持**浏览器风格多标签**，开箱即用，性能与可移植性更强。

## 主要差异（vs 原 Web 版）

| 项目 | 原 Web 版 | Native 版 |
|---|---|---|
| UI 载体 | 系统浏览器 | 原生窗口（Fyne + OpenGL） |
| 多标签 | ❌ 单视图 | ✅ 顶部自定义标签栏，可任意增删 |
| 主题 | 仅亮色 | 亮/暗一键切换，记忆到配置 |
| 配置格式 | `prompts`/`sentences` | `tabs[]`，自动兼容旧格式 |
| 部署 | 浏览器打开 `127.0.0.1:随机端口` | 单个 `.exe`，双击即用 |

## 功能

- **主界面布局**：左侧竖向「页面」导航（替代原历史侧栏）在**编辑器 / 模拟键盘输入 / 文件夹 Tree / AI 设置**之间切换；底部状态栏左侧是「⚙ 配置」菜单 + 状态文本，右侧是「AI 聊天」「Markdown」「LAN」三个图标按钮 + 标签数
- **多标签**：编辑器顶部标签栏右侧 `+` 新增，单击标签切换，`×` 关闭（最后一个标签的 `×` 退化为「清空此标签」）
- **每标签 5 行**：`提示词` / `语句` 两个多行文本框 + 两个独立复选框，文本/勾选实时联动合并
- **合并预览**：底部只读多行框实时显示结果，去抖 200 ms
- **去抖落盘**：编辑停顿 1.5 s 自动写 `config.json`，关闭窗口时同步落盘
- **模拟键盘输入**：独立页面，一键把任意标签的合并结果逐字「真实键入」到目标程序（绕过粘贴检测）。支持启动延迟 / 字符间隔 / 重复次数、实时进度与停止；**Enter 可屏蔽或替换为 Ctrl+Enter / 空格**，防止把换行直接发出去误触「发送」
- **文件夹 Tree**：独立页面，选择一个文件夹后按类 `tree` 缩进输出目录结构。可勾选屏蔽隐藏项（`.git` 等）与常见构建/依赖目录（`build`、`node_modules`…），并支持**添加要屏蔽的文件夹名**与**匹配关键词**（名称含即整棵忽略），结果一键复制
- **Markdown 渲染**：底部一个按钮即在本机**随机空闲端口**起本地服务并打开系统浏览器。页面为浏览器自含工具：左侧**粘贴 Markdown** 或**打开本地 .md/.txt**，右侧用内嵌 marked + highlight.js **实时 GitHub 风格预览**（代码高亮 + 复制，完全离线）；打开 `.pgm(+.yaml)` 则识别为 **2D 激光 SLAM 地图**，浏览器内提供**比例尺 / 测量尺子 / 障碍物擦除橡皮**，修改后可**保存下载** .pgm/.yaml；程序退出自动释放端口
- **局域网设备扫描**：底部一个按钮即在本机**随机空闲端口**起本地服务并自动用系统浏览器打开（复用独立「LAN Scan」的 Web 界面与探测逻辑），再次点击只重开浏览器。自动枚举网卡与子网、CIDR 快捷 /24-/26、并发/超时/端口可调；系统 ping（ICMP 被拦自动回退 TCP）+ 主机名 / MAC / 开放端口，表格**实时刷新**、支持**导出 CSV**；程序退出或关窗自动取消扫描并释放端口
- **AI 设置**：左侧页面栏新增「AI 设置」页——配置 OpenAI 兼容接口的 **API 地址 / API Key / 模型 / 系统提示词**，保存进 `config.json`（页面形态参考 RikkaHub 的提供商/模型设置思路）
- **AI 聊天（局域网）**：底部「AI 聊天」一个按钮即在本机随机空闲端口起服务并自动用系统浏览器打开。页面为浏览器自含对话（**布局/交互参考 RikkaHub Web UI**）：左侧会话列表（新建/切换/删除，本机 localStorage）、顶部模型下拉（自动读取上游 `/models`）、消息气泡 + **Markdown 渲染（代码高亮/复制）**、重新生成/复制、底部输入框（Enter 发送、Shift+Enter 换行、停止生成）；走 **OpenAI 兼容 `/chat/completions` 流式(SSE)**，可接 OpenAI / DeepSeek / Qwen / Moonshot / 本地 Ollama·LM Studio 等；程序退出自动释放端口
- **亮/暗主题**：左下角「⚙ 配置」菜单一键切换，记忆到配置
- **界面风格**：JetBrains / VSCode 取向的暗色 IDE 外观——深墨蓝底（#1E2333）、#2C3247 控件底、#3A96FF 强调色、#E1E4EB 柔和文本；左侧导航选中项为圆角渐变卡片，标签栏为 2 px 蓝色下划线，文本域等宽字体 + 4 px 圆角
- **配置迁移**：自动检测旧版 `prompts`/`sentences` 结构并迁移成单标签

## 合并规则（与原版一致）

对每一行（i = 0..4）：

| 提示词 | 语句 | 输出片段 |
|---|---|---|
| 启用且非空 | 启用且非空 | `提示词：\n语句` |
| 启用且非空 | 关闭或空 | `提示词` |
| 关闭或空 | 启用且非空 | `语句` |
| 关闭或空 | 关闭或空 | 跳过 |

最后把所有片段用 `，\n` 连接，末尾追加 `。`。

## 项目结构

```
提示词v2-native/
├── main.go                // 入口
├── app.rc                 // Windows 图标资源脚本（windres 使用）
├── app_icon_windows_amd64.syso // 由 default_icon.ico 生成的资源对象，go build 自动链接
├── ico/                   // 应用图标源文件
├── data/                  // 纯逻辑层（可单元测试）
│   ├── merger.go          // 合并规则
│   ├── merger_test.go     // 单元测试
│   ├── config.go          // 配置读写 + 旧格式迁移
│   ├── history.go         // 历史记录读写（遗留：新版 UI 已移除历史栏）
│   ├── tree.go            // 文件夹 tree 生成 + 屏蔽规则
│   └── tree_test.go       // 单元测试
├── render/                // 浏览器渲染服务（纯逻辑层，可单元测试）
│   ├── pgm.go             // PGM(P2/P5) 解析、yaml 元数据解析
│   ├── server.go          // 随机空闲端口监听 / 释放 / 打开系统浏览器
│   ├── markdown.go        // Markdown 页（内嵌 marked + highlight.js）
│   ├── mapview.go         // 2D SLAM 地图页（比例尺/测量/橡皮/下载）
│   ├── mdeditor.go        // 浏览器自含 Markdown/地图输入页 + /api/map 上传
│   ├── embed.go           // go:embed 内嵌前端静态资源
│   ├── render_test.go     // 单元测试
│   └── mdeditor_test.go   // 单元测试
├── netscan/               // 局域网扫描探测（改编自 MIT LAN Scan，纯标准库）
│   ├── scanner.go         // 并发扫描编排 + CIDR 展开
│   ├── ping.go            // 系统 ping / TCP 探测 / MAC / 反向 DNS
│   ├── interfaces.go      // 网卡与子网枚举
│   ├── doc.go             // 包说明与出处声明
│   └── scan_test.go       // 单元测试
├── lanscan/               // 浏览器版局域网扫描服务（复用 LAN Scan Web UI）
│   ├── embed.go           // go:embed 内嵌浏览器页面资源
│   ├── server.go          // HTTP / SSE 路由 + 扫描会话管理
│   ├── server_test.go     // 单元测试
│   └── assets/            // index.html / style.css / app.js
├── chat/                  // 浏览器版 AI 聊天（OpenAI 兼容流式）
│   ├── chat.go            // /api/config · /api/models · /api/chat(SSE 转发)
│   ├── page.go            // 对话页（参考 RikkaHub UI：会话列表/消息流/输入）
│   └── chat_test.go       // 单元测试
├── ui/                    // UI 层
│   ├── app.go             // 主窗口 + 左侧页面切换 + 多标签编排
│   ├── editor.go          // 单标签编辑器（5 行 + 预览 + 按钮）
│   ├── tabbar.go          // 自定义标签栏（Fyne 未公开 closable API，自己实现）
│   ├── leftrail.go        // 左侧竖向页面导航（替代原历史侧栏）
│   ├── browser_tools.go   // AI 聊天 / 局域网扫描 / Markdown 渲染的浏览器服务生命周期
│   ├── aisettings.go      // AI 设置页（API 地址/Key/模型/系统提示）
│   ├── theme.go           // 亮/暗自定义主题
│   ├── paste.go           // 标签填充小工具
│   ├── simpanel.go        // 模拟键盘输入独立页面
│   ├── simtype.go         // 按键序列规划（Enter 屏蔽 / 替换 Ctrl+Enter / 空格）
│   ├── simtype_windows.go // Windows SendInput 底层（Unicode 逐字键入）
│   ├── simtype_other.go   // 非 Windows 占位实现
│   ├── treepanel.go       // 文件夹 Tree 独立页面（屏蔽过滤设置 + 结果）
│   ├── treepanel_demo.go  // Tree 页截图演示入口
│   ├── simpanel_demo.go   // 模拟输入页截图演示入口
│   └── layout.go          // fixedHeightLayout：多行框最小高度裁剪布局
└── go.mod / go.sum
```

## 编译运行

```bash
# 拉依赖
go mod tidy

# （可选）打包 exe 图标：先由 ico/default_icon.ico 生成资源对象
# windres -J rc -O coff -o app_icon_windows_amd64.syso app.rc
# 之后 go build 会自动链接该 .syso（app_icon_windows_amd64.syso 已随仓库提供）

# 开发编译（带控制台，可看 log）
go build -o merger-native.exe .

# 正式编译（GUI 子系统，双击启动无黑色 cmd 框；含 .syso 时自动带图标）
go build -ldflags "-H windowsgui" -o merger-native.exe .

# 生产构建（带图标也可使用 fyne package）
fyne package -os windows -icon icon.png -name "提示词+语句合并工具"
```

> 需要 CGO（Windows 上需 MinGW gcc）。本机已验证 Go 1.25 + MinGW 16.1 可正常构建。
>
> 注意：直接用 `go build` 默认生成的是「控制台子系统」程序，双击会闪一个黑色 cmd 框；
> 正式分发给用户时应使用上面的 `-ldflags "-H windowsgui"`（等价于 `fyne package` 的效果）。

## 多平台打包

| 平台 | 产物 | 构建方式 |
|---|---|---|
| Windows x64 | `merger-native.exe` / `.zip` | 本机运行 `build.bat` |
| Windows x64、Linux amd64/arm64 | `fyne-cross/dist/` 下的安装包 | 安装 Docker 后运行 `build-multiplatform.bat` |
| macOS Intel、Apple Silicon | `.app.tar.gz` | 推送到 GitHub 后运行 `.github/workflows/build-multiplatform.yml` |

本机有 Docker 时，`build-multiplatform.bat` 会自动调用固定版本的 `fyne-cross`，
交叉构建 Windows amd64 和 Linux amd64/arm64。

macOS 不能在普通 Windows/Linux 主机上合法、完整地跨编译，因为它需要 Apple SDK 并受 Xcode 许可约束。
仓库中的 GitHub Actions 会在 `windows-latest`、`ubuntu-latest`、`macos-15` 和 `macos-15-intel` 上原生构建。
手动运行该 workflow 可下载各平台 Artifact；推送 `v*` 标签时还会自动创建 GitHub Release 并附加全部安装包。

Linux 端需要系统提供 X11/OpenGL 运行库；macOS 包当前未做 Apple 签名和公证，首次打开可能需要在 Finder 中右键选择“打开”。

## 性能与可靠性

- 文本输入 → 合并预览：**200 ms 去抖**，避免每键击都触发重算
- 编辑器 → 配置文件：**1.5 s 去抖**，短时间内多次修改只触发一次落盘
- 窗口关闭：`SetOnClosed` 同步 `flushAll()`，停掉所有定时器、浏览器服务并立刻同步落盘
- Fyne UI 更新全部走 `fyne.Do` 回到主线程
- 配置写入前保护定时器互斥锁

## 配置文件

| 文件 | 位置 | 说明 |
|---|---|---|
| `config.json` | 与 exe 同目录 | 多标签配置（含主题、激活标签、`ai` 段：API 地址/Key/模型/系统提示） |
| `FyneApp.toml` | 与 exe 同目录 | 首次启动自动生成（含线程模型迁移标记） |

`config.json` 的 `ai` 示例：

```json
{
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-...",
  "model": "gpt-4o-mini",
  "system_prompt": "你是一个贴心的助手"
}
```

> 旧版本自动追加的 `history.json`（历史记录侧栏）本版本已移除：左侧改竖向页面导航，合并结果不再自动记录。

## 已知限制

- Fyne v2.8 没有公开 `SetTabsClosable` / `OnClosed`，所以本项目自带 `customTabBar` 实现这些能力；如果你升级到更高版本（公开这些 API 后），可以把 `tabbar.go` 替换成 `container.NewAppTabs` + `SetTabsClosable`
- 没有图标和托盘（`fyne package` 时可补）
- 没有应用内「设置」面板——主题切换/数据目录靠 `config.json` 修改
