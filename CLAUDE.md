# CLAUDE.md

本文件给 Claude Code 在本仓库工作时使用。回答与代码注释、提交信息、文档一律用**中文**。

## 项目是什么

提示词复合工具箱（Copy-Paste-Mutools）：Go 1.25 + Fyne v2.8 的多合一原生工具箱，
由旧 Python 脚本集迁移而来（`legacy/python/` 只读参考，不参与构建）。
一个代码库产出 4 个程序 + 一套网页工具：

| 程序 | 入口 | 说明 |
| --- | --- | --- |
| `merger` | `cmd/merger` | 桌面端（Fyne）：提示词合并、模板、模拟键入、文件夹树、Markdown、地图、局域网扫描、AI 聊天 |
| `toolbox` | `cmd/toolbox` | 单一 exe 承载全部 CLI 子命令（`internal/toolbox/cli` 分发），纯 Go、不依赖 cgo |
| `paint` | `cmd/paint` | 独立画板 |
| Android | `cmd/android` + `internal/mobile` | 手机端 5 页（编辑 / 聊天 / 扫描 / 画板 / 更多），由 `.github/workflows/android.yml` 构建 |
| 网页工具 | `docs/tools/*.html` | 部署在 GitHub Pages（`main` 分支 `/docs`，legacy 构建） |

`cmd/checktest`、`cmd/mkicon`、`cmd/screenshot` 是开发辅助工具，不属于发布产物。

## 包结构

```
internal/
├── data/      # 纯逻辑：拼接规则、配置/模板/历史读写（原子写盘）、目录树。改动优先写单测
├── ai/        # 统一 OpenAI 兼容流式客户端（SSE 解析）。桌面 chat 与 Android mobile 共用
├── chat/      # 桌面端 AI 聊天页
├── theme/     # 桌面端与 Android 共用的调色板与 Fyne 主题（两端观感一致）
├── ui/        # 桌面端 Fyne 界面层（体量最大）
├── mobile/    # Android 界面层 + 本地存储
├── render/    # Markdown / PGM 地图渲染服务（Go 原始字符串页面 + 内嵌 JSON + 本地 HTTP）
├── netscan/   # 跨平台局域网探测引擎（ICMP/TCP/MAC/主机名）
├── lanscan/   # 浏览器版局域网扫描服务
├── netspeed/  # 网速测试服务（gorilla/websocket + 内嵌前端）
└── toolbox/   # CLI 各子命令；toolbox/webassets 为 go:embed 前端资源
```

平台差异一律走 `_windows.go` / `_linux.go` / `_other.go` 构建标签（见 `internal/ui/platform_*`、`internal/toolbox/usb/`、`internal/netscan/proc_*` 等），不要在业务代码里散落 `runtime.GOOS` 分支。

## 本机工具链（重要）

Git Bash 的 PATH **没有** Go 和 MinGW，直接 `go build` 会报 `go: command not found`。
每次跑 Go 命令前先加上：

```bash
export PATH="/c/Program Files/Go/bin:/ucrt64/bin:$PATH"
```

- Go：`C:\Program Files\Go`（go1.25.4）
- gcc：`/ucrt64/bin/gcc`（MSYS2 UCRT64，`x86_64-w64-mingw32`）。
  **不能用** PATH 里的 `/usr/bin/gcc`——那是 Cygwin 版，cgo 会报
  `don't use the cygwin compiler to build native Windows programs`。
- 用户日常在 PowerShell 7 + 虚拟环境 `E:\venvs\py312`（`& "E:\venvs\py312\Scripts\Activate.ps1"`）；
  该环境里 `go` 可 `Get-Command` 到，但当前 Claude 会话的 PowerShell 子进程继承不到它的 PATH。
- MSYS2 入口：`D:\WindowsServer2025_1\software\MSYS2\ucrt64.exe`；tmux（psmux）：`D:\WindowsServer2025_1\software\psmux-v3.3.6-windows-x64\tmux.exe`
- pip 走清华源：`pip install -i https://mirrors.tuna.tsinghua.edu.cn/pypi/web/simple <pkg>`

## 常用命令

```bash
export PATH="/c/Program Files/Go/bin:/ucrt64/bin:$PATH"
go build ./...
go vet ./...
go test ./...
go test -race ./internal/data/... ./internal/netscan/... ./internal/lanscan/... ./internal/mobile/... ./internal/ai/... ./internal/chat/...
gofmt -l ./cmd ./internal            # 必须为空
python tests/web/test_tools_smoke.py # Playwright 网页冒烟测试
```

