package render

import (
	"net/http"
	"strings"
)

// mapPageToken 替换为地图文档 JSON。
const mapPageToken = "__MAP_JSON__"

const mapPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>2D 激光 SLAM 地图</title>
<style>
  html,body{margin:0;height:100%;background:#0d1117;color:#e6edf3;
    font:14px/1.5 -apple-system,"Segoe UI","Microsoft YaHei",sans-serif;overflow:hidden}
  #topbar{display:flex;flex-wrap:wrap;gap:8px;align-items:center;padding:8px 12px;
    background:#161b22;border-bottom:1px solid #30363d}
  #topbar .title{font-weight:600}
  #topbar .muted{color:#8b949e;font-size:12px}
  #topbar .sp{flex:1}
  #topbar button{background:#21262d;border:1px solid #30363d;color:#c9d1d9;border-radius:6px;
    padding:4px 12px;cursor:pointer}
  #topbar button:hover{background:#30363d}
  #topbar button.active{background:#1f6feb;border-color:#1f6feb;color:#fff}
  #topbar label{font-size:12px;color:#8b949e}
  #stage{height:calc(100% - 47px);overflow:auto;display:flex;align-items:center;justify-content:center;
    background:#010409}
  #mapwrap{position:relative;margin:24px}
  #mapwrap canvas{position:absolute;left:0;top:0}
  #mapwrap canvas#ov{cursor:crosshair}
  #tip{padding:0 4px;color:#8b949e;font-size:12px}
  #tip b{color:#e6edf3}
</style>
</head>
<body>
<div id="topbar">
  <span class="title">2D 激光 SLAM 地图</span>
  <span class="muted" id="meta"></span>
  <span class="sp"></span>
  <button id="measureBtn" title="点击两次测量两点直线距离">测量尺子</button>
  <button id="eraseBtn" title="把地图上的障碍物像素擦成空白">障碍物擦除</button>
  <label>橡皮</label><input id="brush" type="range" min="1" max="40" value="8" style="width:120px">
  <button id="saveBtn">保存下载 .pgm + .yaml</button>
