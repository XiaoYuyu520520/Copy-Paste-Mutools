"""网页版工具的 Playwright 冒烟与功能测试。

覆盖：
  - 页面能打开、主要按钮存在且可点击、无 JS 运行时错误
  - Base64：UTF-8 往返转换、URL-safe、非法输入的错误提示
  - 二维码：文本生成 + PNG/SVG 下载
  - Markdown：导入渲染、代码高亮、左右对照、主题切换
  - 编解码检测：异常 API 情况下的降级不崩
  - 2D 激光地图：擦除障碍物后，连续改变窗口尺寸 / 缩放 / 平移，
    导出的 PGM 必须保留编辑内容（不退回原图）
  - PGM 灰度归一化：P2/P5、8/16 位、不同 maxval 必须与 Go 版一致

运行：
    python -m pytest tests/web/test_tools_smoke.py -v
    # 或
    python tests/web/test_tools_smoke.py

依赖：playwright（pip install playwright）。
若本机未下载 Playwright 自带浏览器，可用系统 Chrome：
    PLAYWRIGHT_CHROME_PATH 环境变量指定 chrome.exe 路径。
"""

from __future__ import annotations

import base64
import os
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
TOOLS = ROOT / "docs" / "tools"
SAMPLES = ROOT / "tests" / "data"

try:
    from playwright.sync_api import expect, sync_playwright
except ImportError:  # pragma: no cover - 环境未安装时跳过
    print("未安装 playwright，跳过网页端测试", file=sys.stderr)
    raise SystemExit(0)


def _chrome_path() -> str | None:
    env = os.environ.get("PLAYWRIGHT_CHROME_PATH")
    if env and os.path.exists(env):
        return env
    for cand in (
        r"C:\Program Files\Google\Chrome\Application\chrome.exe",
        r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
        "/usr/bin/google-chrome",
        "/usr/bin/chromium",
    ):
        if os.path.exists(cand):
            return cand
    return None


def _launch(p):
    path = _chrome_path()
    if path:
        return p.chromium.launch(executable_path=path)
    return p.chromium.launch()


def _page_url(name: str) -> str:
    return "file:///" + str(TOOLS / name).replace(os.sep, "/")


def _drop_pgm(page, w: int = 40, h: int = 30, maxv: int = 255) -> None:
    """构造一个 PGM 并以「拖放」方式送入页面（绕过 file input 的限制）。"""
    px = bytearray()
    for y in range(h):
        for x in range(w):
            px.append(0 if (x in (5, 6, 7) and y in (5, 6)) else maxv)
    head = f"P5\n{w} {h}\n{maxv}\n"
    data = head.encode() + bytes(px)
    page.evaluate(
        """async (arg) => {
            const bin = atob(arg.b64);
            const bytes = new Uint8Array(bin.length);
            for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
            const file = new File([bytes], arg.name, { type: 'image/x-portable-anymap' });
            const ev = new Event('drop', { bubbles: true, cancelable: true });
            ev.dataTransfer = { files: [file] };
            window.dispatchEvent(ev);
        }""",
        {"b64": base64.b64encode(data).decode(), "name": "test_map.pgm"},
    )
    page.wait_for_timeout(500)