产出三件套用 `build.bat`（Windows，输出到 `bin\`，GUI 程序带 `-H windowsgui`）；
多平台打包用 `build-multiplatform.bat`（依赖 Docker + fyne-cross）。

## 硬性约定

### 1. 落盘一律用原子写入

不要直接 `os.WriteFile` 写配置/状态 JSON：

```go
data.SaveJSONAtomic(path, v)        // .tmp → 旧文件 → .bak → 原子 rename
data.SaveFileAtomic(path, bytes)
ok, usedBackup := data.LoadJSONWithBackup(path, &v) // 损坏时回退 .bak
```

### 2. 保存失败必须反馈，不能吞错

桌面端「立即保存」唯一入口是 `MainApp.saveWithFeedback()`（`internal/ui/app.go`），
Ctrl+S 走它；`OnSave` / `OnSaveProviders` 签名返回 `error`，`AISettingsPanel.Save()`
只调 `OnSaveProviders` 一次。
Android 用 `store.SetSaveResultHandler` + `SaveState.Label()` 显示
「有未保存的改动 / 正在保存… / 已保存 / 保存失败」。

反模式：`m.saveConfigNow()` 后无条件提示“已保存”、`_ = s.Save()` 吞错。

### 3. AI 请求只走 `internal/ai`

新增任何 AI 调用都调 `ai.Stream` / `ai.StreamWithOptions` / `ai.ListModels`，
消息整理用 `ai.BuildMessages`。不要在 UI 包重写 SSE 解析。
已有客户端处理：多行 `data:` 拼接、`reasoning_content`/`reasoning`、非流式回退、
`scanner.Err()` 上报、ctx 取消、`[DONE]`/注释跳过、`{"error":...}` 转换。

### 4. 可增删列表项用稳定 ID，不用下标

服务商等列表必须用 `AIProvider.ID`；删选用 `Config.DeleteProvider(idx)`（内部按 ID 保持选中），
整体替换用 `Config.SelectProvidersByID(list, activeID)`。

### 5. 存储位置

`data.StorageDir()`：默认系统用户配置目录（`prompt-merger-toolbox`），
设 `PROMPT_MERGER_PORTABLE=1` 或 exe 旁放 `portable.txt` 切到便携模式。
不要假设可写 `Program Files`。

### 6. 安全

- 根目录 `我的Github令牌-千万不要上传.txt` 由 `.gitignore` 的 `*token*.txt` 忽略，**保持此状态**。
  令牌只通过 `TOKEN=$(head -1 ...)` 读入，不要打印到输出、写进代码/文档/提交信息。
- git over HTTPS 用 PAT 时必须 Basic 认证；`Authorization: Bearer` 会 `invalid credentials`。
- 网页工具全部本地处理，不引入会外传数据的 CDN / 上报脚本；外部库下载后内置到 `docs/tools/`。

## 测试

- 每个 `internal/` 逻辑包都有 `*_test.go`；`internal/ui` 用 Fyne 的 `test` 包，
  `TestMain` 已处理 CI 的 `LANG=C` 与单线程渲染，不要删。
- `internal/render` 的 PGM 用 `tests/data/*.pgm` 样本（P2/P5、8/16 位、不同 maxv）。
- 网页工具：`tests/web/test_tools_smoke.py`（Playwright）；改 `docs/tools/` 后必须跑。
- 并发相关改动（配置快照、扫描取消、流式刷新）要能过 `-race`。
- httptest 模拟 SSE 时，“读取中断”用例要用 `http.Hijacker` 手动断开，
  否则 httptest 会补结尾、测不出中断。

## 网页工具（docs/tools/）

- 自包含 HTML + 公共 `common.css` / `common.js`；各页顶栏导航保持统一。
- 新增工具要同时更新：`docs/tools/index.html`（总览页）、各页导航、`docs/index.html`、`docs/index.md`、README。
- 改完先 `node --check`（内联脚本提取后再 check 一次）。
- `docs/tools/assets/` 与 `internal/render/assets/` 保持同源。
- Pages 是分支自动构建，**没有** deploy workflow，推送 `main` 的 `docs/` 即生效，不要加部署 job。

## 文档与记忆

- 用户可见文档在 `docs/`（GitHub Pages 源），README 是入口总览；功能改动要同步对应文档。
- 每完成一部分工作，按用户约定把**事实与结论**写入 `.memories/`，**可复用方法**写入 `.skills/`
  （这两个目录目前未纳入 git）。
- 命令超时不要设为 300s/3000s 这类超长值；长任务用后台方式启动并定期查看进度。

## 提交

提交信息用 Conventional Commits 前缀，中文正文，例如：
`fix: 统一写盘原子性、AI 流式解析与保存反馈，补齐网页端测试`。
