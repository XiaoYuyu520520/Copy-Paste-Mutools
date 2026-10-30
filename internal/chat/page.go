package chat

// indexHTML 浏览器端 AI 聊天页。布局/交互参考 RikkaHub Web UI 主对话形态。
//
// 额外能力：
//   - 历史会话服务端记忆（/api/history），跨随机端口可恢复；左侧可新建/删除/切换
//   - 流式时实时展示模型「思考过程(思维链)」（reasoning_content），可折叠
//   - 每条助手回复后展示：首字延迟(含思维链时间) / 思考耗时 / 生成耗时 / 输入·输出 tokens / tokens 速度
//
// 说明：本文件为原始 HTML；为避免与 Go 原始字符串冲突，页面内 JS 不使用反引号模板串。
const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>AI 聊天</title>
<link rel="stylesheet" href="/assets/github.min.css" id="hlLight">
<link rel="stylesheet" href="/assets/github-dark.min.css" id="hlDark" disabled>
<link rel="stylesheet" href="/assets/github-markdown.min.css">
<style>
:root{
  --bg:#ffffff; --fg:#18181b; --muted:#52525b; --muted-2:#a1a1aa;
  --card:#ffffff; --border:#e4e4e7; --sidebar:#fafafa; --hover:#f4f4f5;
  --accent:#f4f4f5; --primary:#18181b; --primary-fg:#fafafa;
  --bubble:#f4f4f5; --danger:#ef4444; --ring:#18181b;
}
:root.dark{
  --bg:#0c0c0c; --fg:#e4e4e7; --muted:#a1a1aa; --muted-2:#71717a;
  --card:#0c0c0c; --border:#27272a; --sidebar:#171717; --hover:#262626;
  --accent:#262626; --primary:#e4e4e7; --primary-fg:#18181b;
  --bubble:#262626; --danger:#f87171; --ring:#e4e4e7;
}
*{box-sizing:border-box}
html,body{margin:0;height:100%}
body{background:var(--bg);color:var(--fg);font:14px/1.55 -apple-system,"Segoe UI","Microsoft YaHei",Roboto,sans-serif}
button{font:inherit;cursor:pointer;border:0;background:transparent;color:inherit}
input,textarea,select{font:inherit;color:inherit}
#app{display:flex;height:100vh;overflow:hidden}

/* ---------- 左侧会话栏 ---------- */
#side{width:272px;min-width:272px;background:var(--sidebar);border-right:1px solid var(--border);
  display:flex;flex-direction:column}
#sideTop{padding:12px 10px 8px;display:flex;gap:8px;align-items:center}
#sideTop h1{font-size:16px;margin:0;flex:1;font-weight:650}
#newBtn{display:flex;align-items:center;gap:6px;background:var(--primary);color:var(--primary-fg);
  border-radius:8px;padding:7px 12px;font-size:13px;font-weight:500}
#newBtn:hover{opacity:.92}
#convList{flex:1;overflow:auto;padding:6px 8px}
#convList .empty{padding:18px 10px;color:var(--muted-2);font-size:13px;text-align:center}
.conv{display:flex;align-items:center;gap:8px;padding:9px 10px;border-radius:8px;cursor:pointer;
  color:var(--fg)}
.conv:hover{background:var(--hover)}
.conv.active{background:var(--accent)}
.conv .t{flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:13.5px}
.conv .meta{color:var(--muted-2);font-size:11px}
.conv .del{display:none;color:var(--muted-2);padding:2px 5px;border-radius:6px;font-size:14px}
.conv:hover .del{display:inline}
.conv .del:hover{color:var(--danger);background:var(--hover)}
#sideFoot{padding:10px;border-top:1px solid var(--border);color:var(--muted-2);font-size:12px;line-height:1.5}

/* ---------- 主区 ---------- */
#main{flex:1;display:flex;flex-direction:column;min-width:0}
#topbar{display:flex;align-items:center;gap:10px;padding:10px 18px;border-bottom:1px solid var(--border);background:var(--bg)}
#title{font-size:15px;font-weight:600;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
#cfgHint{color:var(--muted-2);font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
#topbar .sp{flex:1}
#modelWrap{display:flex;align-items:center;gap:6px;border:1px solid var(--border);border-radius:8px;padding:2px 6px;background:var(--card)}
#modelWrap label{color:var(--muted-2);font-size:12px}
#modelSel{border:0;outline:0;background:transparent;max-width:190px;color:var(--fg);font-size:13px;padding:4px 0}
.iconBtn{display:inline-flex;align-items:center;justify-content:center;width:30px;height:30px;border-radius:8px;color:var(--muted)}
.iconBtn:hover{background:var(--accent);color:var(--fg)}