def test_map_edit_survives_resize_zoom_pan():
    """验收标准：擦除障碍物 → 改窗口大小 / 缩放 / 平移 → 导出 PGM，编辑必须保留。"""
    with sync_playwright() as p:
        browser = _launch(p)
        page = browser.new_page(viewport={"width": 1280, "height": 900})
        errors: list[str] = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.goto(_page_url("mapview.html"))
        page.wait_for_timeout(300)

        _drop_pgm(page)
        assert page.evaluate("() => !!window.__mapState()"), "地图未载入"

        # 1) 擦除一处障碍物（工具切换 + 在图上拖动）
        page.click("#eraseBtn")
        box = page.locator("#ov").bounding_box()
        assert box, "画布不可见"
        state = page.evaluate("() => window.__mapState()")
        scale = state["scale"]
        # 图像坐标 → 画布 CSS 坐标 → 页面坐标
        target_x = box["x"] + 6 * scale
        target_y = box["y"] + 5 * scale
        page.mouse.move(target_x, target_y)
        page.mouse.down()
        page.mouse.move(target_x + 1, target_y + 1)
        page.mouse.up()
        page.wait_for_timeout(200)

        edited = page.evaluate("() => window.__mapState().edited")
        assert edited > 0, "擦除没有生效"

        # 2) 改变窗口尺寸（多次）
        for size in [(1000, 800), (1400, 900), (900, 700)]:
            page.set_viewport_size({"width": size[0], "height": size[1]})
            page.wait_for_timeout(150)

        # 3) 缩放（滚轮）与平移（普通模式拖动）
        page.mouse.move(target_x, target_y)
        page.mouse.wheel(0, -300)
        page.wait_for_timeout(150)
        page.mouse.wheel(0, 600)
        page.wait_for_timeout(150)
        page.click("#measureBtn")  # 退出擦除模式
        page.click("#measureBtn")
        page.mouse.move(box["x"] + 20, box["y"] + 20)
        page.mouse.down()
        page.mouse.move(box["x"] + 60, box["y"] + 45)
        page.mouse.up()
        page.wait_for_timeout(150)

        # 4) 导出 PGM 并核对：编辑内容必须还在
        after = page.evaluate("() => window.__mapState()")
        assert after["edited"] == edited, f"编辑内容在视图变化后丢失: {edited} -> {after['edited']}"

        exported = page.evaluate("() => window.__exportPGM()")
        assert exported is not None and len(exported) > 0, "导出失败"
        # 头部之后是像素数据：被擦除的位置应为 255（空白），原障碍物为 0
        head_len = len("P5\n40 30\n255\n")
        pixels = exported[head_len:]
        assert len(pixels) == 40 * 30
        assert pixels[5 * 40 + 6] == 255, "导出的 PGM 没有保留擦除结果"

        assert not errors, f"页面存在 JS 错误: {errors}"
        browser.close()


# ---------- PGM 灰度归一化（回归网页端未按 maxv 换算导致整图偏暗的缺陷） ----------

# 样本文件 → 期望的 8 位灰度像素（与 Go 版 internal/render/pgm_test.go 一致）
PGM_CASES = [
    ("p5_8bit_255.pgm", [0, 64, 128, 255]),
    ("p5_8bit_15.pgm", [0, 136, 255]),
    ("p5_8bit_3.pgm", [0, 85, 170, 255]),
    ("p2_ascii_15.pgm", [0, 136, 255]),
    ("p2_ascii_255.pgm", [0, 85, 170, 255]),
    ("p5_16bit_65535.pgm", [0, 255]),
    ("p5_16bit_1000.pgm", [0, 255]),
]


def test_mapview_pgm_gray_normalization():
    """网页端解析 PGM 必须按声明的 maxval 归一化到 0..255。"""
    with sync_playwright() as p:
        browser = _launch(p)
        page = browser.new_page()
        errors: list[str] = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.goto(_page_url("mapview.html"))
        page.wait_for_timeout(300)

        for name, want in PGM_CASES:
            path = SAMPLES / name
            if not path.exists():
                raise AssertionError(f"缺少共享测试样本: {path}")
            raw = path.read_bytes()
            b64 = base64.b64encode(raw).decode()
            got = page.evaluate(
                """(b64) => {
                    const bin = atob(b64);
                    const bytes = new Uint8Array(bin.length);
                    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
                    return window.__decodePGM(bytes);
                }""",
                b64,
            )
            assert got is not None, f"{name} 解码失败"
            assert got["pixels"] == want, f"{name} 灰度归一化错误: {got['pixels']} != {want}"

        assert not errors, f"页面存在 JS 错误: {errors}"
        browser.close()


def test_mapview_rejects_bad_pgm():
    """非法 PGM 必须报错而不是静默产出错误图像。"""
    with sync_playwright() as p:
        browser = _launch(p)
        page = browser.new_page()
        page.goto(_page_url("mapview.html"))
        page.wait_for_timeout(200)

        for name, blob in [
            ("未知魔数", b"P3\n1 1\n255\n0\n"),
            ("尺寸为 0", b"P5\n0 0\n255\n"),
            ("数据不足", b"P5\n4 4\n255\nshort"),
        ]:
            b64 = base64.b64encode(blob).decode()
            failed = page.evaluate(
                """(b64) => {
                    const bin = atob(b64);
                    const bytes = new Uint8Array(bin.length);
                    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
                    try { window.__decodePGM(bytes); return false; } catch (e) { return true; }
                }""",
                b64,
            )
            assert failed, f"{name} 应抛出错误"

        browser.close()


# ---------- Base64 ----------

