import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Box, Button, Checkbox, GlobalStyles, Switch, TextField, ThemeProvider, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { EmptyState, Panel, Spinner, ToolbarBox, ToolbarLabel } from '../components/ui';
import PingChartDialog from '../components/PingChartDialog';
import { useToast } from '../components/Feedback';
import { delayLevel, delayRisePct, dlocal, fmtMs, fromInputTime, getJSON, meshQueryString, nodeName, sourceNodes } from '../api';
import { mono, palette, shadowLg } from '../theme';

const MOBILE = '@media (max-width:900px)';
const WINDOWS = [
  { v: '15', label: '15分钟' }, { v: '60', label: '1小时' }, { v: '360', label: '6小时' },
  { v: '1440', label: '1天' }, { v: '10080', label: '7天' }, { v: 'custom', label: '自定义' },
];
const ZOOMS = [
  { v: 'std', label: '标准', title: '较大格子，显示节点 IP' },
  { v: 'compact', label: '紧凑', title: '较小格子，隐藏 IP' },
  { v: 'heat', label: '色块', title: '最密色块，仍显示延迟/丢包率' },
];
const LEGEND = [
  ['#10b981', '正常'],
  ['#fbbf24', '延迟升高≥10% / 丢包率≥5%'],
  ['#fb923c', '延迟升高≥20% / 丢包率≥20%'],
  ['#f43f5e', '延迟升高≥30% / 丢包率≥50%'],
  ['#eef2f7', '无数据'],
];
const SELF_LEGEND = { background: 'repeating-linear-gradient(-45deg,#f8fafc,#f8fafc 2px,#e2e8f0 2px,#e2e8f0 4px)', border: `1px solid ${palette.border}` };

function readZoom() {
  let z = 'std';
  try { z = localStorage.getItem('mesh-zoom') || 'std'; } catch (e) { /* 隐私模式 */ }
  return z === 'heat' || z === 'compact' || z === 'std' ? z : 'std';
}
function readMeshOnly() {
  try { return localStorage.getItem('mesh-only') === '1'; } catch (e) { return false; }
}

function buildTopo(cfg, meshOnly) {
  const sources = sourceNodes(cfg);
  const meshAddrs = {};
  sources.forEach((s) => { meshAddrs[s.Addr] = true; });
  const targetSet = {};
  sources.forEach((s) => {
    targetSet[s.Addr] = true;
    (s.Ping || []).forEach((t) => {
      if (meshOnly && !meshAddrs[t]) return;
      targetSet[t] = true;
    });
  });
  let targets = [];
  const seen = {};
  sources.forEach((s) => {
    if (targetSet[s.Addr] && !seen[s.Addr]) {
      seen[s.Addr] = true;
      targets.push(s.Addr);
    }
  });
  if (!meshOnly) {
    const extras = Object.keys(targetSet).filter((t) => !seen[t]);
    extras.sort((a, b) => {
      const na = nodeName(cfg, a); const nb = nodeName(cfg, b);
      if (na === nb) return a < b ? -1 : 1;
      return na < nb ? -1 : 1;
    });
    targets = targets.concat(extras);
  }
  return { sources, targets };
}

function computeFit(wrap, n, zoom) {
  // 手机屏幕收窄行头, 把宽度留给矩阵格子
  const phone = wrap.clientWidth < 600;
  const rowW = phone ? (zoom === 'std' ? 88 : 72) : (zoom === 'std' ? 132 : (zoom === 'compact' ? 108 : 104));
  const gap = zoom === 'heat' ? 2 : (zoom === 'compact' ? 3 : 4);
  const avail = Math.max(200, wrap.clientWidth - rowW - 8);
  const raw = Math.floor((avail - gap * (n + 1)) / n);
  let cell; let font;
  if (zoom === 'heat') {
    // 色块也要能放下延迟+丢包两行
    cell = Math.max(34, Math.min(52, raw > 0 ? raw : 40));
    font = cell >= 42 ? 10.5 : 9.5;
  } else if (zoom === 'compact') {
    cell = Math.max(52, Math.min(78, raw > 0 ? raw : 60));
    font = cell >= 60 ? 11.5 : 10.5;
  } else {
    cell = Math.max(72, Math.min(118, raw > 0 ? raw : 92));
    font = cell >= 90 ? 12.5 : 11.5;
  }
  const radius = zoom === 'heat' ? '4px' : (zoom === 'compact' ? '5px' : '7px');
  return { cell, rowW, gap, font, radius, n, zoom };
}