</div>
<div id="stage"><div id="mapwrap"><canvas id="map"></canvas><canvas id="ov"></canvas></div></div>
<script id="payload" type="application/json">__MAP_JSON__</script>
<script>
(function(){
  var D = JSON.parse(document.getElementById('payload').textContent);
  var meta = document.getElementById('meta');
  var wrap = document.getElementById('mapwrap');
  var mapCv = document.getElementById('map');
  var ovCv = document.getElementById('ov');
  var mctx = mapCv.getContext('2d');
  var octx = ovCv.getContext('2d');
  var metric = !!(D.info && D.info.resolution > 0);

  var img = new Image();
  img.onload = init;
  img.src = 'data:image/png;base64,' + D.png;

  var scale = 1, cssW = 0, cssH = 0;   // cssW/H = 显示尺寸；画布内部=原图分辨率
  var tool = 'none';                    // none | measure | erase
  var seg = [], curSeg = null, dirty = false;
  var btnM = document.getElementById('measureBtn');
  var btnE = document.getElementById('eraseBtn');
  var brush = document.getElementById('brush');
  var tip = document.createElement('span');

  function fit(){
    var stage = document.getElementById('stage');
    var availW = stage.clientWidth - 60;
    var availH = stage.clientHeight - 80;
    scale = Math.min(availW / D.width, availH / D.height, 3);
    if (scale <= 0) scale = 0.2;
    cssW = Math.round(D.width * scale);
    cssH = Math.round(D.height * scale);
    mapCv.width = D.width; mapCv.height = D.height;
    ovCv.width = cssW; ovCv.height = cssH;
    [mapCv, ovCv].forEach(function(c){ c.style.width = cssW + 'px'; c.style.height = cssH + 'px'; });
    wrap.style.width = cssW + 'px';
    wrap.style.height = cssH + 'px';
    mctx.drawImage(img, 0, 0);
    drawOverlay();
  }

  function init(){
    var resTxt = metric ? (D.info.resolution + ' m/px · ' +
      (D.width * D.info.resolution).toFixed(1) + ' x ' +
      (D.height * D.info.resolution).toFixed(1) + ' m') : '无 .yaml，仅预览/擦除/下载';
    meta.textContent = D.base_name + ' · ' + D.width + 'x' + D.height + ' px · ' + resTxt;
    fit();
    if (!metric) {
      btnM.disabled = true; btnM.title = '缺少 .yaml（resolution），无法换算真实尺寸';
    }
    var stage = document.getElementById('stage');
    stage.appendChild(tip);
  }
  window.addEventListener('resize', fit);

  function pick(ev){
    var r = ovCv.getBoundingClientRect();
    var x = (ev.clientX - r.left) * D.width / r.width;
    var y = (ev.clientY - r.top) * D.height / r.height;
    x = Math.max(0, Math.min(D.width - 1, x));
    y = Math.max(0, Math.min(D.height - 1, y));
    return [x, y];
  }
  function cssP(p){ return [p[0] * scale, p[1] * scale]; }

  // ---------- 工具栏 ----------
  function setTool(t){
    tool = t;
    btnM.classList.toggle('active', t === 'measure');
    btnE.classList.toggle('active', t === 'erase');
    var modeTxt = t === 'measure' ? '测量模式：点地图两点，显示直线距离；再点工具关闭' :
                  t === 'erase'   ? '擦除模式：按住鼠标在障碍物上拖动，把像素刷成空白' :
                  '在地图上按住可平移 / 选择右上工具';
    tip.innerHTML = '<b>' + modeTxt + '</b>';
  }
  btnM.onclick = function(){ setTool(tool === 'measure' ? 'none' : 'measure'); };
  btnE.onclick = function(){ setTool(tool === 'erase' ? 'none' : 'erase'); };

  function niceM(target){
    var mags = [1,2,5,10,20,50,100,200,500];
    for (var i=0;i<mags.length;i++){
      var m = mags[i] * 0.001;
      if (m * (cssW / (D.width * D.info.resolution)) >= target * 0.6) return m;
    }
    return mags[mags.length-1] * 0.001;
  }

  function drawOverlay(){
    octx.clearRect(0,0,cssW,cssH);
    // 比例尺（左下角）
    if (metric) {
      var cssPerM = cssW / (D.width * D.info.resolution);
      var m = niceM(120);
      var bar = Math.round(m * cssPerM);
      var x = 14, y = cssH - 24;
      octx.strokeStyle = 'rgba(255,255,255,0.9)';
      octx.fillStyle = 'rgba(255,255,255,0.9)';
      octx.lineWidth = 2;
      octx.beginPath(); octx.moveTo(x,y); octx.lineTo(x+bar,y); octx.stroke();
      octx.beginPath(); octx.moveTo(x,y-5); octx.lineTo(x,y+5); octx.stroke();
      octx.beginPath(); octx.moveTo(x+bar,y-5); octx.lineTo(x+bar,y+5); octx.stroke();
      var txt = m >= 1 ? m + ' m' : (m*1000) + ' cm';
      octx.fillText(txt, x, y - 8);
    }
    // 测量结果
    octx.strokeStyle = '#ffa657'; octx.fillStyle = '#ffa657';
    octx.lineWidth = 2;
    seg.concat(curSeg ? [curSeg] : []).forEach(function(s){
      var a = cssP([s[0],s[1]]), b = cssP([s[2],s[3]]);
      octx.beginPath(); octx.moveTo(a[0],a[1]); octx.lineTo(b[0],b[1]); octx.stroke();
      octx.beginPath(); octx.arc(a[0],a[1],4,0,Math.PI*2); octx.fill();
      octx.beginPath(); octx.arc(b[0],b[1],4,0,Math.PI*2); octx.fill();
      if (metric) {
        var dist = Math.hypot((s[2]-s[0])*D.info.resolution, (s[3]-s[1])*D.info.resolution);
        octx.fillText(dist.toFixed(2) + ' m', (a[0]+b[0])/2 + 8, (a[1]+b[1])/2 - 8);
      }
    });
  }

  // ---------- 指针事件（统一在 overlay 上） ----------
  var painting = false;
  var eraseColor = (D.info && D.info.negate !== 0) ? 0 : 255; // 常规地图白色=空白

  ovCv.addEventListener('pointerdown', function(ev){
    ev.preventDefault();
    ovCv.setPointerCapture(ev.pointerId);
    painting = true;
    var p = pick(ev);
    if (tool === 'measure') {
      if (!curSeg) { curSeg = [p[0],p[1],p[0],p[1]]; }
      else { seg.push(curSeg); curSeg = null; drawOverlay(); }
    } else if (tool === 'erase') {
      eraseAt(p[0], p[1]);
    }
  });
  ovCv.addEventListener('pointermove', function(ev){
    var p = pick(ev);
    if (tool === 'measure' && curSeg) {
      curSeg[2] = p[0]; curSeg[3] = p[1]; drawOverlay();
    } else if (tool === 'erase' && painting) {
      eraseAt(p[0], p[1]);
    }
  });
  function endDrag(ev){
    if (tool === 'erase') painting = false;
    if (tool === 'measure') { /* 保持临时线直到下一次点击确定 */ }
  }
  ovCv.addEventListener('pointerup', endDrag);
  ovCv.addEventListener('pointercancel', endDrag);
  ovCv.addEventListener('dblclick', function(){ seg = []; curSeg = null; drawOverlay(); setTool('none'); });

  function eraseAt(x, y){
    dirty = true;
    var R = Math.max(1, parseInt(brush.value, 10) || 8);
    mctx.fillStyle = 'rgb(' + eraseColor + ',' + eraseColor + ',' + eraseColor + ')';
    mctx.beginPath();
    mctx.arc(x, y, R, 0, Math.PI * 2);
    mctx.fill();
  }

  // ---------- 保存下载 ----------
  function trigger(name, blob){
    var a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = name;
    document.body.appendChild(a); a.click();
    setTimeout(function(){ URL.revokeObjectURL(a.href); a.remove(); }, 800);
  }

  document.getElementById('saveBtn').onclick = function(){
    var base = D.base_name + '_edited';
    var fname = base + '.pgm';
    var px = mctx.getImageData(0, 0, D.width, D.height).data;
    var out = new Uint8Array(D.width * D.height);
    for (var i = 0; i < out.length; i++) out[i] = px[i*4]; // 灰度
    var head = 'P5\n# edited with PromptMerger Viewer\n' + D.width + ' ' + D.height + '\n255\n';
    var bytes = new Uint8Array(head.length + out.length);
    for (var k = 0; k < head.length; k++) bytes[k] = head.charCodeAt(k) & 0xff;
    bytes.set(out, head.length);
    trigger(fname, new Blob([bytes], {type:'image/x-portable-anymap'}));
    if (D.info && D.info.raw) {
      var yaml = D.info.raw.replace(/^image\s*:.*$/m, 'image: ' + fname);
      trigger(base + '.yaml', new Blob([yaml], {type:'text/yaml'}));
    }
  };
})();
</script>
</body>
</html>
`

// MapHTML 生成带地图文档内嵌数据的完整页面。
func MapHTML(doc *MapDoc) (string, error) {
	payload, err := jsonMarshal(doc)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(mapPageHTML, mapPageToken, payload), nil
}

// NewMapHandler 构造地图页 HTTP handler。
func NewMapHandler(doc *MapDoc) (http.Handler, error) {
	page, err := MapHTML(doc)
	if err != nil {
		return nil, err
	}
	return NewPageHandler(page), nil
}