def test_base64_roundtrip_and_errors():
    """Base64 文本往返转换 + URL-safe + 非法输入报错。"""
    with sync_playwright() as p:
        browser = _launch(p)
        page = browser.new_page()
        errors: list[str] = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.goto(_page_url("base64.html"))
        page.wait_for_timeout(200)

        text = "你好，世界！Hello 🌍 123"
        page.fill("#input", text)
        page.click("#btnEncode")
        encoded = page.input_value("#output")
        assert encoded, "编码结果为空"
        assert "5L2g5aW9" in encoded, f"中文编码结果不对: {encoded}"

        # 往返：把编码结果放回输入框解码
        page.fill("#input", encoded)
        page.click("#btnDecode")
        assert page.input_value("#output") == text, "往返转换后内容不一致"

        # URL-safe：不应出现 + / 与 =
        page.check("#optUrlSafe")
        page.fill("#input", "???>>>~~~")
        page.click("#btnEncode")
        urlsafe = page.input_value("#output")
        assert "+" not in urlsafe and "/" not in urlsafe and "=" not in urlsafe, f"URL-safe 失败: {urlsafe}"
        page.uncheck("#optUrlSafe")

        # 非法字符必须给出错误提示（而不是抛 JS 异常）
        page.fill("#input", "这不是 base64!!!")
        page.click("#btnDecode")
        status = page.text_content("#status") or ""
        assert "解码失败" in status, f"非法输入未报错: {status}"

        assert not errors, f"页面存在 JS 错误: {errors}"
        browser.close()


# ---------- 二维码 ----------

def test_qrcode_generate_and_download():
    """二维码：文本生成 + PNG / SVG 下载。"""
    with sync_playwright() as p:
        browser = _launch(p)
        page = browser.new_page()
        errors: list[str] = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.goto(_page_url("qrcode.html"))
        page.wait_for_timeout(200)

        page.fill("#content", "https://github.com/XiaoYuyu520520/Copy-Paste-Mutools")
        page.click("#btnGen")
        page.wait_for_timeout(500)

        # 预览区应出现内联 SVG
        assert page.locator("#preview svg").count() >= 1, "二维码未生成预览"

        # 下载 PNG
        with page.expect_download() as dl:
            page.click("#btnPng")
        png = dl.value
        assert png.suggested_filename.endswith(".png"), png.suggested_filename
        png_path = pathlib.Path(png.path())
        assert png_path.stat().st_size > 0, "PNG 内容为空"

        # 下载 SVG
        with page.expect_download() as dl2:
            page.click("#btnSvg")
        svg = dl2.value
        assert svg.suggested_filename.endswith(".svg"), svg.suggested_filename
        svg_path = pathlib.Path(svg.path())
        assert svg_path.stat().st_size > 0, "SVG 内容为空"

        assert not errors, f"页面存在 JS 错误: {errors}"
        browser.close()


# ---------- Markdown ----------

def test_markdown_render_split_and_theme():
    """Markdown：输入即渲染 + 左右对照 + 主题切换。"""
    with sync_playwright() as p:
        browser = _launch(p)
        page = browser.new_page(viewport={"width": 1280, "height": 900})
        errors: list[str] = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.goto(_page_url("markdown.html"))
        page.wait_for_timeout(300)

        page.fill("#src", "# 标题\n\n正文段落 **加粗**\n\n```go\nfunc main() {}\n```\n")
        page.wait_for_timeout(500)

        out = page.text_content("#out") or ""
        assert "标题" in out, f"标题未渲染: {out[:200]}"
        assert "加粗" in out, "正文未渲染"
        assert "func main" in out, "代码块未渲染"

        # 左右对照
        page.click("#btnSplit")
        page.wait_for_timeout(300)
        assert page.locator("#edit").is_visible(), "左右对照未显示源码区"

        # 主题切换（不应抛错）
        page.click("#btnTheme")
        page.wait_for_timeout(200)
        page.click("#btnTheme")
        page.wait_for_timeout(200)

        # 清空
        page.click("#btnClear")
        page.wait_for_timeout(200)

        assert not errors, f"页面存在 JS 错误: {errors}"
        browser.close()


