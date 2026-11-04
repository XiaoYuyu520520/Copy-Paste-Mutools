"""网页版工具的 Playwright 冒烟测试。

覆盖：
  - 页面能打开、主要按钮存在且可点击、无 JS 运行时错误
  - 2D 激光地图：擦除障碍物后，连续改变窗口尺寸 / 缩放 / 平移，
    导出的 PGM 必须保留编辑内容（不退回原图）

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


def _drop_pgm(page, w: int = 40, h: int = 30) -> None:
    """构造一个 PGM 并以「拖放」方式送入页面（绕过 file input 的限制）。"""
    px = bytearray()
    for y in range(h):
        for x in range(w):
            px.append(0 if (x in (5, 6, 7) and y in (5, 6)) else 255)
    head = f"P5\n{w} {h}\n255\n"
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
        # 障碍物位于图像 (6,5)；先适应窗口，再换算到屏幕坐标
        page.evaluate("() => { const b=document.getElementById('ov').getBoundingClientRect(); return null; }")
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


if __name__ == "__main__":
    failed = 0
    for fn in (test_map_edit_survives_resize_zoom_pan, test_tool_pages_load_with_common_nav):
        try:
            fn()
            print(f"PASS {fn.__name__}")
        except Exception as exc:  # noqa: BLE001
            failed += 1
            print(f"FAIL {fn.__name__}: {exc}")
    raise SystemExit(1 if failed else 0)