function lossText(c) {
  let l = (Math.round(c.loss * 10) / 10) + '%';
  if (l.indexOf('.') < 0 && String(c.loss).indexOf('.') >= 0) l = c.loss.toFixed(1) + '%';
  return l;
}

function CellBody({ c, cellPx }) {
  if (c.loss >= 99.5) return <span className="v-d">✕</span>;
  const d = fmtMs(c.avgdelay);
  const l = lossText(c);
  // 小格子用单行, 否则两行: 延迟 / 丢包
  if (cellPx < 40) return <><span className="v-d">{d}</span><span className="v-l">/{l}</span></>;
  return <><span className="v-d">{d}</span><span className="v-l">{l}</span></>;
}

function cellTip(srcName, dstName, c) {
  const rise = delayRisePct(c.avgdelay, c.baseline);
  const riseTxt = rise == null ? '' : ('\n相对基线 ' + (rise >= 0 ? '+' : '') + rise.toFixed(0) + '% (基线 ' + fmtMs(c.baseline) + 'ms)');
  return srcName + ' → ' + dstName
    + '\n延迟 ' + fmtMs(c.avgdelay) + 'ms · 丢包率 ' + c.loss.toFixed(1) + '%'
    + '\n最大 ' + fmtMs(c.maxdelay) + 'ms · 抖动 ' + fmtMs(c.jitter || 0) + 'ms · 采样 ' + c.points + ' 次'
    + riseTxt + '\n' + c.lastcheck;
}

const fsStyles = (
  <GlobalStyles styles={{
    'main:fullscreen': { background: palette.bg, overflowY: 'auto', padding: '28px 36px' },
    'main:fullscreen .mesh-stats .ms b': { fontSize: 22 },
    'main:fullscreen .mesh-stats .ms': { fontSize: 13, minWidth: 120 },
  }} />
);

const statsSx = {
  display: 'flex', gap: '10px', flexWrap: 'wrap', alignItems: 'stretch',
  '& .ms': {
    display: 'flex', flexDirection: 'column', justifyContent: 'center', minWidth: 108, p: '10px 14px',
    bgcolor: palette.bg, border: `1px solid ${palette.border}`, borderRadius: '10px', fontSize: 12, color: palette.text3,
  },
  '& .ms b': { fontSize: 18, color: palette.text, m: '0 0 2px', fontVariantNumeric: 'tabular-nums', letterSpacing: '-.02em', lineHeight: 1.15 },
  '& .ms.ok b': { color: palette.green },
  '& .ms.bad b': { color: palette.red },
  '& .ms.accent b': { color: palette.primary },
  [MOBILE]: { gap: '14px', '& .ms': { minWidth: 96, p: '8px 12px' }, '& .ms b': { fontSize: 16 } },
};

