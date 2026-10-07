// 与旧版 assets/js/app.js 的辅助函数逐一对应, 语义保持一致(URL 拼接、时间格式、阈值分档)。

export class HttpError extends Error {
  constructor(status, body) {
    super(`HTTP ${status}`);
    this.status = status;
    this.body = body;
  }
}

async function request(url, init = {}, timeout = 0) {
  const ctrl = new AbortController();
  const timer = timeout > 0 ? setTimeout(() => ctrl.abort(), timeout) : null;
  try {
    const r = await fetch(url, { credentials: 'same-origin', ...init, signal: ctrl.signal });
    const text = await r.text();
    if (!r.ok) throw new HttpError(r.status, text);
    return text;
  } finally {
    if (timer) clearTimeout(timer);
  }
}

export async function getJSON(url, { timeout = 0 } = {}) {
  const text = await request(url, {}, timeout);
  return JSON.parse(text);
}

export async function getText(url, { timeout = 0 } = {}) {
  return request(url, {}, timeout);
}

// 与 jQuery 1.7 $.post 逐字节一致: encodeURIComponent 后 %20 → '+'
// (URLSearchParams 还会转义 ! ' ( ) ~, 解码结果相同但字节不同, 这里保持与旧版一致)
export function jqParam(data) {
  return Object.keys(data)
    .map((k) => encodeURIComponent(k) + '=' + encodeURIComponent(data[k] == null ? '' : data[k]))
    .join('&')
    .replace(/%20/g, '+');
}

export async function postForm(url, data = {}, { timeout = 0 } = {}) {
  const text = await request(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8', 'X-Requested-With': 'XMLHttpRequest' },
    body: jqParam(data),
  }, timeout);
  return JSON.parse(text);
}

export function errorText(e, fallback = '请求失败') {
  if (!e) return fallback;
  if (e.name === 'AbortError') return '请求超时';
  if (e.body) return String(e.body).trim().substring(0, 200) || fallback;
  return e.message || fallback;
}

/* ---------- 格式化 ---------- */
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
  if (n < 10 && Math.abs(n - Math.round(n)) > 0.05) return n.toFixed(1) + '%';
  if (Math.abs(n - Math.round(n)) < 0.05) return Math.round(n) + '%';
  return n.toFixed(2) + '%';
}
export function fmtChartVal(v) {
  if (v == null || v === '-') return '-';
  const n = typeof v === 'number' ? v : parseFloat(v);
  return Number.isNaN(n) ? '-' : n.toFixed(2);
}

// 延迟: 相对基线涨幅 ≥10%黄 / ≥20%橙 / ≥30%红; 丢包: ≥5%黄 / ≥20%橙 / ≥50%红 (取较高档)
export function delayLevel(delay, loss, baseline) {
  let lvl = 0;
  delay = Number(delay) || 0;
  loss = Number(loss) || 0;
  baseline = Number(baseline) || 0;
  if (baseline > 0 && delay > 0) {
    const pct = ((delay - baseline) / baseline) * 100;
    if (pct >= 30) lvl = 4;
    else if (pct >= 20) lvl = 3;
    else if (pct >= 10) lvl = 2;
  }
  if (loss >= 50) lvl = Math.max(lvl, 4);
  else if (loss >= 20) lvl = Math.max(lvl, 3);
  else if (loss >= 5) lvl = Math.max(lvl, 2);
  return lvl;
}
export function delayRisePct(delay, baseline) {
  delay = Number(delay) || 0;
  baseline = Number(baseline) || 0;
  if (baseline <= 0 || delay <= 0) return null;
  return ((delay - baseline) / baseline) * 100;
}

