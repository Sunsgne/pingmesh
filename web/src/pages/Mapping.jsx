import { useEffect, useMemo, useRef, useState } from 'react';
import { Box, Button, Chip, Stack, Table, TableBody, TableCell, TableHead, TableRow, TextField, Typography } from '@mui/material';
import EChart from '../components/EChart';
import { Badge, EmptyState, Panel, Spinner } from '../components/ui';
import { useToast } from '../components/Feedback';
import { delayLevel, fromInputTime, getJSON, sourceNodes } from '../api';
import { mono, palette } from '../theme';

const UNREACH = 1999; // 探测端约定: 不可达时延迟为 2000ms
const MOBILE = '@media (max-width:900px)';
const LEGEND = [
  ['#10b981', '正常'],
  ['#fcd34d', '延迟升高≥10% / 丢包率≥5%'],
  ['#fb923c', '延迟升高≥20% / 丢包率≥20%'],
  ['#f43f5e', '延迟升高≥30% / 丢包率≥50%'],
  ['#94a3b8', '不可达'],
  ['#eef2f7', '未配置探测'],
];

// 聚合所有线路: 每个地区取可达线路的平均值, 全部不可达则标记不可达
function mergedAll(data) {
  const byRegion = {};
  Object.values(data.avgdelay || {}).forEach((list) => {
    (list || []).forEach((item) => {
      (byRegion[item.name] = byRegion[item.name] || []).push(item.value);
    });
  });
  return Object.keys(byRegion).map((region) => {
    const ok = byRegion[region].filter((v) => v < UNREACH);
    let v;
    if (ok.length > 0) {
      const sum = ok.reduce((a, x) => a + x, 0);
      v = Math.round((sum / ok.length) * 100) / 100;
    } else {
      v = 2000;
    }
    return { name: region, value: v };
  });
}

function seriesFor(data, line) {
  if (!data) return [];
  return line === '全部' ? mergedAll(data) : (data.avgdelay[line] || []);
}

function mapBaseline(seriesData) {
  const vals = [];
  (seriesData || []).forEach((it) => {
    if (it && it.value != null && !Number.isNaN(Number(it.value)) && it.value < UNREACH && it.value > 0) vals.push(Number(it.value));
  });
  if (vals.length === 0) return 0;
  vals.sort((a, b) => a - b);
  return vals[Math.floor(vals.length / 2)];
}

function mapOption(data, activeLine) {
  const seriesData = seriesFor(data, activeLine);
  const base = mapBaseline(seriesData);
  const pieces = base > 0 ? [
    { min: 0, max: base * 1.1, color: '#10b981', label: '正常' },
    { min: base * 1.1, max: base * 1.2, color: '#fcd34d', label: '≥10%' },
    { min: base * 1.2, max: base * 1.3, color: '#fb923c', label: '≥20%' },
    { min: base * 1.3, max: UNREACH, color: '#f43f5e', label: '≥30%' },
    { min: UNREACH, color: '#94a3b8', label: '不可达' },
  ] : [
    { min: 0, max: UNREACH, color: '#10b981', label: '正常' },
    { min: UNREACH, color: '#94a3b8', label: '不可达' },
  ];
  return {
    title: {
      text: (data ? data.text : '') + ' · ' + (activeLine === '全部' ? '全部线路' : (activeLine || '')) + ' 全球延迟',
      subtext: data ? data.subtext : '',
      left: 'center',
      textStyle: { color: '#0f172a', fontSize: 15, fontWeight: 'bold' },
      subtextStyle: { color: '#94a3b8', fontSize: 12 },
    },
    tooltip: {
      trigger: 'item',
      backgroundColor: 'rgba(15,23,42,.92)', borderWidth: 0, padding: [9, 13],
      textStyle: { color: '#e2e8f0', fontSize: 12 },
      formatter: (p) => {
        // 区域名/线路名来自配置或远端节点数据, 写进 HTML 提示框前要转义
        const name = escHtml(p.name);
        const line = escHtml(activeLine);
        if (p.value == null || Number.isNaN(p.value)) return name + '<br>未配置探测点';
        if (p.value >= UNREACH) return name + ' · ' + line + '<br><b style="color:#f87171">不可达</b>';
        let extra = '';
        if (base > 0) {
          const pct = ((Number(p.value) - base) / base) * 100;
          extra = '<br>相对基线 ' + (pct >= 0 ? '+' : '') + pct.toFixed(0) + '%';
        }
        return name + ' · ' + line + '<br>平均延迟 <b>' + Number(p.value).toFixed(1) + ' ms</b>' + extra;
      },
    },
    visualMap: {
      type: 'piecewise', left: 10, bottom: 10, showLabel: true,
      textStyle: { color: '#64748b', fontSize: 11 },
      itemWidth: 14, itemHeight: 10, show: false,
      pieces,
    },
    series: [{
      name: activeLine, type: 'map', map: 'world',
      roam: true, scaleLimit: { min: 0.8, max: 8 },
      top: 60, bottom: 20,
      itemStyle: { areaColor: '#eef2f7', borderColor: '#ffffff', borderWidth: 0.8 },
      emphasis: {
        label: { show: true, color: '#0f172a', fontWeight: 'bold', fontSize: 11 },
        itemStyle: { areaColor: '#c7d2fe' },
      },
      select: { disabled: true },
      data: seriesData,
    }],
  };
}

