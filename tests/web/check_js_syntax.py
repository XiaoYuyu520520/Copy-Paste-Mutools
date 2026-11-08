"""检查 docs/tools 下网页工具脚本的语法。

用法：py check_js_syntax.py [文件...]
  - .html：提取全部内联 <script> 块分别检查
  - .js  ：直接检查

需要 Node.js：依次查找 NODE 环境变量、PATH 里的 node、常见安装目录。
本地没有 node 时跳过（退出码 0），方便在没有 Node 的机器上运行其它检查。
"""

import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2] / "docs" / "tools"


def _node() -> str | None:
    """查找 node：优先 NODE 环境变量，其次 PATH，最后 Windows 默认安装目录。"""
    env = os.environ.get("NODE")
    if env and os.path.exists(env):
        return env
    for name in ("node", "nodejs"):
        p = shutil.which(name)
        if p:
            return p
    for cand in (
        r"C:\Program Files\nodejs\node.exe",
        "/usr/local/bin/node",
        "/usr/bin/node",
    ):
        if os.path.exists(cand):
            return cand
    return None


NODE = _node()

SCRIPT_RE = re.compile(r"<script(?:\s[^>]*)?>(.*?)</script>", re.S)


def scripts_of(path: Path) -> list[str]:
    text = path.read_text(encoding="utf-8")
    if path.suffix == ".js":
        return [text]
    out = []
    for m in SCRIPT_RE.finditer(text):
        body = m.group(1)
        # 有 src 属性的 <script src=...></script> 内联部分为空，跳过
        head = text[m.start() : m.start() + 100]
        if "src=" in head.split(">", 1)[0]:
            continue
        if body.strip():
            out.append(body)
    return out


def check(path: Path) -> bool:
    ok = True
    for i, code in enumerate(scripts_of(path)):
        with tempfile.NamedTemporaryFile(
            "w", suffix=".js", delete=False, encoding="utf-8"
        ) as f:
            f.write(code)
            tmp = f.name
        try:
            r = subprocess.run([NODE, "--check", tmp], capture_output=True, text=True)
            if r.returncode != 0:
                ok = False
                print(f"FAIL {path.name} #{i}:\n{r.stderr}")
        finally:
            os.unlink(tmp)
    return ok


def main(argv: list[str]) -> int:
    if not NODE:
        # 没有 node 时跳过而不是失败：这个检查只是快速旁路，
        # Playwright 用例仍会加载页面并捕获 JS 错误。
        print("未找到 Node.js，跳过语法检查", file=sys.stderr)
        return 0
    targets = [Path(a) for a in argv] or sorted(
        p for p in ROOT.iterdir() if p.suffix in (".html", ".js")
    )
    failed = [p for p in targets if not check(p)]
    if failed:
        print("语法错误：", ", ".join(p.name for p in failed))
        return 1
    print(f"语法检查通过：{len(targets)} 个文件")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