const noHover = { transform: 'none', boxShadow: 'none', filter: 'none' };
const meshSx = {
  '--mesh-cell': '96px', '--mesh-row': '132px', '--mesh-gap': '4px', '--mesh-font': '12.5px', '--mesh-radius': '7px',
  overflowX: 'auto', WebkitOverflowScrolling: 'touch', p: '2px 2px 8px', m: 0,
  '&::-webkit-scrollbar': { height: 8 },
  '&::-webkit-scrollbar-thumb': { background: '#cbd5e1', borderRadius: 999 },
  '& .mesh-table': { borderCollapse: 'separate', borderSpacing: 'var(--mesh-gap)', tableLayout: 'fixed', width: 'max-content', maxWidth: '100%' },
  '& .mesh-table th': { fontSize: 11.5, color: palette.text2, fontWeight: 600, p: '4px', whiteSpace: 'nowrap' },
  '& .mesh-table th.col-h': {
    textAlign: 'center', width: 'var(--mesh-cell)', minWidth: 'var(--mesh-cell)', maxWidth: 'var(--mesh-cell)',
    overflow: 'hidden', textOverflow: 'ellipsis', verticalAlign: 'bottom',
  },
  '& .mesh-table th.col-h .col-name': { display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 11.5, fontWeight: 700, color: palette.text },
  '& .mesh-table th.col-h .col-ip': { display: 'block', fontWeight: 400, fontSize: 10, color: palette.text3, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  '& .mesh-table th.row-h': { textAlign: 'right', pr: '12px', width: 'var(--mesh-row)', minWidth: 'var(--mesh-row)', maxWidth: 'var(--mesh-row)' },
  // 横向滚动时源节点列固定在左侧
  '& .mesh-table th.row-h, & .mesh-table th.corner': { position: 'sticky', left: 0, zIndex: 3, background: '#fff' },
  '& .mesh-table th.row-h .src-name': { fontWeight: 700, fontSize: 12, display: 'block', color: palette.text, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  '& .mesh-table th.row-h .src-ip': { display: 'block', fontWeight: 400, fontSize: 10, color: palette.text3, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  '& .mesh-table th.corner': { textAlign: 'right', pr: '12px', color: palette.text3, fontSize: 11, fontWeight: 600, letterSpacing: '.02em' },
  '& .mesh-table td': { width: 'var(--mesh-cell)', minWidth: 'var(--mesh-cell)', maxWidth: 'var(--mesh-cell)', p: 0 },
  '& .mono': { fontFamily: mono },
  '& .mesh-cell': {
    width: '100%', height: 'var(--mesh-cell)', borderRadius: 'var(--mesh-radius)', textAlign: 'center',
    fontSize: 'var(--mesh-font)', fontWeight: 700, cursor: 'pointer', color: '#fff',
    transition: 'transform .1s ease, box-shadow .1s ease, filter .1s ease', position: 'relative',
    fontVariantNumeric: 'tabular-nums', boxSizing: 'border-box', overflow: 'hidden',
    display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
    gap: '1px', lineHeight: 1.1, p: '2px 1px',
  },
  '& .mesh-cell .v-d': { fontWeight: 800, letterSpacing: '-.02em' },
  '& .mesh-cell .v-l': { fontWeight: 600, opacity: 0.92, fontSize: '.78em' },
  '& .mesh-cell:hover': { transform: 'scale(1.08)', boxShadow: shadowLg, zIndex: 5, filter: 'brightness(1.05)' },
  '& .mesh-cell.na': { background: '#eef2f7', color: palette.text3, cursor: 'default', fontWeight: 500 },
  '& .mesh-cell.na:hover': noHover,
  '& .mesh-cell.self': {
    background: 'repeating-linear-gradient(-45deg, #f8fafc, #f8fafc 3px, #e2e8f0 3px, #e2e8f0 6px)',
    border: `1px solid ${palette.border}`, cursor: 'default', color: 'transparent', fontSize: 0,
  },
  '& .mesh-cell.self:hover': noHover,
  '& .mesh-l0, & .mesh-l1': { background: '#10b981' },
  '& .mesh-l2': { background: '#fbbf24', color: '#78350f' },
  '& .mesh-l3': { background: '#fb923c', color: '#7c2d12' },
  '& .mesh-l4': { background: '#f43f5e' },
  '&.mesh-zoom-compact': { '--mesh-gap': '3px', '--mesh-font': '11px', '--mesh-radius': '5px', '--mesh-row': '108px' },
  '&.mesh-zoom-compact .mesh-table th.col-h .col-ip, &.mesh-zoom-compact .mesh-table th.row-h .src-ip': { display: 'none' },
  '&.mesh-zoom-compact .mesh-table th.col-h .col-name': { fontSize: 10.5 },
  '&.mesh-zoom-compact .mesh-table th.row-h .src-name': { fontSize: 11 },
  '&.mesh-zoom-heat': { '--mesh-gap': '2px', '--mesh-font': '9.5px', '--mesh-radius': '4px', '--mesh-row': '104px' },
  '&.mesh-zoom-heat .mesh-table th.col-h': { height: 86, verticalAlign: 'bottom', pb: '6px' },
  '&.mesh-zoom-heat .mesh-table th.col-h .col-name': {
    writingMode: 'vertical-rl', transform: 'rotate(180deg)', maxHeight: 78, m: '0 auto', fontSize: 10, fontWeight: 600,
    letterSpacing: '.02em', whiteSpace: 'nowrap',
  },
  '&.mesh-zoom-heat .mesh-table th.col-h .col-ip, &.mesh-zoom-heat .mesh-table th.row-h .src-ip': { display: 'none' },
  '&.mesh-zoom-heat .mesh-table th.row-h .src-name': { fontSize: 11 },
  '&.mesh-zoom-heat .mesh-cell': { cursor: 'pointer', gap: 0 },
  '&.mesh-zoom-heat .mesh-cell .v-l': { fontSize: '.72em' },
  '&.mesh-zoom-heat .mesh-cell:hover': { transform: 'scale(1.28)' },
  '&.mesh-zoom-heat .mesh-cell.na:hover, &.mesh-zoom-heat .mesh-cell.self:hover': noHover,
  '&.mesh-zoom-heat .mesh-cell.self': { fontSize: 0 },
  // 手机: 行头只留节点名(放在最后, 覆盖前面各档位的规则)
  '@media (max-width:600px)': {
    '& .mesh-table th.row-h': { pr: '6px' },
    '& .mesh-table th.row-h .src-name': { fontSize: 11 },
    '& .mesh-table th.row-h .src-ip': { display: 'none' },
  },
};

const switchSx = {
  width: 38, height: 22, p: 0, overflow: 'visible',
  '& .MuiSwitch-switchBase': {
    p: '2px', color: '#fff',
    '&.Mui-checked': { transform: 'translateX(16px)', color: '#fff', '& + .MuiSwitch-track': { bgcolor: palette.primary, opacity: 1 } },
    '&:hover': { bgcolor: 'transparent' },
  },
  '& .MuiSwitch-thumb': { width: 18, height: 18, boxShadow: '0 1px 3px rgba(0,0,0,.25)' },
  '& .MuiSwitch-track': { borderRadius: 999, bgcolor: '#cbd5e1', opacity: 1 },
};
const btnSx = { py: '5px', lineHeight: 1.25, minWidth: 0 };
const segSx = { '& .MuiToggleButton-root': { py: '6px', lineHeight: 1.25 }, [MOBILE]: { maxWidth: '100%', overflowX: 'auto', flexWrap: 'nowrap' } };
const inputSx = { width: 185, '& .MuiOutlinedInput-input': { p: '6px 8px', fontSize: 13.5 } };

export default function Pingmesh({ config }) {
  const toast = useToast();
  const [cfg, setCfg] = useState(config);
  const [seg, setSeg] = useState('15');
  const [mins, setMins] = useState(15);
  const [custom, setCustom] = useState(null);
  const [cr, setCr] = useState({ start: '', end: '' });
  const [zoom, setZoom] = useState(readZoom);
  const [meshOnly, setMeshOnly] = useState(readMeshOnly);
  const [meshData, setMeshData] = useState({});
  const [rendered, setRendered] = useState(false);
  const [loading, setLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [isFs, setIsFs] = useState(false);
  const [chart, setChart] = useState(null);
  const [fit, setFit] = useState(null);
  const [, setResizeTick] = useState(0);

  const rootRef = useRef(null);
  const wrapRef = useRef(null);
  const cfgRef = useRef(config);
  const queryRef = useRef({ mins: 15, custom: null });
  const chartRef = useRef(null);
  chartRef.current = chart;

  const { sources, targets } = useMemo(() => buildTopo(cfg, meshOnly), [cfg, meshOnly]);

  const loadSeq = useRef(0);
  const load = useCallback((q = queryRef.current) => {
    const my = ++loadSeq.current;
    // 换了时间窗口就清掉旧矩阵, 避免新旧窗口的行混在一起
    if (q !== queryRef.current) setMeshData({});
    queryRef.current = q;
    setLoading(true);
    getJSON('/api/config.json').then((ncfg) => ncfg, () => cfgRef.current).then((c) => {
      cfgRef.current = c;
      setCfg(c);
      const srcs = sourceNodes(c);
      if (srcs.length === 0) { setLoading(false); return; }
      let pending = srcs.length;
      const qs = meshQueryString(q.custom ? null : q.mins, q.custom);
      // 1 天及以上的窗口聚合较久, 给到 30 秒, 否则节点会被误判为不可达
      const baseTimeout = ((c.Base && c.Base.Timeout) || 5) * 1000;
      const timeout = q.custom || q.mins >= 1440 ? Math.max(baseTimeout, 30000) : baseTimeout;
      srcs.forEach((s) => {
        const url = s.Addr === c.Addr
          ? '/api/pingmesh.json' + qs
          : '/api/proxy.json?g=http://' + s.Addr + ':' + c.Port + '/api/pingmesh.json' + qs.replace(/&/g, '%26');
        getJSON(url, { timeout })
          .then((row) => row, () => ({ error: true }))
          .then((row) => {
            if (my !== loadSeq.current) return;
            pending--;
            setMeshData((m) => ({ ...m, [s.Addr]: row }));
            setRendered(true);
            if (pending === 0) setLoading(false);
          });
      });
    });
  }, []);
  const loadRef = useRef(load);
  loadRef.current = load;

  useEffect(() => { load(); }, [load]);

  useEffect(() => {
    try { localStorage.setItem('mesh-zoom', zoom); } catch (e) { /* 隐私模式 */ }
  }, [zoom]);

  useEffect(() => {
    if (!autoRefresh) return undefined;
    const timer = setInterval(() => {
      // 历史图弹窗打开时暂停刷新, 避免背后矩阵重绘拖慢鼠标
      if (chartRef.current || document.hidden) return;
      loadRef.current();
    }, 60000);
    return () => clearInterval(timer);
  }, [autoRefresh]);

  useEffect(() => {
    const onResize = () => setResizeTick((t) => t + 1);
    let fsTimer = null;
    const onFs = () => {
      setIsFs(!!(document.fullscreenElement || document.webkitFullscreenElement));
      clearTimeout(fsTimer);
      fsTimer = setTimeout(() => { setRendered(true); onResize(); }, 50);
    };
    window.addEventListener('resize', onResize);
    document.addEventListener('fullscreenchange', onFs);
    document.addEventListener('webkitfullscreenchange', onFs);
    return () => {
      clearTimeout(fsTimer);
      window.removeEventListener('resize', onResize);
      document.removeEventListener('fullscreenchange', onFs);
      document.removeEventListener('webkitfullscreenchange', onFs);
    };
  }, []);

  useLayoutEffect(() => {
    const wrap = wrapRef.current;
    if (!wrap || !targets.length) return;
    const f = computeFit(wrap, targets.length, zoom);
    setFit((old) => (old && old.cell === f.cell && old.font === f.font && old.zoom === f.zoom && old.n === f.n ? old : f));
  });

  const onSeg = (_, v) => {
    if (v == null) return;
    setSeg(v);
    if (v === 'custom') {
      if (!cr.start) setCr({ start: dlocal(new Date(Date.now() - 6 * 3600 * 1000)), end: dlocal(new Date()) });
      return;
    }
    const m = parseInt(v, 10);
    setCustom(null);
    setMins(m);
    load({ mins: m, custom: null });
  };
  const applyCustom = () => {
    const s = cr.start; const e = cr.end;
    if (!s || !e || s >= e) { toast('请选择正确的起止时间', 'err'); return; }
    const c = { start: fromInputTime(s), end: fromInputTime(e) };
    setCustom(c);
    load({ mins, custom: c });
  };
  const onZoom = (_, v) => {
    if (v == null) return;
    setZoom(v);
    setRendered(true);
  };
  const onMeshOnly = (e) => {
    const on = e.target.checked;
    setMeshOnly(on);
    try { localStorage.setItem('mesh-only', on ? '1' : '0'); } catch (err) { /* 隐私模式 */ }
    setRendered(true);
  };
  const toggleFs = () => {
    const el = (rootRef.current && rootRef.current.closest('main')) || document.documentElement;
    if (document.fullscreenElement || document.webkitFullscreenElement) {
      (document.exitFullscreen || document.webkitExitFullscreen).call(document);
    } else {
      (el.requestFullscreen || el.webkitRequestFullscreen).call(el);
    }
  };
  const openCell = (src, dst) => {
    const apiurl = src === cfg.Addr
      ? '/api/ping.json?ip=' + dst
      : '/api/proxy.json?g=http://' + src + ':' + cfg.Port + '/api/ping.json?ip=' + dst;
    setChart({ title: nodeName(cfg, src) + ' → ' + nodeName(cfg, dst) + ' (' + dst + ')', apiurl });
  };

  const cellPx = fit ? fit.cell : 96;
  let stTotal = 0; let stOk = 0; let stBad = 0; let stSum = 0; let stCnt = 0;
  const rows = sources.map((s) => {
    const row = meshData[s.Addr];
    const monitoredList = ((cfg.Network || {})[s.Addr] || {}).Ping || [];
    const cells = targets.map((t) => {
      if (t === s.Addr) return <td key={t}><div className="mesh-cell self" title="自身" /></td>;
      const c = row && row.cells ? row.cells[t] : null;
      if (monitoredList.indexOf(t) < 0 || !row || row.error || !c || c.points === 0) {
        const err = !!(row && row.error);
        return <td key={t}><div className="mesh-cell na" title={err ? '节点不可达' : '无数据'}>{err ? '!' : ''}</div></td>;
      }
      const lvl = delayLevel(c.avgdelay, c.loss, c.baseline);
      stTotal++; stSum += c.avgdelay; stCnt++;
      if (lvl >= 3) stBad++; else stOk++;
      return (
        <td key={t}>
          <div className={'mesh-cell mesh-l' + lvl} title={cellTip(s.Name, nodeName(cfg, t), c)} onClick={() => openCell(s.Addr, t)}>
            <CellBody c={c} cellPx={cellPx} />
          </div>
        </td>
      );
    });
    return (
      <tr key={s.Addr}>
        <th className="row-h"><span className="src-name">{s.Name}</span><span className="src-ip mono">{s.Addr}</span></th>
        {cells}
      </tr>
    );
  });
  const stats = rendered && sources.length > 0
    ? { nodes: sources.length, links: stTotal, ok: stOk, bad: stBad, avg: stCnt > 0 ? fmtMs(stSum / stCnt) + ' ms' : '-' }
    : { nodes: '-', links: '-', ok: '-', bad: '-', avg: '-' };

  const wrapStyle = fit ? {
    '--mesh-cell': fit.cell + 'px', '--mesh-row': fit.rowW + 'px', '--mesh-gap': fit.gap + 'px',
    '--mesh-font': fit.font + 'px', '--mesh-radius': fit.radius,
  } : undefined;
  const fitHint = fit && targets.length ? targets.length + ' 列 · 格宽 ' + fit.cell + 'px' + (meshOnly ? ' · 仅 Mesh' : '') : '';
  const zoomCls = zoom === 'compact' ? 'mesh-zoom-compact' : (zoom === 'heat' ? 'mesh-zoom-heat' : '');

  return (
    <Box ref={rootRef}>
      {fsStyles}
      <Panel sx={{ mb: 2 }} bodySx={{ p: '14px 16px' }}>
        <Box className="mesh-stats" sx={statsSx}>
          <div className="ms"><b>{stats.nodes}</b>探测节点</div>
          <div className="ms"><b>{stats.links}</b>监测链路</div>
          <div className="ms ok"><b>{stats.ok}</b>正常</div>
          <div className="ms bad"><b>{stats.bad}</b>异常</div>
          <div className="ms accent"><b>{stats.avg}</b>全网平均延迟</div>
          <Box sx={{ flex: 1 }} />
          <Box component="span" sx={{ alignSelf: 'center' }}>{loading && <Spinner />}</Box>
        </Box>
      </Panel>

      <Panel
        title="Pingmesh"
        sub="行 = 探测源，列 = 目标 · 色块内为延迟 / 丢包率 · 点击查看历史"
        actions={(
          <>
            <Box component="label" sx={{ display: 'flex', alignItems: 'center', gap: '7px', fontSize: 12.5, color: palette.text2, cursor: 'pointer' }}>
              <Switch checked={autoRefresh} onChange={(e) => setAutoRefresh(e.target.checked)} disableRipple sx={switchSx} />
              自动刷新
            </Box>
            <Button size="small" variant="outlined" sx={btnSx} onClick={() => load()}>刷新</Button>
            <Button size="small" variant="outlined" sx={btnSx} title="投屏到监控大屏, ESC 退出" onClick={toggleFs}>
              {isFs ? '✖ 退出全屏' : '⛶ 全屏'}
            </Button>
          </>
        )}
      >
        <ToolbarBox sx={{ gap: '10px 14px', [MOBILE]: { p: '10px 12px', gap: '10px' }, '& .tb-label': { [MOBILE]: { display: 'none' } } }}>
          <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', minWidth: 0 }}>
            <span className="tb-label"><ToolbarLabel>时间</ToolbarLabel></span>
            <ToggleButtonGroup exclusive size="small" value={seg} onChange={onSeg} sx={segSx}>
              {WINDOWS.map((w) => <ToggleButton key={w.v} value={w.v}>{w.label}</ToggleButton>)}
            </ToggleButtonGroup>
            {seg === 'custom' && (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <TextField size="small" type="datetime-local" value={cr.start} sx={inputSx}
                  onChange={(e) => setCr((x) => ({ ...x, start: e.target.value }))} />
                <Typography component="span" sx={{ color: palette.text3 }}>~</Typography>
                <TextField size="small" type="datetime-local" value={cr.end} sx={inputSx}
                  onChange={(e) => setCr((x) => ({ ...x, end: e.target.value }))} />
                <Button size="small" variant="contained" sx={btnSx} onClick={applyCustom}>应用</Button>
              </Box>
            )}
          </Box>
          <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', minWidth: 0 }}>
            <span className="tb-label"><ToolbarLabel>密度</ToolbarLabel></span>
            <ToggleButtonGroup exclusive size="small" value={zoom} onChange={onZoom} sx={segSx}>
              {ZOOMS.map((z) => <ToggleButton key={z.v} value={z.v} title={z.title}>{z.label}</ToggleButton>)}
            </ToggleButtonGroup>
          </Box>
          <Box component="label" title="隐藏 DNS 等纯被测目标，只看探测节点之间的 Mesh" sx={{
            display: 'inline-flex', alignItems: 'center', gap: '7px', fontSize: 12.5, fontWeight: 600, color: palette.text2,
            cursor: 'pointer', userSelect: 'none', p: '5px 10px', borderRadius: 2, border: `1px solid ${palette.border}`, bgcolor: '#fff',
            '&:hover': { borderColor: '#c7d2fe', color: palette.primary },
          }}>
            <Checkbox checked={meshOnly} onChange={onMeshOnly} disableRipple size="small"
              sx={{ p: 0, color: '#767676', '&.Mui-checked': { color: palette.primary }, '& svg': { fontSize: 18 } }} />
            <span>仅节点 Mesh</span>
          </Box>
          <Box sx={{ flex: '1 1 12px' }} />
          <Typography component="span" sx={{ color: palette.text3, fontSize: 11.5 }}>{fitHint}</Typography>
        </ToolbarBox>

        <Box ref={wrapRef} className={zoomCls} sx={meshSx} style={wrapStyle}>
          {sources.length === 0 ? (
            <table className="mesh-table"><tbody><tr><td>
              <EmptyState>没有启用探测的源节点，请到「系统配置 - 节点管理」开启</EmptyState>
            </td></tr></tbody></table>
          ) : (
            <table className="mesh-table">
              <tbody>
                <tr>
                  <th className="row-h corner">源 \ 目标</th>
                  {targets.map((t) => (
                    <th key={t} className="col-h" title={nodeName(cfg, t) + ' · ' + t}>
                      <span className="col-name">{nodeName(cfg, t)}</span>
                      <span className="col-ip mono">{t}</span>
                    </th>
                  ))}
                </tr>
                {rows}
              </tbody>
            </table>
          )}
        </Box>

        <Box sx={{
          display: 'flex', alignItems: 'center', gap: '14px', flexWrap: 'wrap', fontSize: 12, color: palette.text2,
          pt: '4px', borderTop: '1px solid #f1f5f9', mt: '14px', [MOBILE]: { gap: '10px' },
        }}>
          {LEGEND.map(([c, t]) => <Legend key={t} sq={{ background: c }}>{t}</Legend>)}
          <Legend sq={SELF_LEGEND}>自身</Legend>
        </Box>
      </Panel>
      {isFs ? (
        <ThemeProvider theme={(outer) => withDialogContainer(outer, document.fullscreenElement || document.webkitFullscreenElement)}>
          <PingChartDialog chart={chart} onClose={() => setChart(null)} />
        </ThemeProvider>
      ) : <PingChartDialog chart={chart} onClose={() => setChart(null)} />}
    </Box>
  );
}

// 全屏时弹窗默认挂在 body 上会被遮住, 改挂到全屏元素内
function withDialogContainer(outer, container) {
  if (!container) return outer;
  const comps = outer.components || {};
  const dlg = comps.MuiDialog || {};
  return { ...outer, components: { ...comps, MuiDialog: { ...dlg, defaultProps: { ...dlg.defaultProps, container } } } };
}

function Legend({ sq, children }) {
  return (
    <Box component="span" sx={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
      <Box component="span" sx={{ width: 12, height: 12, borderRadius: '3px', display: 'inline-block', flex: 'none', boxShadow: 'inset 0 0 0 1px rgba(15,23,42,.06)', ...sq }} />
      {children}
    </Box>
  );
}

