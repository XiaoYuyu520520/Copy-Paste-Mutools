package render

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// docToken 在页面模板中被替换成当前文档的 JSON，让首屏直接带内容，不必再等一次请求。
const docToken = "__DOC_JSON__"

// markdownEditorPage 是浏览器自含的 Markdown 阅读页：
//   - 拖入 .md/.markdown/.txt 立即「全屏渲染」；「左右对照」是可选开关（Ctrl+E）
//   - 亮色 / 夜间手动切换（Ctrl+D）：两套 GitHub 主题 CSS 显式互斥，不依赖系统配色，
//     暗色下代码块底色与 hljs 深色主题配套，避免「深色字配深色底」
//   - 拖入 .pgm(+.yaml) 在同一页内用 iframe 打开 2D 激光 SLAM 地图编辑器
//   - 原生窗口拖入的文件由 /api/doc 与 /api/events(SSE) 推送进来并热切换
//
// 页面与 /assets 同源，完全离线；JS 里不使用反引号模板串（Go 原始字符串限制）。
const markdownEditorPage = `<!DOCTYPE html>
<html lang="zh-CN" data-theme="light">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Markdown 渲染</title>
<link rel="icon" href="data:,">
<link rel="stylesheet" href="/assets/github-markdown-light.min.css" id="mdLight">
<link rel="stylesheet" href="/assets/github-markdown-dark.min.css" id="mdDark" disabled>
<link rel="stylesheet" href="/assets/github.min.css" id="hlLight">
<link rel="stylesheet" href="/assets/github-dark.min.css" id="hlDark" disabled>
<style>
:root{
  --bg:#ffffff; --fg:#1f2328; --muted:#59636e; --border:#d1d9e0; --panel:#f6f8fa;
  --btn:#ffffff; --btn-hover:#eef1f4; --accent:#0969da; --shadow:rgba(31,35,40,.18);
}
:root[data-theme="dark"]{
  --bg:#0d1117; --fg:#e6edf3; --muted:#9198a1; --border:#30363d; --panel:#161b22;
  --btn:#21262d; --btn-hover:#30363d; --accent:#1f6feb; --shadow:rgba(1,4,9,.6);
}
*{box-sizing:border-box}
html,body{margin:0;height:100%}
body{display:flex;flex-direction:column;background:var(--bg);color:var(--fg);overflow:hidden;
  font:14px/1.5 -apple-system,"Segoe UI","Microsoft YaHei",sans-serif}
button{font:inherit;cursor:pointer;background:var(--btn);color:var(--fg);
  border:1px solid var(--border);border-radius:6px;padding:4px 10px}
button:hover{background:var(--btn-hover)}
button.on{background:var(--accent);border-color:var(--accent);color:#fff}

/* ---------- 顶栏 ---------- */
#top{flex:0 0 auto;display:flex;flex-wrap:wrap;gap:8px;align-items:center;
  padding:7px 12px;background:var(--panel);border-bottom:1px solid var(--border)}
#top .title{font-weight:700}
#top .sp{flex:1}
#info{color:var(--muted);font-size:12px;max-width:40vw;overflow:hidden;
  text-overflow:ellipsis;white-space:nowrap}
.seg{display:inline-flex}
.seg button{border-radius:0}
.seg button:first-child{border-radius:6px 0 0 6px}
.seg button:last-child{border-radius:0 6px 6px 0;border-left:0}

/* ---------- 主体：源码 / 渲染 ---------- */
#body{flex:1 1 auto;min-height:0;display:flex}
#edit{display:flex;flex-direction:column;min-width:0;border-right:1px solid var(--border)}
#src{flex:1;resize:none;border:0;outline:none;padding:14px;background:var(--panel);color:var(--fg);
  font:13px/1.7 ui-monospace,"Cascadia Mono",Consolas,monospace}
#stat{padding:4px 12px;color:var(--muted);font-size:12px;border-top:1px solid var(--border);
  white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
#preview{position:relative;flex:1 1 auto;min-width:0;overflow:auto;background:var(--bg)}
body[data-view="render"] #edit{display:none}
body[data-view="split"] #edit{flex:0 0 50%}

/* 渲染结果占满整屏；代码块底色统一交给当前 Markdown 主题，保证深浅两套都有对比度 */
#out{max-width:none;margin:0;padding:18px 30px 140px}
#out pre{position:relative}
#out pre code.hljs{background:transparent;padding-right:64px}
#out .err{color:#d1242f}
.copy{position:absolute;top:8px;right:8px;opacity:0;transition:opacity .15s;font-size:12px;padding:2px 8px}
#out pre:hover .copy{opacity:1}
.copy.ok{opacity:1;color:#1a7f37;border-color:#1a7f37}

/* ---------- 空状态 ---------- */
#empty{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;
  background:var(--bg);padding:16px}
#empty .card{border:2px dashed var(--border);border-radius:14px;padding:32px 40px;
  text-align:center;max-width:640px}
#empty .big{font-size:20px;font-weight:700;margin-bottom:12px}
#empty .sub{color:var(--muted);font-size:13px;line-height:1.9;margin-bottom:18px}
#empty .row{display:flex;gap:10px;justify-content:center}

/* ---------- 拖放提示 / 提示条 ---------- */
#drop{position:fixed;inset:0;display:none;align-items:center;justify-content:center;z-index:30;
  background:rgba(9,105,218,.14);border:3px dashed var(--accent)}
#drop.on{display:flex}
#drop .hint{background:var(--accent);color:#fff;padding:10px 22px;border-radius:999px;font-weight:700}
#toast{position:fixed;left:50%;bottom:22px;transform:translate(-50%,20px);opacity:0;z-index:40;
  pointer-events:none;transition:opacity .2s,transform .2s;background:var(--panel);color:var(--fg);
  border:1px solid var(--border);border-radius:8px;padding:7px 14px;font-size:13px;
  box-shadow:0 6px 20px var(--shadow);max-width:70vw}
#toast.on{opacity:1;transform:translate(-50%,0)}

/* ---------- 地图查看器 ---------- */
#viewer{display:none;position:fixed;inset:0;z-index:20;background:#010409;flex-direction:column}
#vbar{display:flex;gap:10px;align-items:center;padding:6px 12px;background:#161b22;
  border-bottom:1px solid #30363d;color:#e6edf3;font-size:13px}
#vbar .sp{flex:1}
#vbar .muted{color:#8b949e;font-size:12px}
#vbar button{background:#21262d;border:1px solid #30363d;color:#c9d1d9}
#vbar button:hover{background:#30363d}
#vframe{flex:1;width:100%;border:0;background:#010409}

@media print{
  #top,#edit,#drop,#empty,#toast,#viewer{display:none!important}
  body{overflow:visible;background:#fff;color:#000}
  #body{display:block}
  #preview{overflow:visible}
  #out{padding:0}
}
</style>
</head>
<body data-view="render">
<div id="top">
  <span class="title">Markdown 渲染</span>
  <span id="info">未打开文件 · 拖入 .md 即全屏渲染</span>
  <span class="sp"></span>
  <span class="seg">
    <button id="btnRender" class="on" title="只显示渲染结果，占满整屏（Ctrl+E 切换）">全屏渲染</button>
    <button id="btnSplit" title="左源码 / 右预览（Ctrl+E 切换）">左右对照</button>
  </span>
  <button id="btnTheme" title="切换亮色 / 夜间（Ctrl+D）">🌙 夜间</button>
  <button id="btnFull" title="全屏阅读，Esc 退出">全屏</button>
  <button id="btnOpen" title="打开 .md/.txt/.pgm/.yaml（Ctrl+O）">打开文件…</button>
  <button id="btnClear" title="清空当前内容">清空</button>
  <button id="btnPrint" title="打印 / 导出 PDF">打印</button>
  <input id="file" type="file" multiple hidden
    accept=".md,.markdown,.mdown,.txt,.text,.log,.pgm,.yaml,.yml">
</div>

<div id="body">
  <div id="edit">
    <textarea id="src" spellcheck="false"
      placeholder="把 README 拖到这里，或直接粘贴 Markdown&#10;（Ctrl+E 切换左右对照）"></textarea>
    <div id="stat">等待输入</div>
  </div>
  <div id="preview">
    <article id="out" class="markdown-body"></article>
    <div id="empty">
      <div class="card">
        <div class="big">把 README 拖到这里</div>
        <div class="sub">
          .md / .markdown / .txt 松手即全屏渲染；<br>
          .pgm(+.yaml) 打开 2D 激光 SLAM 地图编辑器
        </div>
        <div class="row">
          <button id="btnOpen2">打开文件…</button>
          <button id="btnPaste">粘贴 Markdown</button>
        </div>
      </div>
    </div>
  </div>
</div>

<div id="drop"><div class="hint">松手即渲染</div></div>
<div id="toast"></div>

<div id="viewer">
  <div id="vbar">
    <button id="vclose">← 返回工具页</button>
    <span id="vtitle"></span>
    <span class="sp"></span>
    <span class="muted">页内可测量 / 擦除 / 另存 .pgm + .yaml</span>
  </div>
  <iframe id="vframe" title="2D 激光 SLAM 地图编辑器"></iframe>
</div>

<script src="/assets/marked.min.js"></script>
<script src="/assets/highlight.min.js"></script>
<script>var INIT_DOC = __DOC_JSON__;</script>
<script>
(function(){
  function $(id){ return document.getElementById(id); }
  var els = {
    src:$('src'), out:$('out'), stat:$('stat'), info:$('info'), empty:$('empty'),
    drop:$('drop'), toast:$('toast'), viewer:$('viewer'), vtitle:$('vtitle'),
    vframe:$('vframe'), file:$('file'), theme:$('btnTheme'), full:$('btnFull'),
    btnRender:$('btnRender'), btnSplit:$('btnSplit')
  };
  var state = { kind:'empty', name:'', version:0, dirty:false, theme:'light', view:'render' };
  var KEY = { theme:'md.theme', view:'md.view' };
  var timer = null, toastTimer = null;

  function lsGet(k){ try { return localStorage.getItem(k); } catch(e){ return null; } }
  function lsSet(k, v){ try { localStorage.setItem(k, v); } catch(e){} }

  function toast(msg){
    els.toast.textContent = msg;
    els.toast.classList.add('on');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function(){ els.toast.classList.remove('on'); }, 2800);
  }

  // ---------- 主题：亮 / 夜间，手动切换且显式互斥两套 CSS ----------
  // 不依赖 prefers-color-scheme：系统配色不会再把 Markdown 与代码高亮拆成两套，避免暗色下糊成一片。
  function applyTheme(theme, persist){
    state.theme = theme === 'dark' ? 'dark' : 'light';
    var dark = state.theme === 'dark';
    document.documentElement.setAttribute('data-theme', state.theme);
    $('mdLight').disabled = dark;
    $('mdDark').disabled = !dark;
    $('hlLight').disabled = dark;
    $('hlDark').disabled = !dark;
    els.theme.textContent = dark ? '☀️ 日间' : '🌙 夜间';
    if (persist) lsSet(KEY.theme, state.theme);
    if (state.kind === 'markdown') render();   // hljs 打过标记，换主题要重新上色
  }
  function toggleTheme(){ applyTheme(state.theme === 'dark' ? 'light' : 'dark', true); }

  // ---------- 视图：全屏渲染 / 左右对照 ----------
  function applyView(view, persist){
    state.view = view === 'split' ? 'split' : 'render';
    document.body.setAttribute('data-view', state.view);
    els.btnRender.classList.toggle('on', state.view === 'render');
    els.btnSplit.classList.toggle('on', state.view === 'split');
    if (persist) lsSet(KEY.view, state.view);
  }
  function toggleView(){ applyView(state.view === 'split' ? 'render' : 'split', true); }

  // ---------- 渲染 ----------
  function esc(s){
    return String(s).replace(/[&<>"]/g, function(c){
      return { '&':'&amp;', '<':'&lt;', '>':'&gt;', '"':'&quot;' }[c];
    });
  }

  function copyText(text, btn){
    var done = function(){
      btn.textContent = '已复制 ✓'; btn.classList.add('ok');
      setTimeout(function(){ btn.textContent = '复制'; btn.classList.remove('ok'); }, 1200);
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, function(){ fallbackCopy(text, done); });
    } else {
      fallbackCopy(text, done);
    }
  }
  function fallbackCopy(text, done){
    var ta = document.createElement('textarea');
    ta.value = text; document.body.appendChild(ta); ta.select();
    try { document.execCommand('copy'); done(); } catch(e){}
    document.body.removeChild(ta);
  }

  function decorate(box){
    box.querySelectorAll('pre').forEach(function(pre){
      var code = pre.querySelector('code');
      if (!code) return;
      if (window.hljs) { try { hljs.highlightElement(code); } catch(e){} }
      if (pre.querySelector('.copy')) return;
      var btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'copy';
      btn.textContent = '复制';
      btn.onclick = function(){ copyText(code.innerText, btn); };
      pre.appendChild(btn);
    });
  }

  function render(){
    var text = els.src.value;
    if (typeof marked === 'undefined') {
      els.out.innerHTML = '<p class="err">marked 未加载，无法渲染。</p>';
      return;
    }
    try {
      els.out.innerHTML = marked.parse(text, { gfm:true, breaks:false });
    } catch(e) {
      els.out.innerHTML = '<p class="err">渲染出错：' + esc(e.message) + '</p>';
    }
    decorate(els.out);
    var lines = text === '' ? 0 : text.split(/\n/).length;
    els.stat.textContent = (state.name ? state.name + ' · ' : '') + lines + ' 行 · ' + text.length + ' 字符';
  }

  // ---------- 文档装载 ----------
  function showEmpty(){
    state.kind = 'empty'; state.name = ''; state.dirty = false;
    els.src.value = ''; els.out.innerHTML = '';
    els.info.textContent = '未打开文件 · 拖入 .md 即全屏渲染';
    els.empty.style.display = 'flex';
    els.stat.textContent = '等待输入';
  }

  function setMarkdown(name, text){
    state.kind = 'markdown'; state.name = name || ''; state.dirty = false;
    els.src.value = text || '';
    els.empty.style.display = 'none';
    els.info.textContent = (name || '未命名文档') + ' · 已渲染';
    applyView('render', false);   // 拖入 / 打开即全屏渲染
    render();
  }

  function openMapByHtml(html, title){
    els.vtitle.textContent = title || '';
    els.vframe.srcdoc = html;
    els.viewer.style.display = 'flex';
  }
  function closeMap(){
    els.viewer.style.display = 'none';
    els.vframe.srcdoc = '';
  }
  function openMapFromServer(doc){
    fetch('/api/map/current?v=' + (doc.version || 0)).then(function(r){
      if (!r.ok) throw new Error('HTTP ' + r.status);
      return r.text();
    }).then(function(html){
      openMapByHtml(html, '2D 激光 SLAM 地图：' + (doc.name || ''));
      els.info.textContent = (doc.name || '地图') + ' · 地图编辑器';
    }).catch(function(e){ toast('地图打开失败：' + e.message); });
  }

  // applyDoc 处理来自原生窗口的文档推送（含首屏内嵌的 INIT_DOC）。
  function applyDoc(doc){
    if (!doc || !doc.kind) return;
    if (doc.version && doc.version === state.version) return;
    state.version = doc.version || state.version;

    if (doc.kind === 'map') {
      state.kind = 'map'; state.name = doc.name || '';
      openMapFromServer(doc);
      return;
    }
    if (doc.kind === 'markdown') {
      if (state.dirty && !confirm('页面里有未保存的改动，载入「' + (doc.name || '新文档') + '」会覆盖它。继续吗？')) return;
      setMarkdown(doc.name, doc.content);
      toast('已载入 ' + (doc.name || '文档'));
      return;
    }
    // empty：只在本地也没内容时清空，避免擦掉页面上正在看的东西
    if (doc.error) toast(doc.error);
    if (!state.name && !state.dirty) showEmpty();
  }

  function watch(){
    if (!window.EventSource) return;
    try {
      var es = new EventSource('/api/events');
      es.onmessage = function(ev){
        try { applyDoc(JSON.parse(ev.data)); } catch(e){}
      };
    } catch(e){}
  }

  // ---------- 文件：拖入 / 选择 ----------
  function fileKind(name){
    if (/\.pgm$/i.test(name)) return 'pgm';
    if (/\.(yaml|yml)$/i.test(name)) return 'yaml';
    if (/\.(md|markdown|mdown|txt|text|log)$/i.test(name)) return 'md';
    return 'other';
  }

  function handleFiles(files){
    var list = Array.prototype.slice.call(files || []);
    if (!list.length) return;
    var pgm = null, yaml = null, md = null, other = null;
    list.forEach(function(f){
      var k = fileKind(f.name);
      if (k === 'pgm') { if (!pgm) pgm = f; }
      else if (k === 'yaml') { if (!yaml) yaml = f; }
      else if (k === 'md') { if (!md) md = f; }
      else if (!other) other = f;
    });
    if (pgm) { uploadMap(pgm, yaml); return; }
    if (yaml) { toast('请把 .yaml 和同名 .pgm 一起拖进来'); return; }
    if (md) { readLocal(md); return; }
    toast('不支持的文件类型：' + (other ? other.name : '未知') + '（支持 .md/.markdown/.txt/.pgm/.yaml）');
  }

  function readLocal(file){
    var rd = new FileReader();
    rd.onload = function(){
      setMarkdown(file.name, String(rd.result || ''));
      toast('已载入 ' + file.name);
    };
    rd.onerror = function(){ toast('读取 ' + file.name + ' 失败'); };
    rd.readAsText(file, 'utf-8');
  }

  function uploadMap(pgm, yaml){
    var fd = new FormData();
    fd.append('pgm', pgm);
    if (yaml) fd.append('yaml', yaml);
    els.stat.textContent = '正在解析地图 ' + pgm.name + ' …';
    fetch('/api/map', { method:'POST', body: fd }).then(function(r){
      if (r.ok) return r.text();
      return r.text().then(function(t){ throw new Error(t || ('HTTP ' + r.status)); });
    }).then(function(html){
      state.kind = 'map';
      state.name = pgm.name + (yaml ? ' + ' + yaml.name : '');
      openMapByHtml(html, '2D 激光 SLAM 地图：' + state.name);
      els.info.textContent = state.name + ' · 地图编辑器';
      els.stat.textContent = '地图已打开';
    }).catch(function(e){ toast('地图打开失败：' + e.message); });
  }

  // ---------- 拖放：页面任意位置松手即渲染 ----------
  var dragDepth = 0;
  window.addEventListener('dragenter', function(e){
    e.preventDefault();
    dragDepth++;
    els.drop.classList.add('on');
  });
  window.addEventListener('dragover', function(e){
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy';
  });
  window.addEventListener('dragleave', function(e){
    e.preventDefault();
    dragDepth = Math.max(0, dragDepth - 1);
    if (!dragDepth) els.drop.classList.remove('on');
  });
  window.addEventListener('drop', function(e){
    e.preventDefault();
    dragDepth = 0;
    els.drop.classList.remove('on');
    handleFiles(e.dataTransfer && e.dataTransfer.files);
  });

  function toggleFull(){
    if (document.fullscreenElement) { document.exitFullscreen(); return; }
    var el = document.documentElement;
    if (el.requestFullscreen) {
      el.requestFullscreen().catch(function(){ toast('当前浏览器未允许全屏'); });
    } else {
      toast('当前浏览器不支持全屏');
    }
  }

  // ---------- 事件绑定 ----------
  els.src.addEventListener('input', function(){
    state.dirty = true;
    clearTimeout(timer);
    timer = setTimeout(render, 220);
  });
  els.btnRender.onclick = function(){ applyView('render', true); };
  els.btnSplit.onclick = function(){ applyView('split', true); };
  els.theme.onclick = toggleTheme;
  els.full.onclick = toggleFull;
  $('btnOpen').onclick = function(){ els.file.click(); };
  $('btnOpen2').onclick = function(){ els.file.click(); };
  $('btnPaste').onclick = function(){ applyView('split', true); els.src.focus(); };
  $('btnClear').onclick = function(){
    showEmpty();
    toast('已清空');
  };
  $('btnPrint').onclick = function(){ window.print(); };
  $('vclose').onclick = closeMap;
  els.file.addEventListener('change', function(){
    var files = els.file.files;
    els.file.value = '';
    handleFiles(files);
  });
  window.addEventListener('keydown', function(e){
    if (!(e.ctrlKey || e.metaKey)) {
      if (e.key === 'Escape' && els.viewer.style.display === 'flex') closeMap();
      return;
    }
    var k = (e.key || '').toLowerCase();
    if (k === 'e') { e.preventDefault(); toggleView(); }
    else if (k === 'd') { e.preventDefault(); toggleTheme(); }
    else if (k === 'o') { e.preventDefault(); els.file.click(); }
  });

  // ---------- 初始化：主题先跟随系统一次，之后记住手动选择 ----------
  var savedTheme = lsGet(KEY.theme);
  if (!savedTheme) {
    savedTheme = (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) ? 'dark' : 'light';
  }
  applyTheme(savedTheme, false);
  applyView(lsGet(KEY.view) || 'render', false);
  showEmpty();
  if (window.INIT_DOC) applyDoc(window.INIT_DOC);
  watch();
})();
</script>
</body>
</html>
`