def test_markdown_open_file():
    """Markdown：拖入 .md 文件应渲染并显示文件名。"""
    with sync_playwright() as p:
        browser = _launch(p)
        page = browser.new_page(viewport={"width": 1280, "height": 900})
        errors: list[str] = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.goto(_page_url("markdown.html"))
        page.wait_for_timeout(300)

        content = "# 来自文件\n\n这是拖入的 Markdown 内容。\n"
        b64 = base64.b64encode(content.encode()).decode()
        page.evaluate(
            """(arg) => {
                const bin = atob(arg.b64);
                const bytes = new Uint8Array(bin.length);
                for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
                const file = new File([bytes], arg.name, { type: 'text/markdown' });
                const ev = new Event('drop', { bubbles: true, cancelable: true });
                ev.dataTransfer = { files: [file] };
                window.dispatchEvent(ev);
            }""",
            {"b64": b64, "name": "sample.md"},
        )
        page.wait_for_timeout(600)

        out = page.text_content("#out") or ""
        assert "来自文件" in out, f"拖入文件未渲染: {out[:200]}"
        info = page.text_content("#info") or ""
        assert "sample.md" in info, f"未显示文件名: {info}"

        assert not errors, f"页面存在 JS 错误: {errors}"
        browser.close()


# ---------- 编解码检测 ----------

def test_codec_detect_runs_and_reports():
    """编解码检测页应完成检测并给出报告，异常 API 时不崩。"""
    with sync_playwright() as p:
        browser = _launch(p)
        page = browser.new_page(viewport={"width": 1280, "height": 900})
        errors: list[str] = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.goto(_page_url("codec.html"))
        page.wait_for_timeout(1200)

        status = (page.text_content("#status") or "").strip()
        assert status, "检测状态为空"
        assert "失败" not in status and "错误" not in status, f"检测失败: {status}"

        # 环境表格应已填充
        assert page.locator("#videoBody tr").count() > 0, "视频表格为空"
        assert page.locator("#audioBody tr").count() > 0, "音频表格为空"

        # 重新检测不应抛错
        page.click("#btnRun")
        page.wait_for_timeout(800)

        assert not errors, f"页面存在 JS 错误: {errors}"
        browser.close()



# ---------- 主题 / 搜索 / 即时转换 ----------

def _bg_luma(page) -> float:
    """读取 body 背景色亮度（0~255）：亮色主题应显著高于深色主题。"""
    rgb = page.evaluate("() => getComputedStyle(document.body).backgroundColor")
    m = rgb.replace("rgba(", "").replace("rgb(", "").replace(")", "").split(",")
    r, g, b = (float(x) for x in m[:3])
    return 0.299 * r + 0.587 * g + 0.114 * b


def test_theme_toggle_persists_across_pages():
    """主题切换：默认跟随系统，手动切换后整个工具站在其它页面也保持该主题。"""
    with sync_playwright() as p:
        browser = _launch(p)
        ctx = browser.new_context(color_scheme="dark")
        page = ctx.new_page()
        errors: list[str] = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.goto(_page_url("index.html"))
        page.wait_for_timeout(250)

        # 默认跟随系统（dark）
        assert page.evaluate("() => document.documentElement.getAttribute('data-theme')") == "dark"
        dark_luma = _bg_luma(page)

        # 顶栏应有主题按钮；点击切到亮色
        btn = page.locator("nav.top .theme-toggle")
        expect(btn).to_have_count(1)
        btn.click()
        page.wait_for_timeout(150)
        assert page.evaluate("() => document.documentElement.getAttribute('data-theme')") == "light"
        light_luma = _bg_luma(page)
        assert light_luma > dark_luma + 80, f"亮色背景亮度未提升: {dark_luma} -> {light_luma}"

        # 换到另一个页面，主题应保持（localStorage 记忆）
        page.goto(_page_url("base64.html"))
        page.wait_for_timeout(200)
        assert page.evaluate("() => document.documentElement.getAttribute('data-theme')") == "light"
        assert _bg_luma(page) > 200, "Base64 页未应用亮色主题"

        # Ctrl+Shift+D 切回夜间
        page.keyboard.press("Control+Shift+D")
        page.wait_for_timeout(150)
        assert page.evaluate("() => document.documentElement.getAttribute('data-theme')") == "dark"
        assert not errors, f"页面存在 JS 错误: {errors}"
        browser.close()