function LevelBadge({ v, base }) {
  if (v >= UNREACH) return <Badge tone="gray">不可达</Badge>;
  const lvl = delayLevel(v, 0, base || 0);
  if (lvl <= 1) return <Badge tone="green" dot>正常</Badge>;
  if (lvl === 2) return <Badge tone="yellow">升高≥10%</Badge>;
  if (lvl === 3) return <Badge tone="yellow">升高≥20%</Badge>;
  return <Badge tone="red" dot>升高≥30%</Badge>;
}

const btnSx = { py: '5px', lineHeight: 1.25, minWidth: 0 };
const chipSx = (active) => ({
  height: 'auto', py: '6px', px: '14px', fontSize: 12.5, borderRadius: 999, border: `1px solid ${active ? palette.primary : palette.border}`,
  transition: 'all .15s', '& .MuiChip-label': { p: 0, lineHeight: 1.25 },
  [MOBILE]: { py: '5px', px: '11px', fontSize: 12 },
  ...(active
    ? { bgcolor: palette.primary, color: '#fff', '&:hover': { bgcolor: palette.primary } }
    : { bgcolor: '#fff', color: palette.text2, '&:hover': { bgcolor: '#fff', borderColor: palette.primary2, color: palette.primary } }),
});

const escHtml = (v) => String(v == null ? '' : v).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

export default function Mapping({ config: cfg }) {
  const disabled = !!(cfg.Base && cfg.Base.Chinamap === 0);
  if (disabled) {
    return (
      <Panel>
        <EmptyState icon="🌍">
          全球延迟功能已关闭<br />
          <Typography component="span" sx={{ color: palette.text3, fontSize: 12.5 }}>管理员可在「系统配置 - 基础设置」中开启</Typography>
          <Box sx={{ mt: 2 }}><Button variant="outlined" href="config.html" sx={{ py: '8px', lineHeight: 1.2 }}>前往系统配置</Button></Box>
        </EmptyState>
      </Panel>
    );
  }
  return <MappingView cfg={cfg} />;
}

