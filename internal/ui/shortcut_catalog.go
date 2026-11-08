package ui

import (
	"sort"

	"fyne.io/fyne/v2"
)

// shortcutEntry 是一条可展示的快捷键说明。
type shortcutEntry struct {
	Group string       // 分组：标签 / 区块 / 编辑 / 窗口
	Label string       // 动作说明（中文）
	Key   fyne.KeyName // 主键
	Mod   fyne.KeyModifier
}

// shortcutName 返回「Ctrl+Shift+N」这类可读组合。
func (s shortcutEntry) shortcutName() string {
	return (&desktopShortcut{key: s.Key, mod: s.Mod}).ShortcutName()
}

// shortcutCatalog 是全部快捷键的唯一事实来源：
// 右键菜单的提示文字、F1 快捷键一览都从这里读，避免两处漂移。
var shortcutCatalog = []shortcutEntry{
	{"标签", "新建标签", fyne.KeyN, fyne.KeyModifierControl},
	{"标签", "关闭当前标签", fyne.KeyW, fyne.KeyModifierControl},
	{"标签", "下一个 / 上一个标签", fyne.KeyTab, fyne.KeyModifierControl},
	{"标签", "切换到第 1–9 个标签", fyne.Key1, fyne.KeyModifierControl},
	{"标签", "重命名当前标签", fyne.KeyR, fyne.KeyModifierControl | fyne.KeyModifierShift},
	{"标签", "保存配置", fyne.KeyS, fyne.KeyModifierControl},
	{"区块", "新增区块", fyne.KeyN, fyne.KeyModifierControl | fyne.KeyModifierShift},
	{"区块", "区块上移", fyne.KeyUp, fyne.KeyModifierAlt},
	{"区块", "区块下移", fyne.KeyDown, fyne.KeyModifierAlt},
	{"编辑", "复制合并结果", fyne.KeyEnter, fyne.KeyModifierControl},
	{"编辑", "聚焦当前输入框", fyne.KeyF, fyne.KeyModifierControl},
	{"编辑", "定位标签名", fyne.KeyL, fyne.KeyModifierControl},
	{"窗口", "打开 AI 聊天", fyne.KeyJ, fyne.KeyModifierControl},
	{"窗口", "打开 Markdown 渲染", fyne.KeyM, fyne.KeyModifierControl | fyne.KeyModifierShift},
	{"窗口", "打开局域网扫描", fyne.KeyL, fyne.KeyModifierControl | fyne.KeyModifierShift},
	{"窗口", "快捷键一览", fyne.KeyF1, 0},
}

// shortcutNameOf 返回某条目���对应的快捷键提示文字；找不到时返回空串。
func shortcutNameOf(label string) string {
	for _, s := range shortcutCatalog {
		if s.Label == label {
			return s.shortcutName()
		}
	}
	return ""
}

// shortcutHelp 生成按分组排列的「动作：组合键」文本行。
func shortcutHelp() [][2]string {
	groups := map[string][]shortcutEntry{}
	order := make([]string, 0, 4)
	for _, s := range shortcutCatalog {
		if _, ok := groups[s.Group]; !ok {
			order = append(order, s.Group)
		}
		groups[s.Group] = append(groups[s.Group], s)
	}
	sort.SliceStable(order, func(i, j int) bool {
		rank := func(g string) int {
			switch g {
			case "标签":
				return 0
			case "区块":
				return 1
			case "编辑":
				return 2
			default:
				return 3
			}
		}
		return rank(order[i]) < rank(order[j])
	})
	out := make([][2]string, 0, len(shortcutCatalog)+len(order))
	for _, g := range order {
		for _, s := range groups[g] {
			out = append(out, [2]string{s.Label, s.shortcutName()})
		}
	}
	return out
}
