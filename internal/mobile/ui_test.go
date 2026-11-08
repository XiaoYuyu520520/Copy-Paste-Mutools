package mobile

import (
	"os"
	"runtime"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/test"

	"github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/data"
	shared "github.com/XiaoYuyu520520/Copy-Paste-Mutools/internal/theme"
)

// TestMain 与 internal/ui 保持一致：CI 的 LANG=C 会让 Fyne 报
// "Error parsing user locale C"，并在 CJK 文本测量时触发字体库 panic。
func TestMain(m *testing.M) {
	if v := os.Getenv("LANG"); v == "" || v == "C" || v == "POSIX" {
		_ = os.Setenv("LANG", "en_US.UTF-8")
	}
	if v := os.Getenv("LC_ALL"); v == "" || v == "C" || v == "POSIX" {
		_ = os.Setenv("LC_ALL", "en_US.UTF-8")
	}
	runtime.GOMAXPROCS(1)
	lang.SystemLocale()
	test.NewApp()

	os.Exit(m.Run())
}

// TestBottomNavSelect 底栏切换选中项：只有被选中的入口显示底色。
func TestBottomNavSelect(t *testing.T) {
	got := -1
	nav := newBottomNav(navEntries, func(i int) { got = i })

	nav.Select(2)
	for i, b := range nav.items {
		if want := i == 2; b.bg.Visible() != want {
			t.Errorf("入口 %d 底色可见 = %v，期望 %v", i, b.bg.Visible(), want)
		}
	}

	// 点击入口应回调对应下标
	nav.items[4].root.(*fyne.Container).Objects[1].(*tapLayer).Tapped(nil)
	if got != 4 {
		t.Errorf("点击入口 4 回调下标 = %d", got)
	}
}

// TestBottomNavEntriesCoverPages 导航入口数必须与页面数一致，
// 否则 switchPage 会越界或被挡住。
func TestBottomNavEntriesCoverPages(t *testing.T) {
	a := test.NewApp()
	win := New(a)
	defer win.Close()

	m := latestMainApp
	if m == nil {
		t.Fatal("latestMainApp 未设置")
	}
	if len(navEntries) != len(m.stack.Objects) {
		t.Fatalf("导航入口 %d 个，页面 %d 个", len(navEntries), len(m.stack.Objects))
	}
	if len(navEntries) < 5 {
		t.Fatalf("导航入口应至少 5 个，实际 %d", len(navEntries))
	}
}

// TestSwitchPageShowsOnePage 切换页面后只有目标页可见。
func TestSwitchPageShowsOnePage(t *testing.T) {
	a := test.NewApp()
	win := New(a)
	defer win.Close()

	m := latestMainApp
	for i := range navEntries {
		m.switchPage(i)
		for j, obj := range m.stack.Objects {
			if want := i == j; obj.Visible() != want {
				t.Errorf("切到第 %d 页后，第 %d 页可见 = %v", i, j, obj.Visible())
			}
		}
		if m.pageIdx != i {
			t.Errorf("pageIdx = %d，期望 %d", m.pageIdx, i)
		}
	}
}

// TestMobileUsesSharedTheme 移动端主题与桌面端共用调色板。
func TestMobileUsesSharedTheme(t *testing.T) {
	a := test.NewApp()
	cfg := data.DefaultConfig()
	cfg.ThemeMode = "dark"
	applyTheme(a, &cfg)

	pal, ok := shared.PaletteOf(a.Settings().Theme())
	if !ok {
		t.Fatal("移动端主题不是 internal/theme 的实现")
	}
	if pal.Accent != shared.Dark().Accent {
		t.Error("暗色主题强调色与共享调色板不一致")
	}

	cfg.ThemeMode = "light"
	applyTheme(a, &cfg)
	pal, _ = shared.PaletteOf(a.Settings().Theme())
	if pal.Bg != shared.Light().Bg {
		t.Error("亮色主题背景与共享调色板不一致")
	}
}

// TestSaveBarReflectsFailure 保存失败时必须出现在全局状态条上（不限编辑页）。
func TestSaveBarReflectsFailure(t *testing.T) {
	a := test.NewApp()
	win := New(a)
	defer win.Close()

	m := latestMainApp
	if m.saveBar == nil {
		t.Fatal("缺少全局保存状态条")
	}
	// 路径置空会走 data.SaveFileAtomic 的「保存路径为空」错误分支，
	// 用来验证失败反馈（Windows 上把路径指向目录并不会失败）。
	m.store.mu.Lock()
	m.store.path = ""
	m.store.mu.Unlock()

	err := m.store.Save()
	if err == nil {
		t.Fatal("路径为空时保存应当失败")
	}
	m.updateSaveBar(err)
	if m.saveBar.Text == "" {
		t.Fatal("保存失败后状态条不应为空")
	}
	if !m.saveBar.TextStyle.Bold {
		t.Error("保存失败应加粗提示")
	}
}