function MappingView({ cfg }) {
  const toast = useToast();
  const nodes = useMemo(() => sourceNodes(cfg), [cfg]);
  const [agent, setAgent] = useState(cfg.Addr);
  const [pick, setPick] = useState('');
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState(null);
  const [activeLine, setActiveLine] = useState(null);
  const [listShown, setListShown] = useState(false);
  const [listBase, setListBase] = useState(0);
  const [q, setQ] = useState('');
  const [sortDesc, setSortDesc] = useState(true);
  const [activeRow, setActiveRow] = useState(null);
  const [tip, setTip] = useState(null);

  const chartRef = useRef(null);
  const mapCardRef = useRef(null);
  const baseRef = useRef('');
  // 与旧版 activeLine 变量一致: 「回到当前」先改它, 请求成功后才重绘地图
  const lineRef = useRef(null);
  const reqSeq = useRef(0);

  // 正在看历史时刻时不自动刷新(否则会被拉回当前)
  const historyRef = useRef(false);
  const load = (d) => {
    historyRef.current = d !== '';
    const seq = ++reqSeq.current;
    setLoading(true);
    const url = baseRef.current + '/api/mapping.json' + (d !== '' ? '?d=' + encodeURIComponent(d) : '');
    getJSON(url).then((result) => {
      if (seq !== reqSeq.current) return;
      setLoading(false);
      const lines = Object.keys(result.avgdelay || {}).sort();
      let line = lineRef.current;
      setActiveRow(null);
      setListShown(true);
      if (lines.length === 0) {
        setActiveLine(line);
        setData(null);
        setListBase(0);
        toast('该时间点暂无探测数据(每分钟采集一次)', 'err');
        return;
      }
      // 地图默认聚合「全部」线路; 查看单条线路通过列表的「地图定位」进入
      if (!line || (line !== '全部' && lines.indexOf(line) < 0)) line = '全部';
      lineRef.current = line;
      setActiveLine(line);
      setData(result);
      setListBase(mapBaseline(seriesFor(result, line)));
    }).catch(() => {
      if (seq !== reqSeq.current) return;
      setLoading(false);
      toast('获取延迟数据失败', 'err');
    });
  };

  useEffect(() => {
    load('');
    // 定时只刷新数据, 不整页重载: 保留所选节点、线路和搜索条件, 也不用每分钟重新下载 1MB 的地图脚本
    const t = setInterval(() => {
      if (!document.hidden && !historyRef.current) load('');
    }, ((cfg.Base && cfg.Base.Refresh) || 1) * 60 * 1000);
    return () => clearInterval(t);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const option = useMemo(() => mapOption(data, activeLine), [data, activeLine]);
  const mapBase = useMemo(() => mapBaseline(seriesFor(data, activeLine)), [data, activeLine]);

  useEffect(() => {
    const c = chartRef.current && chartRef.current.instance;
    if (!tip || !c) return;
    c.dispatchAction({ type: 'hideTip' });
    c.dispatchAction({ type: 'showTip', seriesIndex: 0, name: tip.region });
  }, [tip]);

  const selectAgent = (n) => {
    setAgent(n.Addr);
    baseRef.current = n.Addr === cfg.Addr ? '' : '/api/proxy.json?g=http://' + n.Addr + ':' + cfg.Port;
    load('');
  };
  const pickTime = () => {
    if (!pick) { toast('请选择时间', 'err'); return; }
    load(fromInputTime(pick));
  };
  const backNow = () => {
    setPick('');
    lineRef.current = '全部';
    load('');
  };
  const relist = () => { setActiveRow(null); setListBase(mapBase); };
  const onSearch = (e) => { setQ(e.target.value); relist(); };
  const toggleSort = () => { setSortDesc((s) => !s); relist(); };

  const rows = useMemo(() => {
    if (!listShown) return [];
    const query = q.trim().toLowerCase();
    let out = [];
    if (data) {
      Object.keys(data.avgdelay || {}).forEach((line) => {
        (data.avgdelay[line] || []).forEach((item) => {
          const ipCnt = (((cfg.Chinamap || {})[item.name] || {})[line] || []).length;
          out.push({ region: item.name, line, value: item.value, ips: ipCnt });
        });
      });
    }
    out = out.filter((r) => query === '' || (r.region + ' ' + r.line).toLowerCase().indexOf(query) >= 0);
    out.sort((a, b) => (sortDesc ? b.value - a.value : a.value - b.value));
    return out;
  }, [data, q, sortDesc, listShown, cfg]);

  const clickRow = (r) => {
    setActiveRow(r.region + '\u0000' + r.line);
    if (r.line !== lineRef.current) {
      lineRef.current = r.line;
      setActiveLine(r.line);
    }
    const el = mapCardRef.current;
    if (el) window.scrollTo({ top: el.getBoundingClientRect().top + window.scrollY - 10, behavior: 'smooth' });
    setTip({ region: r.region, seq: Date.now() });
  };

  return (
    <>
      <Box ref={mapCardRef}>
        <Panel
          sx={{ mb: 2 }}
          title="全球 Ping 延迟地图"
          sub="各地区线路延迟着色，滚轮缩放、拖拽平移，可回看历史"
          actions={(
            <>
              <TextField size="small" type="datetime-local" value={pick} onChange={(e) => setPick(e.target.value)}
                sx={{ width: 195, '& .MuiOutlinedInput-input': { p: '6px 8px', fontSize: 13.5 } }} />
              <Button size="small" variant="contained" sx={btnSx} onClick={pickTime}>查看该时间</Button>
              <Button size="small" variant="outlined" sx={btnSx} onClick={backNow}>回到当前</Button>
              {loading && <Spinner />}
            </>
          )}
        >
          <Stack direction="row" alignItems="center" flexWrap="wrap" sx={{ gap: '10px', mb: 1 }}>
            <Typography component="span" sx={{ color: palette.text3, fontSize: 12.5 }}>探测节点</Typography>
            <Stack direction="row" flexWrap="wrap" sx={{ gap: '8px', [MOBILE]: { gap: '6px' } }}>
              {nodes.map((n) => (
                <Chip key={n.Addr} label={n.Name} clickable onClick={() => selectAgent(n)} sx={chipSx(n.Addr === agent)} />
              ))}
            </Stack>
          </Stack>
          <EChart ref={chartRef} option={option} height={560} />
          <Box sx={{
            display: 'flex', alignItems: 'center', gap: '14px', flexWrap: 'wrap', fontSize: 12, color: palette.text2,
            pt: '4px', borderTop: '1px solid #f1f5f9', mt: '14px', [MOBILE]: { gap: '10px' },
          }}>
            {LEGEND.map(([c, t]) => (
              <Box key={t} component="span" sx={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
                <Box component="span" sx={{ width: 12, height: 12, borderRadius: '3px', display: 'inline-block', flex: 'none', background: c, boxShadow: 'inset 0 0 0 1px rgba(15,23,42,.06)' }} />
                {t}
              </Box>
            ))}
          </Box>
        </Panel>
      </Box>

      <Panel
        flat
        title={<>地区延迟列表 <Badge tone="indigo">{listShown ? rows.length + ' 条' : ''}</Badge></>}
        sub={data ? '数据时间 ' + data.subtext : ''}
        actions={(
          <TextField size="small" placeholder="搜索地区 / 线路..." value={q} onChange={onSearch}
            sx={{ width: 240, maxWidth: '100%', '& .MuiOutlinedInput-input': { p: '9px 12px', fontSize: 13.5 } }} />
        )}
      >
        <Box sx={{ overflowX: 'auto' }}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>地区</TableCell>
                <TableCell>线路</TableCell>
                <TableCell>探测点</TableCell>
                <TableCell sx={{ cursor: 'pointer' }} onClick={toggleSort}>平均延迟 <span>{sortDesc ? '↓' : '↑'}</span></TableCell>
                <TableCell>状态</TableCell>
                <TableCell sx={{ width: 110 }}>操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {rows.map((r) => {
                const key = r.region + '\u0000' + r.line;
                const active = key === activeRow;
                return (
                  <TableRow key={key} hover onClick={() => clickRow(r)} sx={{
                    cursor: 'pointer', '&:last-child td': { borderBottom: 'none' },
                    ...(active ? { '& td': { background: `${palette.primarySoft} !important` } } : {}),
                  }}>
                    <TableCell><b>{r.region}</b></TableCell>
                    <TableCell><Badge tone="indigo">{r.line}</Badge></TableCell>
                    <TableCell sx={{ color: palette.text3 }}>{(r.ips || '-') + ' 个 IP'}</TableCell>
                    <TableCell sx={{ fontFamily: mono, fontSize: 12.5 }}><b>{r.value >= UNREACH ? '—' : Number(r.value).toFixed(1) + ' ms'}</b></TableCell>
                    <TableCell><LevelBadge v={r.value} base={listBase} /></TableCell>
                    <TableCell><a className="locate">地图定位 →</a></TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </Box>
        {listShown && rows.length === 0 && (
          <EmptyState icon="🌍">没有匹配的地区，请调整搜索条件或在「系统配置 - 全球延迟」中添加探测点</EmptyState>
        )}
      </Panel>
    </>
  );
}