// markdownEditorHTML 把当前文档内嵌进页面，返回可直接发送的 HTML。
func markdownEditorHTML(doc Document) (string, error) {
	payload, err := jsonMarshal(doc)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(markdownEditorPage, docToken, payload), nil
}

// NewMarkdownEditorHandler 构造 Markdown 渲染页服务：
// 根页面 + /api/doc（当前文档）+ /api/events（文档变更 SSE）+ /api/map（地图）+ /assets 静态资源。
// docs 为 nil 时使用内部空文档源，页面仍可独立工作（纯浏览器拖入 / 粘贴）。
func NewMarkdownEditorHandler(docs *DocStore) http.Handler {
	if docs == nil {
		docs = NewDocStore()
	}
	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(Assets()))))
	mux.HandleFunc("/api/doc", docs.serveDoc)
	mux.HandleFunc("/api/events", docs.serveEvents)
	mux.HandleFunc("/api/map/current", docs.serveMapPage)
	mux.HandleFunc("/api/map", handleMapUpload)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		page, err := markdownEditorHTML(docs.Current())
		if err != nil {
			http.Error(w, "生成页面失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, page)
	})
	return mux
}

// serveDoc 返回当前文档快照。
func (s *DocStore) serveDoc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "仅支持 GET", http.StatusMethodNotAllowed)
		return
	}
	payload, err := jsonMarshal(s.Current())
	if err != nil {
		http.Error(w, "序列化失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, payload)
}

// serveMapPage 返回当前地图页 HTML（页面里用 iframe 打开）。
func (s *DocStore) serveMapPage(w http.ResponseWriter, r *http.Request) {
	page, ok := s.MapHTML()
	if !ok {
		http.Error(w, "当前没有地图文档", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, page)
}

// serveEvents 用 SSE 把文档变更推给页面：连接时先补发当前快照，之后只推增量。
func (s *DocStore) serveEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "当前连接不支持流式响应", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, cancel := s.Subscribe()
	defer cancel()

	send := func(doc Document) bool {
		payload, err := jsonMarshal(doc)
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !send(s.Current()) {
		return
	}

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case doc, ok := <-ch:
			if !ok {
				return
			}
			if !send(doc) {
				return
			}
		case <-ticker.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// handleMapUpload 接收页面里拖入的 .pgm(+.yaml)，返回地图编辑器页面。
func handleMapUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "仅支持 POST", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		http.Error(w, "解析上传失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	pgmF, _, err := r.FormFile("pgm")
	if err != nil {
		http.Error(w, "缺少 .pgm 文件（字段名 pgm）", http.StatusBadRequest)
		return
	}
	pgmData, err := io.ReadAll(pgmF)
	_ = pgmF.Close()
	if err != nil {
		http.Error(w, "读取 .pgm 失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	tmp, err := os.MkdirTemp("", "prompt-map-*")
	if err != nil {
		http.Error(w, "创建临时目录失败", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmp)

	pgmPath := filepath.Join(tmp, "map.pgm")
	if err := os.WriteFile(pgmPath, pgmData, 0o644); err != nil {
		http.Error(w, "写入临时文件失败", http.StatusInternalServerError)
		return
	}

	yamlPath := ""
	if yf, _, ferr := r.FormFile("yaml"); ferr == nil {
		yamlData, rerr := io.ReadAll(yf)
		_ = yf.Close()
		if rerr == nil {
			yamlPath = filepath.Join(tmp, "map.yaml")
			_ = os.WriteFile(yamlPath, yamlData, 0o644)
		}
	}

	doc, err := PrepareMap(pgmPath, yamlPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("解析地图失败: %v", err), http.StatusBadRequest)
		return
	}
	page, err := MapHTML(doc)
	if err != nil {
		http.Error(w, "生成地图页失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, page)
}
