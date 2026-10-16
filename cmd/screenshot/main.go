// 真实渲染快照工具：把主窗口搬到 software canvas 上截图，
// 方便无显示器环境下做 UI 验收。
package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/software"
	"fyne.io/fyne/v2/test"

	"prompt-merger-native/data"
	"prompt-merger-native/ui"
)

func snap(name string, cfg data.Config, outDir string, w, h int) error {
	a := test.NewApp()
	a.Settings().SetTheme(ui.NewModernTheme(cfg.ThemeMode == "dark"))

	win := ui.NewMainWindow(a, &cfg)

	c := software.NewCanvas()
	c.SetContent(win.Canvas().Content())
	c.Resize(fyne.NewSize(float32(w), float32(h)))
	time.Sleep(200 * time.Millisecond)

	img := c.Capture()
	if img == nil {
		return fmt.Errorf("Capture nil for %s", name)
	}
	path := filepath.Join(outDir, name)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return err
	}
	fmt.Println("wrote", path)
	return nil
}

// makeHistory 写入指定目录的 history.json，返回该目录路径。
func makeHistory(dir string) string {
	_ = os.MkdirAll(dir, 0755)
	entries := []data.HistoryEntry{
		{Content: "待修改的代码：\n加一个开关，启用后只在 2D pose est 附近 x 米匹配", Timestamp: "12:30:01"},
		{Content: "修改方法：\n在 map_matcher 上方加 USE_LOCAL_MAP 开关", Timestamp: "12:31:18"},
		{Content: "其它要求：\n保留原 API，给出完整 diff", Timestamp: "12:32:05"},
	}
	_ = data.SaveHistory(entries, dir)
	return dir
}

func main() {
	outDir := "screenshots"
	_ = os.MkdirAll(outDir, 0755)

	// 1 个标签 / 亮色 / 1240x820
	c1 := data.DefaultConfig()
	c1.Tabs = c1.Tabs[:1]
	_ = snap("01_one_tab_light_1240x820.png", c1, outDir, 1240, 820)

	// 3 个标签 / 亮色 / 1240x820
	_ = snap("02_three_tabs_light_1240x820.png", data.DefaultConfig(), outDir, 1240, 820)

	// 10 个标签 / 亮色 / 1240x820
	c10 := data.DefaultConfig()
	for len(c10.Tabs) < 10 {
		c10.Tabs = append(c10.Tabs, data.TabConfig{Prompts: data.DefaultItems(), Sentences: data.DefaultItems()})
	}
	_ = snap("03_ten_tabs_light_1240x820.png", c10, outDir, 1240, 820)

	// 小窗
	_ = snap("04_three_tabs_light_900x600.png", data.DefaultConfig(), outDir, 900, 600)

	// 大窗
	_ = snap("05_three_tabs_light_1920x1080.png", data.DefaultConfig(), outDir, 1920, 1080)

	// 暗色
	cd := data.DefaultConfig()
	cd.ThemeMode = "dark"
	_ = snap("06_three_tabs_dark_1240x820.png", cd, outDir, 1240, 820)

	// 含内容 + 历史
	histDir := filepath.Join(outDir, "hist_demo")
	makeHistory(histDir)
	c7 := data.DefaultConfig()
	c7.DataDir = histDir
	if len(c7.Tabs) > 0 {
		c7.Tabs[0].Prompts[0] = data.Item{Text: "待修改的代码", Enabled: true}
		c7.Tabs[0].Sentences[0] = data.Item{Text: "加一个开关，启用后只在 2D pose est 附近 x 米匹配", Enabled: true}
		c7.Tabs[0].Prompts[1] = data.Item{Text: "修改方法", Enabled: true}
		c7.Tabs[0].Sentences[1] = data.Item{Text: "在 map_matcher 上方加 USE_LOCAL_MAP 开关", Enabled: true}
		c7.Tabs[0].Prompts[2] = data.Item{Text: "其它要求", Enabled: true}
		c7.Tabs[0].Sentences[2] = data.Item{Text: "保留原 API，给出完整 diff", Enabled: true}
	}
	_ = snap("07_three_tabs_filled_light_1240x820.png", c7, outDir, 1240, 820)

	// 12. 含内容 / 暗色：验收只读预览区与输入区的对比度
	c12 := c7
	c12.ThemeMode = "dark"
	_ = snap("12_three_tabs_filled_dark_1240x820.png", c12, outDir, 1240, 820)

	// 8/9. 模拟键盘输入独立页（亮/暗）
	snapDemo("08_sim_panel_light_1240x820.png", false, ui.DemoSimPanelObject, outDir)
	snapDemo("09_sim_panel_dark_1240x820.png", true, ui.DemoSimPanelObject, outDir)

	// 10/11. 文件夹 Tree 独立页（亮/暗）
	snapDemo("10_tree_panel_light_1240x820.png", false, ui.DemoTreePanelObject, outDir)
	snapDemo("11_tree_panel_dark_1240x820.png", true, ui.DemoTreePanelObject, outDir)
	// 13/14. AI 设置独立页（亮/暗）
	snapDemo("13_ai_settings_light_1240x820.png", false, ui.DemoAISettingsObject, outDir)
	snapDemo("14_ai_settings_dark_1240x820.png", true, ui.DemoAISettingsObject, outDir)
}

// snapDemo 把任意演示页面对象渲染到 software canvas 并存成 PNG。
//
// 注意：主题必须在 makeObj() 之前设置，否则控件会沿用上一个主题的颜色。
func snapDemo(name string, dark bool, makeObj func() fyne.CanvasObject, outDir string) {
	a := test.NewApp()
	a.Settings().SetTheme(ui.NewModernTheme(dark))

	obj := makeObj()

	c := software.NewCanvas()
	c.SetContent(obj)
	c.Resize(fyne.NewSize(1240, 820))
	time.Sleep(200 * time.Millisecond)
	path := filepath.Join(outDir, name)
	f, err := os.Create(path)
	if err == nil {
		defer f.Close()
		if err := png.Encode(f, c.Capture()); err == nil {
			fmt.Println("wrote", path)
		}
	}
}