#banner{display:none;padding:8px 18px;background:#fef3c7;color:#92400e;font-size:13px;border-bottom:1px solid #fde68a}
:root.dark #banner{background:#3f2d04;color:#fbbf24;border-color:#5b4308}

/* ---------- 消息流 ---------- */
#thread{flex:1;overflow:auto}
#inner{max-width:860px;margin:0 auto;padding:18px 24px 30px;display:flex;flex-direction:column;gap:20px}
#welcome{flex:1;display:flex;flex-direction:column;align-items:center;justify-content:center;color:var(--muted-2);text-align:center;padding:40px}
#welcome .logo{width:52px;height:52px;border-radius:14px;background:linear-gradient(135deg,#6366f1,#8b5cf6);
  display:flex;align-items:center;justify-content:center;color:#fff;font-size:22px;font-weight:700;margin-bottom:14px}
.msg{display:flex;flex-direction:column;gap:8px}
.msg.user{align-items:flex-end}
.msg.ai{align-items:stretch}
.mrow{width:100%;display:flex}
.msg.user .mrow{justify-content:flex-end}
.msg.ai .mrow{justify-content:flex-start}
.avatar{width:30px;height:30px;border-radius:50%;flex-shrink:0;display:flex;align-items:center;justify-content:center;font-size:13px;font-weight:600}
.msg.ai .avatar{margin-right:10px;background:linear-gradient(135deg,#6366f1,#8b5cf6);color:#fff}
.msg.user .avatar{margin-left:10px;background:var(--accent);color:var(--muted);order:2}
.bubble{font-size:14px;min-width:0}
.msg.user .bubble{background:var(--bubble);border-radius:14px;border-bottom-right-radius:4px;padding:9px 14px;max-width:78%;white-space:pre-wrap;word-break:break-word}
.msg.ai .bubble{max-width:100%}
.mname{font-size:12.5px;color:var(--muted);margin-bottom:4px;display:flex;align-items:center;gap:8px}
.mname b{color:var(--fg);font-weight:600}
.mname .model{color:var(--muted-2);font-size:11px;border:1px solid var(--border);border-radius:999px;padding:0 8px}
.err .bubble{color:var(--danger)}
.msg .actions{display:flex;gap:2px;margin-top:6px}
.msg.user .actions{justify-content:flex-end;padding-right:40px}
.msg.ai .actions{padding-left:40px}
.msg .actions button{color:var(--muted-2);font-size:12px;padding:3px 8px;border-radius:6px;display:inline-flex;align-items:center;gap:4px}
.msg .actions button:hover{color:var(--fg);background:var(--accent)}
.mdown{max-width:100%}
.mdown pre{position:relative;background:transparent}
.mdown pre code.hljs{display:block;overflow-x:auto;padding:10px 12px;background:transparent}
.mdown pre:hover code.hljs{background:var(--hover)}
.mdown :not(pre)>code{background:rgba(140,140,150,.18);border-radius:5px;padding:1px 5px;font-size:.92em}
:root.dark .mdown :not(pre)>code{background:rgba(255,255,255,.12)}
.mdown pre{position:relative}
.mdown .copy{position:absolute;top:6px;right:6px;opacity:0;font-size:12px;background:#f6f8fa;color:#24292f;border:1px solid #d0d7de;border-radius:6px;padding:2px 8px;transition:opacity .15s}
.mdown pre:hover .copy{opacity:1}
.cursor{display:inline-block;width:8px;height:15px;background:var(--muted-2);vertical-align:text-bottom;animation:blink 1s steps(2) infinite}
@keyframes blink{50%{opacity:0}}
.stats{display:flex;flex-wrap:wrap;gap:2px 14px;color:var(--muted-2);font-size:11.5px;margin-top:6px;padding-left:40px}
.stats .k{color:var(--muted)}
/* 思维链 */
.think{margin:0 0 8px;border:1px solid var(--border);border-left:3px solid #8b5cf6;border-radius:8px;
  background:var(--sidebar);overflow:hidden}
.think summary{cursor:pointer;padding:7px 10px;color:var(--muted);font-size:12.5px;user-select:none;display:flex;align-items:center;gap:6px}
.think summary .chev{margin-left:auto}
.think .body{margin:0 10px 8px;padding:8px 10px;font-size:13px;color:var(--muted);
  white-space:pre-wrap;word-break:break-word;border-top:1px dashed var(--border)}
.think summary::-webkit-details-marker{display:none}

/* ---------- 底部输入 ---------- */
#composerWrap{padding:8px 18px 16px;background:var(--bg)}
#composer{max-width:860px;margin:0 auto;border:1px solid var(--border);border-radius:16px;background:var(--card);padding:8px 8px 6px;box-shadow:0 1px 2px rgba(0,0,0,.04)}
#ta{width:100%;border:0;outline:0;resize:none;background:transparent;padding:6px 10px;font-size:14.5px;line-height:1.5;min-height:46px;max-height:200px;color:var(--fg)}
#composerBar{display:flex;align-items:center;justify-content:space-between;padding:0 4px 2px}
#hint{color:var(--muted-2);font-size:12px}
#sendBtn{display:inline-flex;align-items:center;gap:6px;background:var(--primary);color:var(--primary-fg);border-radius:9px;padding:7px 16px;font-size:13.5px;font-weight:500}
#sendBtn:disabled{opacity:.4;cursor:not-allowed}
#sendBtn.stop{background:transparent;color:var(--danger);border:1px solid var(--danger)}
.toast{position:fixed;bottom:20px;left:50%;transform:translateX(-50%);background:var(--fg);color:var(--bg);padding:8px 14px;border-radius:8px;font-size:13px;opacity:0;transition:opacity .2s;z-index:50;max-width:70vw}
.toast.show{opacity:1}
.markdown-body{font-size:14px;background:transparent;color:var(--fg)}
</style>
</head>
<body>
<div id="app">
  <aside id="side">
    <div id="sideTop"><h1>AI 对话</h1><button id="newBtn" title="新对话">＋ 新对话</button></div>
    <div id="convList"></div>
    <div id="sideFoot">对话自动保存（服务端记忆）；删除即在左侧点 ×。</div>
  </aside>
  <main id="main">
    <div id="topbar">
      <span id="title">新对话</span>
      <span id="cfgHint"></span>
      <span class="sp"></span>
      <div id="modelWrap"><label>模型</label><select id="modelSel" title="选择模型"></select></div>
      <button id="themeBtn" class="iconBtn" title="切换亮/暗">◐</button>
    </div>
    <div id="banner"></div>
    <div id="thread"><div id="inner"></div></div>
    <div id="composerWrap">
      <div id="composer">
        <textarea id="ta" placeholder="给 AI 发消息…（Enter 发送，Shift+Enter 换行）" rows="1"></textarea>
        <div id="composerBar">
          <span id="hint">Enter 发送 · Shift+Enter 换行</span>
          <button id="sendBtn">发送 ↵</button>
        </div>
      </div>
    </div>
  </main>
</div>
<div id="toast" class="toast"></div>
<script src="/assets/marked.min.js"></script>
<script src="/assets/highlight.min.js"></script>
<script>
(function(){
'use strict';
var LS='aichat.convs.v2';
var store={convs:[], active:null, busy:false, abort:null, theme:'light'};
var $=function(id){return document.getElementById(id);};
var els={list:$('convList'), title:$('title'), ta:$('ta'), send:$('sendBtn'),
  inner:$('inner'), thread:$('thread'), modelSel:$('modelSel'), cfgHint:$('cfgHint'),
  banner:$('banner'), themeBtn:$('themeBtn')};

/* ---------- 会话持久化（本地 + 服务端，跨随机端口） ---------- */
function loadAll(){try{var raw=localStorage.getItem(LS);if(raw){var d=JSON.parse(raw);if(d&&Array.isArray(d.convs)){store.convs=sanitizeList(d.convs);store.theme=d.theme||'light';}}}catch(e){} }
function sanitizeList(list){return list.map(function(c){
  if(c&&Array.isArray(c.messages)){c.messages=c.messages.map(function(m){
    if(m&&m.pending){m.pending=false;} return m;});}
  return c;});}
function saveAll(){try{localStorage.setItem(LS,JSON.stringify({convs:store.convs,theme:store.theme}));}catch(e){} persistServer();}
var histTimer=null;
function persistServer(){clearTimeout(histTimer);histTimer=setTimeout(function(){
  try{ fetch('/api/history',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(store.convs)}).catch(function(){}); }catch(e){}
},500);}
function loadServerHistory(){
  fetch('/api/history').then(function(r){return r.json();}).then(function(list){
    if(Array.isArray(list)&&list.length){
      store.convs=sanitizeList(list);
      if(store.active){var hit=false;store.convs.forEach(function(c){if(c.id===store.active)hit=true;});if(!hit)store.active=null;}
      saveAll();renderSidebar();renderThread();
    }
  }).catch(function(){});
}
function findConv(id){for(var i=0;i<store.convs.length;i++){if(store.convs[i].id===id)return store.convs[i];}return null;}
function mkId(){return Date.now().toString(36)+Math.random().toString(36).slice(2,7);}

function renderSidebar(){
  els.list.innerHTML='';
  if(!store.convs.length){els.list.innerHTML='<div class="empty">还没有对话，点上方「新对话」开始</div>';return;}
  store.convs.slice().sort(function(a,b){return b.updated-a.updated;}).forEach(function(c){
    var row=document.createElement('div');row.className='conv'+(c.id===store.active?' active':'');
    var t=document.createElement('span');t.className='t';t.textContent=c.title||'新对话';
    var meta=document.createElement('span');meta.className='meta';
    var d=new Date(c.updated||Date.now());
    meta.textContent=(d.getMonth()+1)+'/'+d.getDate();
    var del=document.createElement('button');del.className='del';del.textContent='×';del.title='删除对话';
    del.addEventListener('click',function(ev){ev.stopPropagation();store.convs=store.convs.filter(function(x){return x.id!==c.id;});
      if(store.active===c.id){store.active=null;renderThread();}saveAll();renderSidebar();toast('已删除');});
    row.appendChild(t);row.appendChild(meta);row.appendChild(del);
    row.addEventListener('click',function(){store.active=c.id;saveAll();renderSidebar();renderThread();els.ta.focus();});
    els.list.appendChild(row);
  });
}
function renderTitle(c){els.title.textContent=c?(c.title||'新对话'):'新对话';}

/* ---------- 消息渲染 ---------- */
function md2html(src){
  var html='';
  try{html=marked.parse(src||'',{gfm:true,breaks:false});}catch(e){html='<p>'+e.message+'</p>';}
  var wrap=document.createElement('div');wrap.innerHTML=html;
  wrap.querySelectorAll('pre').forEach(function(pre){
    var code=pre.querySelector('code');
    if(code&&window.hljs){try{hljs.highlightElement(code);}catch(e){}}
    if(pre.querySelector('.copy'))return;
    var btn=document.createElement('button');btn.className='copy';btn.textContent='复制';
    btn.onclick=function(){copyText(code?code.innerText:pre.innerText,btn);};
    pre.appendChild(btn);
  });
  return wrap.innerHTML;
}
function copyText(text,btn){
  function ok(){if(btn){btn.textContent='已复制 ✓';setTimeout(function(){btn.textContent='复制';},900);}}
  if(navigator.clipboard&&navigator.clipboard.writeText){navigator.clipboard.writeText(text).then(ok,function(){fb();});}else{fb();}
  function fb(){var ta=document.createElement('textarea');ta.value=text;document.body.appendChild(ta);ta.select();
    try{document.execCommand('copy');ok();}catch(e){}document.body.removeChild(ta);}
}
function sec(ms){if(ms==null)return '';return (ms/1000).toFixed(1)+'s';}
function isCJK(cp){return (cp>=0x4E00&&cp<=0x9FFF)||(cp>=0x3400&&cp<=0x4DBF)||(cp>=0xF900&&cp<=0xFAFF)||(cp>=0x20000&&cp<=0x2A6DF);}
// 估算 token 数：中文≈1 tok/字，其它≈3.5 字符/tok（服务端不返回 usage 时的兜底）
function estimateTokens(text){
  if(!text)return 0;
  var cjk=0,other=0;
  var arr=Array.from(text);
  for(var i=0;i<arr.length;i++){
    var cp=arr[i].codePointAt(0);
    if(isCJK(cp))cjk++;else if(!/\s/.test(arr[i]))other++;
  }
  return Math.max(1,Math.ceil(cjk + other/3.5));
}
function renderStats(m){
  var s=m.stats;if(!s)return '';
  var parts=[];
  if(s.first_ms!=null)parts.push('<span><span class="k">首字</span> '+sec(s.first_ms)+'</span>');
  if(s.think_ms!=null)parts.push('<span><span class="k">思考</span> '+sec(s.think_ms)+'</span>');
  if(s.gen_ms!=null)parts.push('<span><span class="k">生成</span> '+sec(s.gen_ms)+'</span>');
  if(!s.est){
    if(s.prompt_tokens!=null&&s.completion_tokens!=null)parts.push('<span><span class="k">tokens</span> ↑'+s.prompt_tokens+' / ↓'+s.completion_tokens+'</span>');
    if(s.tps!=null)parts.push('<span><span class="k">速度</span> '+s.tps.toFixed(1)+' tok/s</span>');
  }else{
    if(s.completion_tokens!=null)parts.push('<span><span class="k">tokens</span> ↓'+s.completion_tokens+'（估）</span>');
    if(s.tps!=null)parts.push('<span><span class="k">速度</span> '+s.tps.toFixed(1)+' tok/s（估）</span>');
  }
  return parts.join('')? '<div class="stats">'+parts.join('')+'</div>':'';
}
function messageEl(m,idx){
  var box=document.createElement('div');box.className='msg '+(m.role==='user'?'user':'ai')+(m.error?' err':'');
  var row=document.createElement('div');row.className='mrow';
  if(m.role!=='user'){
    var av=document.createElement('div');av.className='avatar';av.textContent='AI';row.appendChild(av);
  }
  var mid=document.createElement('div');
  if(m.role!=='user'){
    var nm=document.createElement('div');nm.className='mname';
    nm.innerHTML='<b>AI 助手</b>'+(m.model?'<span class="model">'+esc(m.model)+'</span>':'');
    mid.appendChild(nm);
  }
  var bubble=document.createElement('div');bubble.className='bubble';
  if(m.role==='user'){
    bubble.textContent=m.content;
  }else if(m.error){
    bubble.textContent=(m.content||'发生错误');
  }else{
    if(m.reasoning){
      var det=document.createElement('details');det.className='think';
      det.open=!!m.pending;
      var sum=document.createElement('summary');
      sum.innerHTML='<span>🧠 思考过程</span><span class="chev">'+(det.open?'▾':'▸')+'</span>';
      sum.addEventListener('click',function(e){requestAnimationFrame(function(){sum.querySelector('.chev').textContent=det.open?'▾':'▸';});});
      det.appendChild(sum);
      var body=document.createElement('div');body.className='body';body.textContent=m.reasoning;
      det.appendChild(body);
      bubble.appendChild(det);
    }
    var md=document.createElement('div');md.className='mdown markdown-body';
    md.innerHTML=md2html(m.content);
    bubble.appendChild(md);
    if(m.pending&&!m.content){var c=document.createElement('span');c.className='cursor';bubble.appendChild(c);}
  }
  mid.appendChild(bubble);
  row.appendChild(mid);
  if(m.role==='user'){
    var uav=document.createElement('div');uav.className='avatar';uav.textContent='我';row.appendChild(uav);
  }
  box.appendChild(row);
  if(m.role!=='user'&&!m.pending&&!m.error){
    var acts=document.createElement('div');acts.className='actions';
    var cp=document.createElement('button');cp.textContent='⧉ 复制';cp.onclick=function(){copyText(m.content);toast('已复制回答');};
    var rg=document.createElement('button');rg.textContent='↻ 重新生成';rg.onclick=function(){regenerate(idx);};
    acts.appendChild(cp);acts.appendChild(rg);box.appendChild(acts);
    var st=document.createElement('div');st.innerHTML=renderStats(m);if(st.childNodes.length){box.appendChild(st);}
  }
  return box;
}
function refreshThread(){
  els.inner.innerHTML='';
  var c=findConv(store.active);
  if(!c){return;}
  c.messages.forEach(function(mm,i){els.inner.appendChild(messageEl(mm,i));});
  scrollBottom();
}
function renderThread(){
  var c=findConv(store.active);
  renderTitle(c);
  if(!c){els.inner.innerHTML='<div id="welcome"><div class="logo">AI</div><div>开始一段新的对话吧</div><div style="margin-top:6px;color:var(--muted-2)">下方输入消息，Enter 发送</div></div>';return;}
  refreshThread();
}
function scrollBottom(){els.thread.scrollTop=els.thread.scrollHeight;}
function esc(s){return String(s==null?'':s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');}

/* ---------- 新对话 ---------- */
function newConv(){var c={id:mkId(),title:'新对话',updated:Date.now(),messages:[],model:els.modelSel.value};store.convs.push(c);store.active=c.id;saveAll();renderSidebar();renderThread();els.ta.focus();}

/* ---------- 发送 / 流式 ---------- */
function pushMsg(role,content,extra){
  var c=findConv(store.active);if(!c)return null;
  var m={role:role,content:content};if(extra)for(var k in extra)m[k]=extra[k];
  c.messages.push(m);touch(c);saveAll();return m;
}
function touch(c){c.updated=Date.now();var t=c.messages[0]?c.messages[0].content:'';c.title=(t||'新对话').slice(0,28);renderTitle(c);}
function setBusy(b){store.busy=b;els.send.innerHTML=b?'停止':'发送 ↵';els.send.classList.toggle('stop',b);els.ta.disabled=b;}
function send(){
  if(store.busy)return stopNow();
  var text=els.ta.value.trim();
  if(!text){toast('先输入内容');return;}
  if(!store.active)newConv();
  els.ta.value='';autoGrow();
  pushMsg('user',text);
  var am=pushMsg('assistant','',{pending:true,model:els.modelSel.value});
  startChat(am);
}
function regenerate(idx){
  var c=findConv(store.active);if(!c)return;
  if(store.busy)return;
  if(idx<0||idx>=c.messages.length)return;
  if(c.messages[idx].role!=='assistant')return;
  c.messages.length=idx;saveAll();refreshThread();
  var am=pushMsg('assistant','',{pending:true,model:els.modelSel.value});
  refreshThread();startChat(am);
}
function startChat(am){
  var c=findConv(store.active);
  var msgs=[];
  c.messages.forEach(function(m){if(m!==am)msgs.push({role:m.role==='error'?'assistant':m.role,content:m.content});});
  am._t0=Date.now();am.reasoning='';am._rs=null;am._cs=null;am.stats=null;
  setBusy(true);
  var ctl=new AbortController();store.abort=ctl;
  fetch('/api/chat',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({model:els.modelSel.value||'',messages:msgs}),signal:ctl.signal})
  .then(function(res){
    if(!res.ok){return res.text().then(function(t){throw new Error(t||('HTTP '+res.status));});}
    var reader=res.body.getReader();var dec=new TextDecoder();var buf='';
    function pump(){return reader.read().then(function(r){
      if(r.done){finalize(am,false);return;}
      buf+=dec.decode(r.value,{stream:true});
      var i;while((i=buf.indexOf('\n'))>=0){var line=buf.slice(0,i);buf=buf.slice(i+1);handleLine(am,line);}
      return pump();
    });}
    return pump();
  }).catch(function(err){
    if(err&&err.name==='AbortError'){finalize(am,true);}
    else{am.pending=false;am.error=true;am.content=(am.content||'')+String(err&&err.message?err.message:err);
      am.model=els.modelSel.value;finalizeStats(am);saveAll();refreshThread();setBusy(false);}
  });
}
function handleLine(am,line){
  line=line.trim();if(!line||line.indexOf('data:')!==0)return;
  var raw=line.slice(5).trim();if(!raw)return;
  var ev;try{ev=JSON.parse(raw);}catch(e){return;}
  if(ev.type==='reasoning'&&ev.content){
    am.reasoning+=ev.content;
    if(am._rs==null)am._rs=Date.now();
    saveAll();refreshThread();
  }
  else if(ev.type==='delta'&&ev.content){
    if(am._cs==null)am._cs=Date.now();
    am.pending=false;am.error=false;am.content+=ev.content;
    saveAll();refreshThread();
  }
  else if(ev.type==='usage'){
    if(ev.prompt_tokens!=null)am.prompt_tokens=ev.prompt_tokens;
    if(ev.completion_tokens!=null)am.completion_tokens=ev.completion_tokens;
  }
  else if(ev.type==='error'){
    am.pending=false;am.error=true;am.content=(am.content||'')+String(ev.content||'请求出错');
    finalizeStats(am);saveAll();refreshThread();setBusy(false);
  }
  else if(ev.type==='done'){finalize(am,false);}
}
function finalize(am,stopped){
  if(store.abort){try{store.abort.abort();}catch(e){}store.abort=null;}
  am.pending=false;am.error=!!am.error;
  if(!am.content&&!am.reasoning)am.content=stopped?'（已停止）':'（无内容）';
  finalizeStats(am);saveAll();setBusy(false);refreshThread();
}
function finalizeStats(am){
  var now=Date.now();
  var st={};
  if(am._t0){var total=now-am._t0;
    st.first_ms=(am._cs!=null)?am._cs-am._t0:(total); // 首字延迟（含思维链时间）
    if(am._rs!=null&&am._cs!=null)st.think_ms=am._cs-am._rs;
    else if(am._rs!=null)st.think_ms=now-am._rs;
    st.gen_ms=(am._cs!=null)?now-am._cs:null;
  }
  if(am.prompt_tokens!=null)st.prompt_tokens=am.prompt_tokens;
  if(am.completion_tokens!=null)st.completion_tokens=am.completion_tokens;
  else if(!am.error){ // 服务端没回 usage 时用文本估算
    var est=estimateTokens(am.reasoning||'')+estimateTokens(am.content||'');
    st.completion_tokens=est;st.est=true;
  }
  if(st.completion_tokens&&st.gen_ms){st.tps=st.completion_tokens/(st.gen_ms/1000);}
  else if(st.completion_tokens&&am._t0){st.tps=st.completion_tokens/((now-am._t0)/1000);}
  am.stats=st;
}
function stopNow(){if(store.abort){store.abort.abort();store.abort=null;}}

/* ---------- 配置 / 模型 ---------- */
function bootCfg(){
  fetch('/api/config').then(function(r){return r.json();}).then(function(cfg){
    if(!cfg.configured){els.banner.style.display='block';els.banner.textContent='未配置 API 与模型：请回到桌面程序，点击左侧「AI 设置」填写 API 地址 / Key / 模型。';}
    else{els.banner.style.display='none';els.cfgHint.textContent=cfg.base_url+' · '+cfg.model;}
    var d=cfg.model||'';
    if(d){var o=document.createElement('option');o.value=d;o.textContent=d;els.modelSel.appendChild(o);}
    fetch('/api/models').then(function(r){return r.json();}).then(function(b){
      var list=b.models||[];
      list.forEach(function(id){if(id!==d&&els.modelSel.querySelector('option[value="'+cssq(id)+'"]')===null){var op=document.createElement('option');op.value=id;op.textContent=id;els.modelSel.appendChild(op);}});
    }).catch(function(){});
  }).catch(function(){});
}
function cssq(v){return String(v).replace(/["\\]/g,'');}

/* ---------- 主题 ---------- */
function applyTheme(){var dark=(store.theme==='dark');
  document.documentElement.classList.toggle('dark',dark);
  var hL=document.getElementById('hlLight'),hD=document.getElementById('hlDark');
  if(hL)hL.disabled=dark;
  if(hD)hD.disabled=!dark;
  els.themeBtn.textContent=dark?'☀':'☾';}
function toggleTheme(){store.theme=store.theme==='dark'?'light':'dark';saveAll();applyTheme();}

/* ---------- 工具 ---------- */
var toastTimer=null;
function toast(msg){var t=$('toast');t.textContent=msg;t.classList.add('show');clearTimeout(toastTimer);
  toastTimer=setTimeout(function(){t.classList.remove('show');},2600);}
function autoGrow(){els.ta.style.height='auto';els.ta.style.height=Math.min(200,els.ta.scrollHeight)+'px';}

/* ---------- 事件 ---------- */
$('newBtn').onclick=newConv;
els.send.onclick=send;
els.themeBtn.onclick=toggleTheme;
els.ta.addEventListener('input',autoGrow);
els.ta.addEventListener('keydown',function(e){
  if(e.key==='Enter'&&!e.shiftKey){e.preventDefault();send();}
});
function initTheme(){try{var saved=localStorage.getItem(LS);if(saved){var d=JSON.parse(saved);if(d.theme)store.theme=d.theme;}}catch(e){}applyTheme();}

/* ---------- 启动 ---------- */
initTheme();loadAll();renderSidebar();renderThread();loadServerHistory();bootCfg();
})();
</script>
</body>
</html>
`