const round2 = (n) => Math.round(n * 100) / 100;
// API 用 "-" 表示空洞, 统一转 null
export function sanitizeSeries(arr) {
  if (!arr || !arr.length) return [];
  return arr.map((v) => {
    if (v === '-' || v === '' || v == null) return null;
    const n = typeof v === 'number' ? v : parseFloat(v);
    return Number.isNaN(n) ? null : round2(n);
  });
}
export function lastMetric(avgdelay, losspk) {
  const out = { delay: '-', loss: '-', lossRaw: null };
  if (!avgdelay || !avgdelay.length) return out;
  for (let k = avgdelay.length - 1; k >= 0; k--) {
    const d = avgdelay[k];
    const l = losspk ? losspk[k] : null;
    if (d === '-' || d === '' || d == null) continue;
    const dn = parseFloat(d);
    if (Number.isNaN(dn)) continue;
    out.delay = fmtMs(dn);
    if (l !== '-' && l !== '' && l != null) {
      const ln = parseFloat(l);
      if (!Number.isNaN(ln)) {
        out.lossRaw = ln;
        out.loss = fmtLossPct(ln);
      }
    }
    break;
  }
  return out;
}
export function chartAxisLabels(lastcheck) {
  if (!lastcheck || !lastcheck.length) return [];
  // 按时间跨度决定是否带日期(超过 24 小时), 不按点数: 10 秒采样下 2 小时就有 720 个点
  const t0 = new Date(String(lastcheck[0]).replace(' ', 'T')).getTime();
  const t1 = new Date(String(lastcheck[lastcheck.length - 1]).replace(' ', 'T')).getTime();
  const showDate = Number.isFinite(t0) && Number.isFinite(t1) ? t1 - t0 > 24 * 3600 * 1000 : lastcheck.length > 180;
  return lastcheck.map((x) => {
    const s = String(x);
    if (showDate && s.length >= 16) return s.substring(5, 16);
    if (s.length >= 16) return s.substring(11, 16);
    return s;
  });
}

/* ---------- 时间 ---------- */
const pad = (n) => (n < 10 ? '0' + n : '' + n);
// 服务端按北京时间存储和查询; 统一按 UTC+8 生成时间字符串, 与设备时区无关
// (设备时区不是北京时间时, 用本地时间会请求到不存在的时段, 曲线右侧整段空白)。中国无夏令时, 固定偏移即可。
const BJ_OFFSET_MS = 8 * 3600 * 1000;
export function fmtBeijing(d, { sep = ' ', seconds = false } = {}) {
  const t = new Date(d.getTime() + BJ_OFFSET_MS);
  const s = `${t.getUTCFullYear()}-${pad(t.getUTCMonth() + 1)}-${pad(t.getUTCDate())}${sep}${pad(t.getUTCHours())}:${pad(t.getUTCMinutes())}`;
  return seconds ? `${s}:${pad(t.getUTCSeconds())}` : s;
}
export function dlocal(d) {
  return fmtBeijing(d, { sep: 'T' });
}
export function fmtLocalMinute(d) {
  return fmtBeijing(d);
}
export function rangeFromPreset(mins) {
  const now = new Date();
  const ago = new Date(now.getTime() - mins * 60 * 1000);
  return { start: fmtLocalMinute(ago), end: fmtLocalMinute(now) };
}
export function timeWindowLabel(mins, custom) {
  if (custom) return `${custom.start} ~ ${custom.end}`;
  if (mins < 60) return `近${mins}分钟`;
  if (mins < 1440) return `近${Math.round(mins / 60)}小时`;
  if (mins === 1440) return '近1天';
  return `近${Math.round(mins / 1440)}天`;
}
export const toInputTime = (s) => (s || '').replace(' ', 'T');
export const fromInputTime = (s) => (s || '').replace('T', ' ');

/* ---------- URL ---------- */
export function proxy(addr, port, path) {
  return '/api/proxy.json?g=http://' + addr + ':' + port + path;
}
// 经 proxy 时, 被代理 URL 里的 & 必须写成 %26, 否则会被当成 proxy.json 自身的参数
export function nestedQs(base, qs) {
  return base === '' ? qs : qs.replace(/&/g, '%26');
}
export function nodeBase(cfg, addr) {
  return addr === cfg.Addr ? '' : '/api/proxy.json?g=http://' + addr + ':' + cfg.Port;
}
export function pingUrlWithRange(apiurl, start, end) {
  if (!start || !end) return apiurl;
  const sep = apiurl.indexOf('proxy.json') >= 0 ? '%26' : '&';
  return apiurl + sep + 'starttime=' + encodeURIComponent(start) + sep + 'endtime=' + encodeURIComponent(end);
}
export function meshQueryString(mins, custom) {
  if (custom && custom.start && custom.end) {
    return '?start=' + encodeURIComponent(custom.start) + '&end=' + encodeURIComponent(custom.end);
  }
  return '?mins=' + (mins || 15);
}
export function proxyMeshUrl(addr, port, mins, custom) {
  const qs = meshQueryString(mins, custom);
  return '/api/proxy.json?g=http://' + addr + ':' + port + '/api/pingmesh.json' + qs.replace(/&/g, '%26');
}
export function rangeQueryString(range) {
  return '?start=' + encodeURIComponent(range.start) + '&end=' + encodeURIComponent(range.end);
}
export function proxyTopologyUrl(addr, port, range) {
  const qs = rangeQueryString(range);
  return '/api/proxy.json?g=http://' + addr + ':' + port + '/api/topology.json' + qs.replace(/&/g, '%26');
}

