// 与 html/assets/js/app.js 中的同名辅助函数保持一致, 共用后端接口
export async function getJSON(url) {
  const r = await fetch(url, { credentials: 'same-origin' });
  if (r.status === 401 || r.redirected) {
    window.location.href = '/login.html';
    throw new Error('unauthorized');
  }
  if (!r.ok) throw new Error(`HTTP ${r.status}`);
  return r.json();
}

const pad = (n) => (n < 10 ? '0' + n : '' + n);
export function fmtLocalMinute(d) {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
export function rangeFromPreset(mins) {
  const now = new Date();
  return { start: fmtLocalMinute(new Date(now.getTime() - mins * 60000)), end: fmtLocalMinute(now) };
}
export function timeWindowLabel(mins, custom) {
  if (custom) return `${custom.start} ~ ${custom.end}`;
  if (mins < 60) return `近${mins}分钟`;
  if (mins < 1440) return `近${Math.round(mins / 60)}小时`;
  if (mins === 1440) return '近1天';
  return `近${Math.round(mins / 1440)}天`;
}

export function nodeBase(rootCfg, addr) {
  return addr === rootCfg.Addr ? '' : `/api/proxy.json?g=http://${addr}:${rootCfg.Port}`;
}
// 经 proxy 时嵌套参数的 & 需转义成 %26
export function withQuery(base, path, params) {
  const qs = Object.entries(params)
    .filter(([, v]) => v !== undefined && v !== null && v !== '')
    .map(([k, v]) => `${k}=${encodeURIComponent(v)}`)
    .join(base ? '%26' : '&');
  return base + path + (qs ? '?' + qs : '');
}

export function sourceNodes(cfg) {
  const list = Object.values((cfg && cfg.Network) || {}).filter(
    (n) => n && n.Pingmesh && ((n.Ping && n.Ping.length) || (n.Topology && n.Topology.length)),
  );
  list.sort((a, b) => (a.Addr === cfg.Addr ? -1 : b.Addr === cfg.Addr ? 1 : a.Name < b.Name ? -1 : 1));
  return list;
}
export function nodeName(cfg, addr) {
  const n = ((cfg && cfg.Network) || {})[addr];
  return n && n.Name ? n.Name : addr;
}

export function delayLevel(delay, loss, baseline) {
  let lvl = 0;
  delay = Number(delay) || 0; loss = Number(loss) || 0; baseline = Number(baseline) || 0;
  if (baseline > 0 && delay > 0) {
    const pct = ((delay - baseline) / baseline) * 100;
    if (pct >= 30) lvl = 4; else if (pct >= 20) lvl = 3; else if (pct >= 10) lvl = 2;
  }
  if (loss >= 50) lvl = Math.max(lvl, 4); else if (loss >= 20) lvl = Math.max(lvl, 3); else if (loss >= 5) lvl = Math.max(lvl, 2);
  return lvl;
}

export function sanitizeSeries(arr) {
  return (arr || []).map((v) => {
    if (v === '-' || v === '' || v == null) return null;
    const n = typeof v === 'number' ? v : parseFloat(v);
    return Number.isNaN(n) ? null : Math.round(n * 100) / 100;
  });
}
export function fmtMs(v) {
  const n = parseFloat(v);
  return Number.isNaN(n) ? '-' : n.toFixed(2);
}
export function fmtLossPct(v) {
  if (v == null || v === '-' || v === '') return '-';
  const n = typeof v === 'number' ? v : parseFloat(v);
  if (Number.isNaN(n)) return '-';
  if (n >= 100) return '100%';
  if (n === 0) return '0%';
  if (Math.abs(n - Math.round(n)) < 0.05) return Math.round(n) + '%';
  return (n < 10 ? n.toFixed(1) : n.toFixed(2)) + '%';
}
export function lastMetric(avgdelay, losspk) {
  for (let k = (avgdelay || []).length - 1; k >= 0; k--) {
    const d = parseFloat(avgdelay[k]);
    if (Number.isNaN(d)) continue;
    const l = parseFloat(losspk ? losspk[k] : NaN);
    return { delay: fmtMs(d), loss: Number.isNaN(l) ? null : l };
  }
  return { delay: '-', loss: null };
}
export function chartAxisLabels(lastcheck) {
  const showDate = (lastcheck || []).length > 180;
  return (lastcheck || []).map((s) => {
    s = String(s);
    if (s.length >= 16) return showDate ? s.substring(5, 16) : s.substring(11, 16);
    return s;
  });
}

const asnCache = new Map();
export function isPrivateHost(addr) {
  if (!addr) return true;
  if (/^(10\.|127\.|192\.168\.|169\.254\.)/.test(addr)) return true;
  const m = addr.match(/^172\.(\d+)\./);
  return !!(m && +m[1] >= 16 && +m[1] <= 31);
}
export function asnLookup(addr) {
  if (isPrivateHost(addr)) return Promise.resolve(null);
  if (!asnCache.has(addr)) {
    asnCache.set(addr, getJSON('/api/asn.json?ip=' + encodeURIComponent(addr))
      .then((r) => (r.status === 'true' ? r : null))
      .catch(() => null));
  }
  return asnCache.get(addr);
}
export function asnShort(info) {
  if (!info || !info.asn) return '';
  let holder = String(info.holder || '').split(' - ')[0].split(',')[0];
  if (holder.length > 18) holder = holder.substring(0, 17) + '…';
  return `AS${info.asn}${holder ? ' · ' + holder : ''}`;
}
