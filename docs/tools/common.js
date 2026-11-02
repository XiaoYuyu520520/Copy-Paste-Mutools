/* 在线工具站共用脚本：复制到剪贴板 */
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
  } catch (err) {
    setStatus("复制失败：" + err.message, "err");
  }
}

function setStatus(text, kind) {
  const el = document.getElementById("status");
  if (!el) return;
  el.textContent = text || "";
  el.className = "status" + (kind ? " " + kind : "");
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
