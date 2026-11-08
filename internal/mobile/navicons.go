package mobile

import (
	"fmt"
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
)

// 内嵌的底栏图标（24×24，Material 风格轮廓）。
//
// Fyne 的 widget.Icon 不允许单独改颜色，而底栏需要「选中 = 强调色、
// 未选中 = 次要色」。这里把同一份路径按颜色生成 StaticResource 并缓存，
// 切换选中态时只换资源，不重建控件。
var navIconPath = map[string]string{
	"edit":  "M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04a1 1 0 0 0 0-1.41l-2.34-2.34a1 1 0 0 0-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z",
	"chat":  "M20 2H4a2 2 0 0 0-2 2v18l4-4h14a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2zM7 9h10v2H7V9zm7 5H7v-2h7v2zm3-6H7V6h10v2z",
	"scan":  "M15.5 14h-.79l-.28-.27a6.5 6.5 0 1 0-.7.7l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0A4.5 4.5 0 1 1 14 9.5 4.5 4.5 0 0 1 9.5 14z",
	"paint": "M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04a1 1 0 0 0 0-1.41l-2.34-2.34a1 1 0 0 0-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83zM2 24h24v-4H2v4z",
	"more":  "M6 10a2 2 0 1 0 0 4 2 2 0 0 0 0-4zm6 0a2 2 0 1 0 0 4 2 2 0 0 0 0-4zm6 0a2 2 0 1 0 0 4 2 2 0 0 0 0-4z",
}

var (
	navIconMu    sync.Mutex
	navIconCache = map[string]fyne.Resource{}
)

// navIcon 返回指定名称、指定颜色的图标资源（带缓存）。
func navIcon(name string, fill color.Color) fyne.Resource {
	r, g, b, _ := fill.RGBA()
	key := fmt.Sprintf("%s-%02x%02x%02x", name, r>>8, g>>8, b>>8)

	navIconMu.Lock()
	defer navIconMu.Unlock()
	if res, ok := navIconCache[key]; ok {
		return res
	}

	path, ok := navIconPath[name]
	if !ok {
		path = navIconPath["more"]
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
<path fill="rgb(%d,%d,%d)" d="%s"/>
</svg>`, r>>8, g>>8, b>>8, path)
	res := fyne.NewStaticResource("nav-"+key+".svg", []byte(svg))
	navIconCache[key] = res
	return res
}