def test_index_tool_search_filters_and_opens():
    """总览页搜索：输入关键词过滤卡片，Enter 打开第一个结果，/ 聚焦搜索框。"""
    with sync_playwright() as p:
        browser = _launch(p)
        page = browser.new_page(viewport={"width": 1280, "height": 900})
        errors: list[str] = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.goto(_page_url("index.html"))
        page.wait_for_timeout(250)

        total = page.locator("#toolList .tool-card").count()
        assert total == 5, f"工具卡片数 = {total}"

        # 按分类标签搜索：只剩二维码
        page.fill("#toolSearch", "二维码")
        page.wait_for_timeout(100)
        visible = page.locator("#toolList .tool-card:visible")
        assert visible.count() == 1, f"过滤后可见卡片 = {visible.count()}"
        assert "qrcode" in (visible.first.get_attribute("href") or "")

        # 用场景词（不在标题里）也能搜到
        page.fill("#toolSearch", "ros")
        page.wait_for_timeout(100)
        visible = page.locator("#toolList .tool-card:visible")
        assert visible.count() == 1 and "mapview" in (visible.first.get_attribute("href") or "")

        # 无结果提示
        page.fill("#toolSearch", "不存在的工具xyz")
        page.wait_for_timeout(100)
        expect(page.locator("#noResult")).to_be_visible()

        # Esc 清空
        page.keyboard.press("Escape")
        page.wait_for_timeout(100)
        assert page.locator("#toolList .tool-card:visible").count() == 5

        # / 聚焦搜索框
        page.click("h1")
        page.keyboard.press("/")
        assert page.evaluate("() => document.activeElement && document.activeElement.id") == "toolSearch"

        # 方向键 + Enter 打开
        page.fill("#toolSearch", "markdown")
        page.wait_for_timeout(100)
        page.keyboard.press("ArrowDown")
        page.keyboard.press("Enter")
        page.wait_for_url("**/markdown.html")

        assert not errors, f"页面存在 JS 错误: {errors}"
        browser.close()


def test_base64_live_convert_and_meters():
    """Base64：输入即自动转换、方向自动识别、字节计量、Ctrl+Shift+Enter 解码。"""
    with sync_playwright() as p:
        browser = _launch(p)
        page = browser.new_page()
        errors: list[str] = []
        page.on("pageerror", lambda e: errors.append(str(e)))
        page.goto(_page_url("base64.html"))
        page.wait_for_timeout(200)

        # 中文输入 → 自动编码
        page.fill("#input", "你好")
        page.wait_for_timeout(400)
        assert page.input_value("#output") == "5L2g5aW9", page.input_value("#output")
        meters = page.text_content("#meters") or ""
        assert "字节" in meters and "字节 8" in meters, f"计量未显示: {meters}"

        # 粘贴 Base64 → 自动识别为解码
        page.fill("#input", "5L2g5aW9")
        page.wait_for_timeout(400)
        assert page.input_value("#output") == "你好", page.input_value("#output")

        # 手动指定解码方向后，即使内容像普通文本也按 Base64 解码并报错
        page.fill("#input", "这不是 base64!!!")
        page.wait_for_timeout(400)
        page.keyboard.press("Control+Shift+Enter")
        page.wait_for_timeout(200)
        assert "解码失败" in (page.text_content("#status") or "")

        assert not errors, f"页面存在 JS 错误: {errors}"
        browser.close()


# ---------- 通用导航 ----------

def test_tool_pages_load_with_common_nav():
    """所有工具页都应使用统一导航（nav.top + 品牌 + 工具总览链接）。"""
    pages = ["index.html", "base64.html", "qrcode.html", "codec.html", "markdown.html", "mapview.html"]
    with sync_playwright() as p:
        browser = _launch(p)
        for name in pages:
            page = browser.new_page()
            errors: list[str] = []
            page.on("pageerror", lambda e: errors.append(str(e)))
            page.goto(_page_url(name))
            page.wait_for_timeout(250)
            nav = page.locator("nav.top")
            expect(nav).to_have_count(1)
            expect(nav).to_contain_text("提示词复合工具箱 · 在线工具")
            expect(nav).to_contain_text("工具总览")
            assert page.locator("footer").count() == 1, f"{name} 缺少统一页脚"
            assert not errors, f"{name} 存在 JS 错误: {errors}"
            page.close()
        browser.close()


ALL_TESTS = [
    test_map_edit_survives_resize_zoom_pan,
    test_mapview_pgm_gray_normalization,
    test_mapview_rejects_bad_pgm,
    test_base64_roundtrip_and_errors,
    test_qrcode_generate_and_download,
    test_markdown_render_split_and_theme,
    test_markdown_open_file,
    test_codec_detect_runs_and_reports,
    test_theme_toggle_persists_across_pages,
    test_index_tool_search_filters_and_opens,
    test_base64_live_convert_and_meters,
    test_tool_pages_load_with_common_nav,
]


if __name__ == "__main__":
    failed = 0
    for fn in ALL_TESTS:
        try:
            fn()
            print(f"PASS {fn.__name__}")
        except Exception as exc:  # noqa: BLE001
            failed += 1
            print(f"FAIL {fn.__name__}: {exc}")
    raise SystemExit(1 if failed else 0)
