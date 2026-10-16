package ui

import (
	"fyne.io/fyne/v2"

	"prompt-merger-native/data"
)

// DemoSimPanelObject 供 cmd/screenshot 无头渲染「模拟键盘输入」页面快照用。
// 构造一份带示例数据的来源标签，并在页面上预填一段含换行/中文的合并内容。
func DemoSimPanelObject() fyne.CanvasObject {
	cfg := data.DefaultConfig()
	cfg.Tabs = cfg.Tabs[:1]
	cfg.Tabs[0].Prompts[0] = data.Item{Text: "需求说明", Enabled: true}
	cfg.Tabs[0].Sentences[0] = data.Item{Text: "加一个开关，启用后只在本地匹配", Enabled: true}
	cfg.Tabs[0].Prompts[1] = data.Item{Text: "模拟键入", Enabled: true}
	cfg.Tabs[0].Sentences[1] = data.Item{Text: "逐字真实键入，可屏蔽 Enter 防误发", Enabled: true}

	bundles := []*tabBundle{{editor: NewTabEditor("标签 1", cfg.Tabs[0], nil)}}
	p := NewSimInputPanel(func() {})
	p.SetBundles(bundles, 0)
	p.txt.SetText("需求说明：\n加一个开关，启用后只在本地匹配\n\n模拟键入：\n逐字真实键入，可屏蔽 Enter 防误发")
	p.View().Show()
	return p.View()
}