/* ---------- 节点 ---------- */
// 互Ping的源节点(探测节点且有监测目标), 当前节点排第一
export function sourceNodes(cfg) {
  const list = Object.values((cfg && cfg.Network) || {}).filter(
    (n) => n && n.Pingmesh && ((n.Ping && n.Ping.length > 0) || (n.Topology && n.Topology.length > 0)),
  );
  list.sort((a, b) => (a.Addr === cfg.Addr ? -1 : b.Addr === cfg.Addr ? 1 : a.Name < b.Name ? -1 : 1));
  return list;
}
export function nodeName(cfg, addr) {
  const net = (cfg && cfg.Network) || {};
  return net[addr] && net[addr].Name ? net[addr].Name : addr;
}

/* ---------- ASN (RIPE NCC, 服务端 24h 缓存; 前端去重) ---------- */
export function isPrivateHost(addr) {
  if (!addr) return true;
  if (/^(10\.|127\.|192\.168\.|169\.254\.)/.test(addr)) return true;
  const m = addr.match(/^172\.(\d+)\./);
  return !!(m && +m[1] >= 16 && +m[1] <= 31);
}
const asnCache = new Map();
const asnPending = new Map();
const cleanHost = (addr) => (addr || '').replace(/^https?:\/\//, '').split('/')[0].split(':')[0];
export function asnLookup(addr) {
  addr = cleanHost(addr);
  if (!addr || isPrivateHost(addr)) return Promise.resolve(null);
  if (asnCache.has(addr)) return Promise.resolve(asnCache.get(addr));
  if (!asnPending.has(addr)) {
    asnPending.set(addr, getJSON('/api/asn.json?ip=' + encodeURIComponent(addr))
      .then((res) => (res.status === 'true' ? res : null))
      .catch(() => null)
      .then((info) => {
        asnCache.set(addr, info);
        asnPending.delete(addr);
        return info;
      }));
  }
  return asnPending.get(addr);
}
// 同步读缓存: undefined=未查询, null=查不到
export function asnCached(addr) {
  addr = (addr || '').split(':')[0];
  return asnCache.has(addr) ? asnCache.get(addr) : undefined;
}
export function asnShort(info) {
  if (!info || !info.asn) return '';
  let holder = String(info.holder || '').split(' - ')[0].split(',')[0];
  if (holder.length > 18) holder = holder.substring(0, 17) + '…';
  return 'AS' + info.asn + (holder ? ' · ' + holder : '');
}

/* ---------- IP 可达探测(并发 6, 短缓存) ---------- */
let reachCache = new Map();
const reachPending = new Map();
const reachQueue = [];
let reachActive = 0;
const REACH_MAX = 6;
const reachClean = (host) => (host || '').replace(/^https?:\/\//, '').split('/')[0];
function reachPump() {
  while (reachActive < REACH_MAX && reachQueue.length) {
    const host = reachQueue.shift();
    reachActive++;
    getJSON('/api/reach.json?ip=' + encodeURIComponent(host))
      .then((res) => (res && res.status === 'true' ? res : null))
      .catch(() => null)
      .then((info) => {
        reachCache.set(host, info);
        reachActive--;
        const cbs = reachPending.get(host) || [];
        reachPending.delete(host);
        cbs.forEach((cb) => cb(info));
        reachPump();
      });
  }
}
export function reachLookup(host) {
  host = reachClean(host);
  if (!host) return Promise.resolve(null);
  if (reachCache.has(host)) return Promise.resolve(reachCache.get(host));
  return new Promise((resolve) => {
    if (reachPending.has(host)) { reachPending.get(host).push(resolve); return; }
    reachPending.set(host, [resolve]);
    reachQueue.push(host);
    reachPump();
  });
}
export function reachClear() {
  reachCache = new Map();
}

/* ---------- localStorage ---------- */
export function loadPref(key, fallback) {
  try {
    const v = JSON.parse(localStorage.getItem(key) || 'null');
    return v == null ? fallback : v;
  } catch (e) {
    return fallback;
  }
}
export function savePref(key, value) {
  try { localStorage.setItem(key, JSON.stringify(value)); } catch (e) { /* 隐私模式等 */ }
}
