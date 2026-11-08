/* 在线工具站共用脚本：复制到剪贴板、状态提示、字节计量、主题按钮 */
async function copyText(text, okMessage) {
  const msg = okMessage || "已复制";
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
    } else {
      const ta = document.createElement("textarea");
      ta.value = text;
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      document.body.removeChild(ta);
    }
    setStatus(msg, "ok");
    return true;
  } catch (err) {
    setStatus("复制失败：" + err.message, "err");
    return false;
  }
}

function setStatus(text, kind) {
  const el = document.getElementById("status");
  if (!el) return;
  el.textContent = text || "";
  el.className = "status" + (kind ? " " + kind : "");
}

/* 复制按钮的即时反馈：按钮文字短暂换成「✓ 已复制」再恢复。
 * 失败时保持原样，错误由 copyText -> setStatus 展示。 */
async function copyWithFeedback(btn, text, okMessage) {
  const ok = await copyText(text, okMessage);
  if (!ok || !btn) return ok;
  const label = btn.textContent;
  btn.classList.add("is-copied");
  btn.textContent = "✓ 已复制";
  btn.disabled = true;
  setTimeout(() => {
    btn.classList.remove("is-copied");
    btn.textContent = label;
    btn.disabled = false;
  }, 1200);
  return ok;
}

/* 统计文本：行数、字符数（按 Unicode 码点）、UTF-8 字节数。 */
function textStats(text) {
  const s = text || "";
  const chars = Array.from(s).length;
  const lines = s === "" ? 0 : s.split("\n").length;
  let bytes = 0;
  try {
    bytes = new TextEncoder().encode(s).length;
  } catch (e) {
    bytes = s.length;
  }
  return { lines, chars, bytes };
}

/* 把统计结果写进 .meters 容器（没有则跳过）。 */
function renderMeters(el, table) {
  if (!el) return;
  const parts = [];
  for (const [label, value] of Object.entries(table)) {
    parts.push(`<span>${label} <b>${value}</b></span>`);
  }
  el.innerHTML = parts.join("");
}

function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/* 在导航末尾挂一个主题切换按钮，点击调用 SiteTheme.toggle()。
 * theme.js 必须先于本脚本引入。 */
(function mountThemeToggle() {
  function run() {
    const nav = document.querySelector("nav.top");
    if (!nav || !window.SiteTheme || nav.querySelector(".theme-toggle")) return;
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "theme-toggle";
    btn.title = "切换亮色 / 夜间（Ctrl+Shift+D）";
    btn.setAttribute("aria-label", "切换亮色 / 夜间");
    const sync = (t) => {
      btn.textContent = t === "dark" ? "☀" : "☾";
    };
    sync(window.SiteTheme.get());
    window.SiteTheme.onChange(sync);
    btn.addEventListener("click", () => window.SiteTheme.toggle());
    nav.appendChild(btn);

    // 全站统一的主题快捷键：Ctrl/⌘+Shift+D（不与各工具自己的 Ctrl+D 冲突）。
    window.addEventListener("keydown", (e) => {
      if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key || "").toLowerCase() === "d") {
        e.preventDefault();
        window.SiteTheme.toggle();
      }
    });
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", run);
  else run();
})();

/* 右下角提示：离线部署与需求/反馈 */
(function () {
  var HIDDEN_KEY = "ghBar.hidden";

  function mount() {
    try {
      if (localStorage.getItem(HIDDEN_KEY) === "1") return;
    } catch (e) {}
    if (document.querySelector(".gh-bar")) return;

    var bar = document.createElement("div");
    bar.className = "gh-bar";

    var releases = document.createElement("a");
    releases.href = "https://github.com/XiaoYuyu520520/Copy-Paste-Mutools/releases";
    releases.target = "_blank";
    releases.rel = "noopener";
    releases.textContent = "离线版下载";

    var sep = document.createElement("span");
    sep.className = "sep";
    sep.textContent = "·";

    var issues = document.createElement("a");
    issues.href = "https://github.com/XiaoYuyu520520/Copy-Paste-Mutools/issues";
    issues.target = "_blank";
    issues.rel = "noopener";
    issues.textContent = "需求 / Bug 反馈";

    var close = document.createElement("button");
    close.type = "button";
    close.title = "不再显示";
    close.setAttribute("aria-label", "不再显示");
    close.textContent = "×";
    close.addEventListener("click", function () {
      bar.remove();
      try { localStorage.setItem(HIDDEN_KEY, "1"); } catch (e) {}
    });

    bar.appendChild(releases);
    bar.appendChild(sep);
    bar.appendChild(issues);
    bar.appendChild(close);
    document.body.appendChild(bar);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mount);
  } else {
    mount();
  }
})();
