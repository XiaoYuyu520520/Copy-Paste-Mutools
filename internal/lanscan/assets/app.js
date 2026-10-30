(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const LS_OPTIONS_KEY = 'lanscan.options.v1';

  // --- Persisted user options ------------------------------------------

  function loadSavedOptions() {
    try {
      const raw = localStorage.getItem(LS_OPTIONS_KEY);
      return raw ? JSON.parse(raw) : null;
    } catch (e) {
      return null;
    }
  }

  function saveOptions() {
    const data = {
      sig: state.sig,
      iface: els.ifaceSelect.value,
      cidr: els.cidrInput.value.trim(),
      use_tcp: els.optTcp.checked,
      lookup_mac: els.optMac.checked,
      lookup_host: els.optHost.checked,
      ssh_mode: els.optSsh.checked,
      workers: els.optWorkers.value,
      timeout: els.optTimeout.value,
      ports: els.optPorts.value,
    };
    try {
      localStorage.setItem(LS_OPTIONS_KEY, JSON.stringify(data));
    } catch (e) { /* storage unavailable; ignore */ }
    persistServerPrefs(data);
  }

  // 服务端记忆：本工具每次打开都是新的随机端口（localStorage 按端口隔离），
  // 因此把选项同步存到后端文件，下次无论哪个端口都能记住。
  let prefsTimer = null;
  function persistServerPrefs(data) {
    if (prefsTimer) clearTimeout(prefsTimer);
    prefsTimer = setTimeout(() => {
      try {
        fetch('/api/prefs', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(data),
        }).catch(() => { /* 忽略写失败 */ });
      } catch (e) { /* ignore */ }
    }, 350);
  }

  async function loadServerPrefs() {
    try {
      const res = await fetch('/api/prefs');
      if (!res.ok) return null;
      const p = await res.json();
      return (p && typeof p === 'object' && Object.keys(p).length > 0) ? p : null;
    } catch (e) {
      return null;
    }
  }
  const els = {
    ifaceSelect: $('ifaceSelect'),
    ifaceHint:   $('ifaceHint'),
    refreshIfaces: $('refreshIfaces'),
    cidrInput:   $('cidrInput'),
    startBtn:    $('startBtn'),
    cancelBtn:   $('cancelBtn'),
    exportBtn:   $('exportBtn'),
    progressText:$('progressText'),
    progressCount: $('progressCount'),
    progressFill: $('progressFill'),
    aliveCount:  $('aliveCount'),
    elapsed:     $('elapsed'),
    currentCidr: $('currentCidr'),
    hostsBody:   $('hostsBody'),
    connState:   $('connState'),
    connText:    $('connText'),
    toast:       $('toast'),
    optTcp:      $('optTcp'),
    optMac:      $('optMac'),
    optHost:     $('optHost'),
    optSsh:      $('optSsh'),
    optWorkers:  $('optWorkers'),
    optTimeout:  $('optTimeout'),
    optPorts:    $('optPorts'),
    resetOptsBtn: $('resetOptsBtn'),
  };

  const state = {
    interfaces: [],
    scanId: null,
    es: null,
    hosts: new Map(), // ip -> host
    order: [],        // discovery order
    sortKey: 'ip',
    sortDir: 'asc',
    startedAt: 0,
    elapsedTimer: null,
    cfg: null,    // server defaults from /api/config
    saved: null,  // remembered options from localStorage
  };

  function setConn(state, text) {
    els.connState.className = 'dot dot-' + state;
    els.connText.textContent = text;
  }

  function toast(msg, kind = '') {
    const el = document.createElement('div');
    el.className = 'toast ' + kind;
    el.textContent = msg;
    els.toast.appendChild(el);
    setTimeout(() => el.remove(), 4000);
  }

  // --- Options: remembered > config file > built-in defaults -----------

  function pickBool(v, fallback) {
    return typeof v === 'boolean' ? v : fallback;
  }

  // configSignature identifies the current backend defaults. When the config
  // file changes, the signature changes and stale remembered options are
  // discarded so the freshly configured values take effect.
  function configSignature(d) {
    return JSON.stringify([
      d.workers, d.timeout_ms, d.max_hosts, d.probe_ports,
      d.use_tcp, d.lookup_mac, d.lookup_host,
    ]);
  }

  function applyOptions(cfg, saved) {
    state.saved = saved;
    const d = cfg || {};
    els.optTcp.checked = pickBool(saved && saved.use_tcp, pickBool(d.use_tcp, true));
    els.optMac.checked = pickBool(saved && saved.lookup_mac, pickBool(d.lookup_mac, true));
    els.optHost.checked = pickBool(saved && saved.lookup_host, pickBool(d.lookup_host, true));
    els.optSsh.checked = pickBool(saved && saved.ssh_mode, false);
    if (saved && saved.workers) els.optWorkers.value = saved.workers;
    else if (d.workers) els.optWorkers.value = d.workers;
    if (saved && saved.timeout) els.optTimeout.value = saved.timeout;
    else if (d.timeout_ms) els.optTimeout.value = d.timeout_ms;
    if (saved && saved.ports) els.optPorts.value = saved.ports;
    else if (d.probe_ports && d.probe_ports.length) els.optPorts.value = d.probe_ports.join(',');
    if (saved && saved.cidr) els.cidrInput.value = saved.cidr;
  }

  function resetOptions() {
    try { localStorage.removeItem(LS_OPTIONS_KEY); } catch (e) { /* ignore */ }
    state.saved = null;
    els.cidrInput.value = '';
    applyOptions(state.cfg, null);
    saveOptions();
    updateIfaceHint();
    updateAutoCidr();
    toast('已恢复 config.json 默认参数', 'success');
  }

  // --- Interfaces ----------------------------------------------------------

  async function loadInterfaces() {
    els.ifaceHint.textContent = '正在枚举本机网卡…';
    try {
      const res = await fetch('/api/interfaces');
      if (!res.ok) throw new Error('HTTP ' + res.status);
      const data = await res.json();
      state.interfaces = data.interfaces || [];
      renderInterfaces();
      setConn('ok', '已就绪');
    } catch (e) {
      els.ifaceHint.textContent = '枚举网卡失败: ' + e.message;
      setConn('bad', '后端不可达');
    }
  }

  function renderInterfaces() {
    const sel = els.ifaceSelect;
    sel.innerHTML = '';
    if (state.interfaces.length === 0) {
      const opt = document.createElement('option');
      opt.textContent = '未发现可用网卡';
      opt.disabled = true;
      sel.appendChild(opt);
      els.ifaceHint.textContent = '请确认本机已连接网络且至少有一个启用的 IPv4 接口。';
      return;
    }
    state.interfaces.forEach((iface) => {
      const opt = document.createElement('option');
      opt.value = iface.index;
      opt.textContent = `${iface.name} (${iface.ips.join(', ')})`;
      opt.dataset.subnet = iface.subnets[0] || '';
      sel.appendChild(opt);
    });
    sel.selectedIndex = 0;
    const savedIface = state.saved && state.saved.iface;
    if (savedIface) {
      for (const opt of sel.options) {
        if (opt.value === String(savedIface)) {
          sel.value = String(savedIface);
          break;
        }
      }
    }
    updateIfaceHint();
    updateAutoCidr();
  }

  function updateIfaceHint() {
    const iface = currentIface();
    if (!iface) return;
    const parts = [];
    if (iface.mac) parts.push('MAC: ' + iface.mac);
    if (iface.mtu) parts.push('MTU: ' + iface.mtu);
    els.ifaceHint.textContent = parts.join(' · ');
  }

  function currentIface() {
    const idx = parseInt(els.ifaceSelect.value, 10);
    return state.interfaces.find((i) => i.index === idx) || null;
  }

  function updateAutoCidr() {
    const iface = currentIface();
    if (!iface || !iface.subnets.length) return;
    const last = state.lastAutoCidr;
    if (!els.cidrInput.value || els.cidrInput.value === last) {
      const first = iface.subnets[0];
      els.cidrInput.value = first;
      state.lastAutoCidr = first;
    }
  }

  els.ifaceSelect.addEventListener('change', () => {
    updateIfaceHint();
    updateAutoCidr();
  });
  els.refreshIfaces.addEventListener('click', loadInterfaces);

  // Persist the parameters whenever they change so they are remembered.
  [
    els.ifaceSelect, els.cidrInput, els.optTcp, els.optMac, els.optHost, els.optSsh,
    els.optWorkers, els.optTimeout, els.optPorts,
  ].forEach((el) => {
    el.addEventListener('change', saveOptions);
    el.addEventListener('input', saveOptions);
  });

  // --- Quick range chips ---------------------------------------------------

  document.querySelectorAll('.chip').forEach((btn) => {
    btn.addEventListener('click', () => {
      const cidr = btn.dataset.cidr;
      if (cidr === 'auto') {
        updateAutoCidr();
        return;
      }
      // cidr like "/24": apply to current subnet's network part
      const iface = currentIface();
      if (!iface || !iface.subnets.length) {
        toast('请先选择网卡', 'error');
        return;
      }
      const base = iface.subnets[0];
      const [ip] = base.split('/');
      els.cidrInput.value = ip + cidr;
      state.lastAutoCidr = ip + cidr;
    });
  });

  // --- Scan ----------------------------------------------------------------

  function resetResults() {
    state.hosts.clear();
    state.order = [];
    els.hostsBody.innerHTML = '<tr class="empty"><td colspan="6">扫描中…</td></tr>';
    els.progressFill.style.width = '0%';
    els.aliveCount.textContent = '0';
    els.elapsed.textContent = '0.0s';
    els.currentCidr.textContent = '';
  }

  function parsePorts(str) {
    return String(str || '')
      .split(/[,，\s]+/)
      .map((s) => parseInt(s, 10))
      .filter((n) => Number.isFinite(n) && n > 0 && n < 65536);
  }

  async function startScan() {
    if (state.es) {
      toast('已有扫描在进行中', 'error');
      return;
    }
    const cidr = els.cidrInput.value.trim();
    if (!cidr) {
      toast('请输入 CIDR', 'error');
      return;
    }
    const iface = currentIface();
    const body = {
      interface: iface ? iface.name : '',
      cidr,
      use_tcp:    els.optTcp.checked,
      lookup_mac: els.optMac.checked,
      lookup_host:els.optHost.checked,
      workers:    parseInt(els.optWorkers.value, 10) || 128,
      timeout_ms: parseInt(els.optTimeout.value, 10) || 800,
      ports:      parsePorts(els.optPorts.value),
    };

    saveOptions();
    resetResults();
    els.startBtn.disabled = true;
    els.cancelBtn.disabled = false;
    els.exportBtn.disabled = true;
    setConn('warn', '扫描中…');
    state.startedAt = Date.now();
    startElapsedTimer();

    try {
      const res = await fetch('/api/scan', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const t = await res.text();
        throw new Error(t || ('HTTP ' + res.status));
      }
      const data = await res.json();
      state.scanId = data.id;
      els.currentCidr.textContent = cidr;
      els.progressText.textContent = '扫描中…';
      openStream(state.scanId);
    } catch (e) {
      toast('启动失败: ' + e.message, 'error');
      finishScan('error');
    }
  }

  async function cancelScan() {
    if (!state.scanId) return;
    const targetId = state.scanId;
    // Optimistic UI update so the user sees the stop take effect immediately.
    state.scanId = null;
    if (state.es) {
      state.es.close();
      state.es = null;
    }
    stopElapsedTimer();
    els.startBtn.disabled = false;
    els.cancelBtn.disabled = true;
    els.progressText.textContent = '正在停止...';
    try {
      await fetch('/api/scan/' + targetId + '/cancel', { method: 'POST' });
      toast('已停止', 'success');
    } catch (e) {
      toast('停止请求失败: ' + e.message, 'error');
    }
    // Always converge to a final state so the UI never gets stuck.
    finishScan('cancelled');
  }

  function startElapsedTimer() {
    if (state.elapsedTimer) clearInterval(state.elapsedTimer);
    state.elapsedTimer = setInterval(() => {
      const sec = (Date.now() - state.startedAt) / 1000;
      els.elapsed.textContent = sec.toFixed(1) + 's';
    }, 100);
  }

  function stopElapsedTimer() {
    if (state.elapsedTimer) {
      clearInterval(state.elapsedTimer);
      state.elapsedTimer = null;
    }
  }

  function finishScan(status) {
    state.scanId = null;
    if (state.es) {
      state.es.close();
      state.es = null;
    }
    stopElapsedTimer();
    els.startBtn.disabled = false;
    els.cancelBtn.disabled = true;
    els.exportBtn.disabled = state.hosts.size === 0;
    const labelMap = {
      done:      '已完成',
      cancelled: '已停止',
      error:     '异常',
    };
    setConn('ok', labelMap[status] || '已停止');
    if (status === 'done')       { els.progressText.textContent = '扫描完成'; toast('扫描完成', 'success'); }
    if (status === 'error')      toast('扫描出错', 'error');
    if (status === 'cancelled')  els.progressText.textContent = '已停止';
  }

  function openStream(id) {
    state.es = new EventSource('/api/scan/' + id + '/events');

    state.es.addEventListener('snapshot', (e) => {
      try {
        const data = JSON.parse(e.data);
        applySnapshot(data);
      } catch (err) { console.warn(err); }
    });

    state.es.addEventListener('event', (e) => {
      try {
        const ev = JSON.parse(e.data);
        applyEvent(ev);
      } catch (err) { console.warn(err); }
    });

    state.es.addEventListener('end', () => {
      state.es.close();
      state.es = null;
      finishScan('done');
    });

    state.es.onerror = () => {
      // If the scan is over (cancelled/done/error) don't auto-reconnect.
      if (!state.scanId) {
        if (state.es) {
          state.es.close();
          state.es = null;
        }
        return;
      }
      // Otherwise reconnect (default browser behaviour).
    };
  }

  function applySnapshot(snap) {
    if (snap.hosts && snap.hosts.length) {
      snap.hosts.forEach((h) => {
        if (!state.hosts.has(h.ip)) state.order.push(h.ip);
        state.hosts.set(h.ip, h);
      });
      renderHosts();
    }
    if (snap.summary) applySummary(snap.summary);
  }

  function applyEvent(ev) {
    if (ev.type === 'host' && ev.host) {
      if (!state.hosts.has(ev.host.ip)) state.order.push(ev.host.ip);
      state.hosts.set(ev.host.ip, ev.host);
      renderHosts();
    }
    if (ev.type === 'start') {
      els.progressText.textContent = '扫描中…';
    }
    if (ev.type === 'done') {
      // Device discovery is complete; enrichment (hostname/MAC/ports) may
      // still trickle in, so hint at that instead of claiming a full finish.
      els.progressText.textContent = (ev.alive > 0)
        ? '设备探测完成，正在解析详细信息…'
        : '扫描完成';
    }
    if (ev.type === 'cancelled') {
      els.progressText.textContent = '已停止';
    }
    if (ev.type === 'error') {
      els.progressText.textContent = '扫描出错: ' + (ev.error || '');
      toast(ev.error || '扫描出错', 'error');
    }
    applySummary({
      scanned:    ev.scanned,
      total:      ev.total,
      alive:      ev.alive,
      duration_ms:0,
    });
  }

  function applySummary(s) {
    const total = s.total || 0;
    const scanned = s.scanned || 0;
    els.progressCount.textContent = scanned + ' / ' + total;
    const pct = total ? Math.min(100, (scanned / total) * 100) : 0;
    els.progressFill.style.width = pct + '%';
    els.aliveCount.textContent = s.alive || 0;
  }

  // --- Rendering -----------------------------------------------------------

  function renderHosts() {
    if (state.hosts.size === 0) {
      els.hostsBody.innerHTML = '<tr class="empty"><td colspan="6">尚无在线主机</td></tr>';
      return;
    }
    const rows = [];
    state.order.forEach((ip) => {
      const h = state.hosts.get(ip);
      if (!h) return;
      rows.push(hostRow(h));
    });
    rows.sort(hostSortFn(state.sortKey, state.sortDir));
    els.hostsBody.innerHTML = rows.join('');
  }

  function hostRow(h) {
    const methodClass = h.method === 'tcp' ? 'method-tcp' : 'method-ping';
    const ports = (h.open_ports || [])
      .map((p) => '<span class="port-pill copy-port" data-ip="' + escapeHtml(h.ip) + '" data-port="' + p + '" title="复制 ' + escapeHtml(h.ip) + ':' + p + '">' + p + '</span>')
      .join(' ');
    const latency = h.latency_ms != null ? h.latency_ms + ' ms' : '—';
    return '<tr>' +
      '<td><span class="copy-ip" data-ip="' + escapeHtml(h.ip) + '" title="复制 ' + escapeHtml(h.ip) + '">' + escapeHtml(h.ip) + '</span></td>' +
      '<td>' + escapeHtml(h.hostname || '—') + '</td>' +
      '<td>' + escapeHtml(h.mac || '—') + '</td>' +
      '<td>' + latency + '</td>' +
      '<td><span class="method-pill ' + methodClass + '">' + (h.method || '—') + '</span></td>' +
      '<td>' + (ports || '<span class="muted">—</span>') + '</td>' +
      '</tr>';
  }

  function escapeHtml(s) {
    return String(s ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  // --- Copy to clipboard ----------------------------------------------------

  async function copyText(text, tip) {
    try {
      await navigator.clipboard.writeText(text);
      toast('已复制 ' + tip, 'success');
    } catch (e) {
      // Fallback for browsers without clipboard API on insecure contexts.
      const ta = document.createElement('textarea');
      ta.value = text;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      let ok = false;
      try {
        ok = document.execCommand('copy');
      } catch (err) { ok = false; }
      document.body.removeChild(ta);
      if (ok) {
        toast('已复制 ' + tip, 'success');
      } else {
        showCopyBox(text);
      }
    }
  }

  function showCopyBox(text) {
    const overlay = document.createElement('div');
    overlay.className = 'copy-overlay';
    overlay.innerHTML =
      '<div class="copy-dialog">' +
        '<div class="copy-dialog-head">无法写入剪贴板，请手动复制</div>' +
        '<textarea class="copy-dialog-text" readonly spellcheck="false"></textarea>' +
        '<div class="copy-dialog-actions">' +
          '<button type="button" class="primary select-all">全选</button>' +
          '<button type="button" class="ghost close">关闭</button>' +
        '</div>' +
      '</div>';
    const ta = overlay.querySelector('.copy-dialog-text');
    ta.value = text;
    overlay.querySelector('.select-all').addEventListener('click', () => {
      ta.focus();
      ta.select();
    });
    const close = () => overlay.remove();
    overlay.querySelector('.close').addEventListener('click', close);
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) close();
    });
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && overlay.isConnected) {
        close();
      }
    });
    document.body.appendChild(overlay);
    ta.focus();
    ta.select();
  }

  els.hostsBody.addEventListener('click', (e) => {
    const ipEl = e.target.closest('.copy-ip');
    if (ipEl) {
      copyText(ipEl.dataset.ip, ipEl.dataset.ip);
      return;
    }
    const portEl = e.target.closest('.copy-port');
    if (portEl) {
      const ip = portEl.dataset.ip;
      const port = portEl.dataset.port;
      let text;
      if (els.optSsh.checked) {
        text = port === '22'
          ? 'ssh root@' + ip
          : 'ssh root@' + ip + ' -p ' + port;
      } else {
        text = ip + ':' + port;
      }
      copyText(text, text);
    }
  });

  function hostSortFn(key, dir) {
    const mul = dir === 'desc' ? -1 : 1;
    return (a, b) => {
      const va = parseCell(extractCell(a, key));
      const vb = parseCell(extractCell(b, key));
      if (va < vb) return -1 * mul;
      if (va > vb) return  1 * mul;
      return 0;
    };
  }
  function extractCell(rowHtml, key) {
    const cols = { ip: 0, hostname: 1, mac: 2, latency: 3, method: 4 };
    const i = cols[key];
    if (i == null) return '';
    const m = rowHtml.match(/<td>([\s\S]*?)<\/td>/g);
    if (!m || !m[i]) return '';
    return m[i].replace(/^<td>/, '').replace(/<\/td>$/, '').replace(/<[^>]+>/g, '').trim();
  }
  function parseCell(v) {
    if (typeof v !== 'string') return v;
    if (v === '—' || v === '') return null;
    const n = parseFloat(v);
    if (!isNaN(n) && /^[\d.\sms]+$/.test(v)) return n;
    return v.toLowerCase();
  }

  document.querySelectorAll('thead th[data-sort]').forEach((th) => {
    th.addEventListener('click', () => {
      const key = th.dataset.sort;
      if (state.sortKey === key) {
        state.sortDir = state.sortDir === 'asc' ? 'desc' : 'asc';
      } else {
        state.sortKey = key;
        state.sortDir = 'asc';
      }
      document.querySelectorAll('thead th').forEach((t) => {
        t.classList.remove('sorted-asc', 'sorted-desc');
      });
      th.classList.add(state.sortDir === 'asc' ? 'sorted-asc' : 'sorted-desc');
      renderHosts();
    });
  });

  // --- Export --------------------------------------------------------------

  function exportCSV() {
    if (state.hosts.size === 0) return;
    const rows = [['IP', 'Hostname', 'MAC', 'Latency(ms)', 'Method', 'OpenPorts']];
    state.order.forEach((ip) => {
      const h = state.hosts.get(ip);
      if (!h) return;
      rows.push([
        h.ip,
        h.hostname || '',
        h.mac || '',
        h.latency_ms ?? '',
        h.method || '',
        (h.open_ports || []).join(';'),
      ]);
    });
    const csv = rows
      .map((r) => r.map((c) => /[,"\n]/.test(String(c)) ? '"' + String(c).replace(/"/g, '""') + '"' : c).join(','))
      .join('\n');
    const blob = new Blob(['\ufeff' + csv], { type: 'text/csv;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'lan-scan-' + new Date().toISOString().replace(/[:.]/g, '-') + '.csv';
    a.click();
    URL.revokeObjectURL(url);
  }

  els.exportBtn.addEventListener('click', exportCSV);
  els.startBtn.addEventListener('click', startScan);
  els.cancelBtn.addEventListener('click', cancelScan);
  els.resetOptsBtn.addEventListener('click', resetOptions);

  // --- Boot ---------------------------------------------------------------

  async function boot() {
    // 1. Server defaults come from the config file / CLI flags.
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        state.cfg = await res.json();
      }
    } catch (e) { /* keep built-in defaults */ }

    state.sig = state.cfg ? configSignature(state.cfg) : null;

    // 2. 选项优先级：服务端记忆（跨端口） > 本浏览器 localStorage（带签名校验）
    //    > 服务端默认值。
    const saved = loadSavedOptions();
    const remembered =
      saved && (!state.sig || saved.sig === state.sig) ? saved : null;
    let effective = remembered;
    const serverPrefs = await loadServerPrefs();
    if (serverPrefs) effective = serverPrefs;

    applyOptions(state.cfg, effective);
    if (!effective) saveOptions(); // persist fresh defaults + signature

    // 3. Enumerate interfaces last; it may auto-fill the CIDR when the user
    //    has no remembered one.
    await loadInterfaces();
  }

  boot();
})();
