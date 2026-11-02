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
