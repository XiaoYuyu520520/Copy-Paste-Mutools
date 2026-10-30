package ui

import "fyne.io/fyne/v2"

// DemoTreePanelObject 供 cmd/screenshot 无头渲染「文件夹 Tree」页面快照。
// 直接以当前进程工作目录为根做一次带示例过滤的扫描。
func DemoTreePanelObject() fyne.CanvasObject {
	p := NewTreePanel(nil, func() {})
	p.addExtra("migrations")
	p.addKeyword("demo")
	p.setFolderSync(".")
	p.View().Show()
	return p.View()
}
