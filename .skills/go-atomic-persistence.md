# 技能：Go 桌面应用的数据写入可靠性

适用：本仓库（Go + Fyne 桌面 + Android）所有需要落盘的配置 / 状态文件。

## 1. 统一用 data.SaveJSONAtomic，不要直接 os.WriteFile

```go
import "github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"

if err := data.SaveJSONAtomic(path, obj); err != nil { /* 反馈给用户 */ }
```

内部流程（`internal/data/atomic.go`）：

1. `os.WriteFile(path + ".tmp", data)` —— 先写同目录临时文件；
2. `os.Rename(path, path+".bak")` —— 旧文件留一份备份；
3. `os.Rename(path + ".tmp", path)` —— 同目录 rename 是原子的。

好处：
- 写入过程中断电 / 进程被杀，目标文件要么是旧完整内容、要么是新完整内容；
- 新内容损坏时 `.bak` 仍可恢复；
- 不留 `.tmp` 残留。

读取时用 `data.LoadJSONWithBackup(path, &obj)`：主文件缺失或 JSON 损坏时
自动回退 `.bak`，返回 `usedBackup` 供上层提示。

## 2. 写入失败必须反馈给用户

桌面端：
- 「立即保存」统一走 `MainApp.saveWithFeedback()`（`internal/ui/app.go`）；
- AI 设置的 `OnSave` / `OnSaveProviders` 回调签名是 `func(...) error`；
  `AISettingsPanel.Save()` 只调用 `OnSaveProviders` 一次，避免重复写盘。

Android：
- `store.Save()` 返回 error；去抖保存用 `store.SetSaveResultHandler(func(error))`
  回调上报，`store.SaveStatus()` 返回 `(SaveState, detail)`；
- 界面用 `SaveState.Label()` 显示「有未保存的改动 / 正在保存… / 已保存 / 保存失败」。

反模式（不要再出现）：
```go
m.saveConfigNow()          // 丢弃 error，随后无条件 setStatus("配置已保存")
_ = s.Save()               // 定时器里静默吞掉写盘错误
```

## 3. 列表项选中不要依赖下标

服务商等「可增删的列表项」必须有稳定唯一 ID：

- `data.AIProvider.ID`（旧配置用 `providerIDFrom(baseURL, model, name)` 补齐）；
- 删除用 `Config.DeleteProvider(idx)`，内部按 ID 保持选中；
- 整体替换用 `Config.SelectProvidersByID(list, activeID)`。

## 4. 本机编译 Fyne（CGO）

Windows 上 PATH 里的 `/usr/bin/gcc` 是 Cygwin 版，cgo 会失败并报
`don't use the cygwin compiler to build native Windows programs`。

用 MSYS2 UCRT64（已装在 `/ucrt64`）：

```bash
PATH="/ucrt64/bin:$PATH" CC=gcc go build ./...
PATH="/ucrt64/bin:$PATH" CC=gcc go test ./...
PATH="/ucrt64/bin:$PATH" CC=gcc go test -race ./internal/...
```

`build.bat` / `build-multiplatform.bat` 依赖 PATH 里有 MinGW 的 gcc。
