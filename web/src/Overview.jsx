import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert, Box, Button, Card, CardActionArea, CardContent, CardHeader, Chip, CircularProgress, Dialog,
  DialogContent, DialogTitle, IconButton, Snackbar, Stack, TextField, ToggleButton, ToggleButtonGroup, Typography,
} from '@mui/material';
import HubOutlined from '@mui/icons-material/HubOutlined';
import PublicOutlined from '@mui/icons-material/PublicOutlined';
import ScheduleOutlined from '@mui/icons-material/ScheduleOutlined';
import WarningAmberOutlined from '@mui/icons-material/WarningAmberOutlined';
import CloseIcon from '@mui/icons-material/Close';
import EChart from './EChart';
import { bigOption, miniOption } from './charts';
import { palette, shadowLg } from './theme';
import {
  asnLookup, asnShort, delayLevel, fmtLossPct, fmtMs, getJSON, lastMetric, nodeBase, nodeName,
  rangeFromPreset, sourceNodes, timeWindowLabel, withQuery,
} from './api';

const PRESETS = [
  { v: 15, label: '15分钟' }, { v: 60, label: '1小时' }, { v: 120, label: '2小时' },
  { v: 360, label: '6小时' }, { v: 1440, label: '24小时' },
];
const STORE_KEY = 'index-time';

function loadTimePref() {
  try {
    const s = JSON.parse(localStorage.getItem(STORE_KEY) || 'null');
    if (s && s.custom && s.custom.start && s.custom.end) return { mins: 120, custom: s.custom };
    if (s && s.mins) return { mins: s.mins, custom: null };
  } catch (e) { /* ignore */ }
  return { mins: 120, custom: null };
}

function lossChipColor(v) {
  if (v == null) return { bgcolor: palette.greenSoft, color: '#047857' };
  if (v >= 20) return { bgcolor: palette.redSoft, color: '#b91c1c' };
  if (v >= 5) return { bgcolor: palette.yellowSoft, color: '#b45309' };
  return { bgcolor: palette.greenSoft, color: '#047857' };
}

function StatCard({ icon, tone, value, label }) {
  const tones = {
    indigo: [palette.primarySoft, palette.primary], green: [palette.greenSoft, palette.green],
    yellow: [palette.yellowSoft, palette.yellow], red: [palette.redSoft, palette.red],
  };
  const [bg, fg] = tones[tone];
  return (
    <Card sx={{ transition: 'box-shadow .18s, transform .18s', '&:hover': { boxShadow: shadowLg, transform: 'translateY(-2px)' } }}>
      <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 1.75, p: '18px !important' }}>
        <Box sx={{ width: 44, height: 44, borderRadius: 3, bgcolor: bg, color: fg, display: 'grid', placeItems: 'center' }}>{icon}</Box>
        <Box sx={{ minWidth: 0 }}>
          <Typography sx={{ fontSize: 22, fontWeight: 700, lineHeight: 1.1 }}>{value}</Typography>
          <Typography noWrap sx={{ fontSize: 12.5, color: palette.text3, mt: 0.25 }}>{label}</Typography>
        </Box>
      </CardContent>
    </Card>
  );
}

function PingCard({ title, target, url, range, onOpen }) {
  const [data, setData] = useState(null);
  const [failed, setFailed] = useState(false);
  const [asn, setAsn] = useState('');

  useEffect(() => {
    let alive = true;
    setFailed(false);
    getJSON(withQuery(url.base, url.path, { ip: target, starttime: range.start, endtime: range.end }))
      .then((d) => alive && setData(d))
      .catch(() => alive && setFailed(true));
    return () => { alive = false; };
  }, [url.base, url.path, target, range.start, range.end]);

  useEffect(() => {
    asnLookup(target).then((info) => setAsn(asnShort(info)));
  }, [target]);

  const option = useMemo(() => (data ? miniOption(data) : null), [data]);
  const m = data ? lastMetric(data.avgdelay, data.losspk) : { delay: '-', loss: null };
  const mono = { fontFamily: 'ui-monospace, Menlo, Consolas, monospace' };

  return (
    <Card sx={{ transition: 'box-shadow .18s, transform .18s', '&:hover': { boxShadow: shadowLg, transform: 'translateY(-2px)' } }}>
      <CardActionArea onClick={onOpen} sx={{ '&:hover .MuiCardActionArea-focusHighlight': { opacity: 0 } }}>
        <Typography noWrap sx={{ px: 2, pt: 1.75, pb: 0.5, fontSize: 13.5, fontWeight: 600 }}>{title}</Typography>
        <Box sx={{ px: 0.5 }}><EChart option={option} height={130} /></Box>
        <Stack direction="row" flexWrap="wrap" gap={0.75} sx={{ px: 2, pb: 1.75, pt: 0.5 }}>
          <Chip size="small" label={target} sx={{ ...mono, bgcolor: '#f1f5f9', color: palette.text2 }} />
          <Chip size="small" label={failed ? '失败' : `${m.delay} ms`} sx={{ bgcolor: palette.primarySoft, color: palette.primary }} />
          <Chip size="small" label={`丢包率 ${m.loss == null ? '-' : fmtLossPct(m.loss)}`} sx={lossChipColor(m.loss)} />
          {asn && <Chip size="small" label={asn} sx={{ bgcolor: '#f1f5f9', color: palette.text2 }} />}
        </Stack>
      </CardActionArea>
    </Card>
  );
}

function ChartDialog({ open, title, url, target, initialRange, onClose }) {
  const [range, setRange] = useState(initialRange);
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => { if (open) setRange(initialRange); }, [open, initialRange]);
  const load = useCallback(() => {
    if (!open || !url) return;
    setLoading(true);
    getJSON(withQuery(url.base, url.path, { ip: target, starttime: range.start, endtime: range.end }))
      .then(setData).catch(() => setData(null)).finally(() => setLoading(false));
  }, [open, url, target, range.start, range.end]);
  useEffect(load, [open, url, target]); // eslint-disable-line react-hooks/exhaustive-deps

  const option = useMemo(() => (data ? bigOption(data) : null), [data]);
  const toInput = (s) => (s || '').replace(' ', 'T');
  const fromInput = (s) => (s || '').replace('T', ' ');

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="lg">
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', fontSize: 15, fontWeight: 700, borderBottom: `1px solid ${palette.border}` }}>
        <Box sx={{ flex: 1, minWidth: 0 }} component="span">{title}</Box>
        <IconButton size="small" onClick={onClose}><CloseIcon fontSize="small" /></IconButton>
      </DialogTitle>
      <DialogContent sx={{ pt: '20px !important' }}>
        <Stack direction="row" alignItems="center" gap={1.25} flexWrap="wrap" sx={{ mb: 2 }}>
          <TextField size="small" type="datetime-local" label="开始" value={toInput(range.start)} InputLabelProps={{ shrink: true }}
            onChange={(e) => setRange((r) => ({ ...r, start: fromInput(e.target.value) }))} />
          <TextField size="small" type="datetime-local" label="结束" value={toInput(range.end)} InputLabelProps={{ shrink: true }}
            onChange={(e) => setRange((r) => ({ ...r, end: fromInput(e.target.value) }))} />
          <Button variant="contained" disableElevation onClick={load}>查询</Button>
          {loading && <CircularProgress size={18} />}
        </Stack>
        <EChart option={option} height={420} />
      </DialogContent>
    </Dialog>
  );
}

export default function Overview({ rootCfg }) {
  const pref = useMemo(loadTimePref, []);
  const [mins, setMins] = useState(pref.mins);
  const [custom, setCustom] = useState(pref.custom);
  const [draft, setDraft] = useState(pref.custom || rangeFromPreset(360));
  const [editingCustom, setEditingCustom] = useState(!!pref.custom);
  const [agent, setAgent] = useState(rootCfg.Addr);
  const [agentCfg, setAgentCfg] = useState(null);
  const [stats, setStats] = useState({ delay: '-', bad: '-' });
  const [err, setErr] = useState('');
  const [dialog, setDialog] = useState(null);
  const [tick, setTick] = useState(0);

  const range = useMemo(() => custom || rangeFromPreset(mins), [custom, mins, tick]); // eslint-disable-line react-hooks/exhaustive-deps
  const label = timeWindowLabel(custom ? null : mins, custom);
  const nodes = useMemo(() => sourceNodes(rootCfg), [rootCfg]);
  const base = nodeBase(rootCfg, agent);

  useEffect(() => {
    try { localStorage.setItem(STORE_KEY, JSON.stringify({ mins, custom })); } catch (e) { /* ignore */ }
  }, [mins, custom]);

  useEffect(() => {
    const t = setInterval(() => setTick((x) => x + 1), ((rootCfg.Base && rootCfg.Base.Refresh) || 1) * 60000);
    return () => clearInterval(t);
  }, [rootCfg]);

  useEffect(() => {
    setAgentCfg(null);
    setStats({ delay: '-', bad: '-' });
    getJSON(base + '/api/config.json').then(setAgentCfg).catch(() => setErr(`无法连接节点 ${agent}`));
  }, [agent, base]);

  useEffect(() => {
    if (!agentCfg) return;
    const q = custom ? { start: custom.start, end: custom.end } : { mins };
    getJSON(withQuery(base, '/api/pingmesh.json', q)).then((row) => {
      const cells = Object.values(row.cells || {});
      if (!cells.length) { setStats({ delay: '-', bad: '-' }); return; }
      const sum = cells.reduce((a, c) => a + (c.avgdelay || 0), 0);
      const bad = cells.filter((c) => delayLevel(c.avgdelay, c.loss, c.baseline) >= 3).length;
      setStats({ delay: fmtMs(sum / cells.length) + ' ms', bad });
    }).catch(() => setStats({ delay: '-', bad: '-' }));
  }, [agentCfg, base, mins, custom, tick]);

  const self = agentCfg ? (agentCfg.Network || {})[agentCfg.Addr] || {} : {};
  const targets = self.Ping || [];
  const pingUrl = useMemo(() => ({ base, path: '/api/ping.json' }), [base]);

  const onPreset = (_, v) => {
    if (v == null) return;
    if (v === 'custom') { setEditingCustom(true); return; }
    setEditingCustom(false); setCustom(null); setMins(v);
  };
  const applyCustom = () => {
    if (!draft.start || !draft.end) { setErr('请选择起止时间'); return; }
    if (draft.start >= draft.end) { setErr('结束时间需晚于开始时间'); return; }
    setCustom({ ...draft });
  };

  return (
    <>
      <Box sx={{ display: 'grid', gap: 2, mb: 2, gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(4, 1fr)' } }}>
        <StatCard tone="indigo" icon={<HubOutlined />} value={nodes.length} label="监测节点" />
        <StatCard tone="green" icon={<PublicOutlined />} value={agentCfg ? targets.length : '-'} label="监测目标" />
        <StatCard tone="yellow" icon={<ScheduleOutlined />} value={stats.delay} label={`平均延迟 (${label})`} />
        <StatCard tone="red" icon={<WarningAmberOutlined />} value={stats.bad} label={`异常目标 (${label})`} />
      </Box>

      <Card>
        <CardHeader
          title="正向 Ping 监控"
          subheader="点击图表查看历史曲线，支持自定义时间范围"
          titleTypographyProps={{ fontSize: 14.5, fontWeight: 600 }}
          subheaderTypographyProps={{ fontSize: 12, color: palette.text3 }}
          action={!agentCfg ? <CircularProgress size={18} sx={{ m: 1.5 }} /> : null}
          sx={{ borderBottom: `1px solid ${palette.border}`, py: 1.5, px: 2.25, '& .MuiCardHeader-content': { display: 'flex', alignItems: 'baseline', gap: 1.25, flexWrap: 'wrap' } }}
        />
        <CardContent sx={{ p: 2.25 }}>
          <Stack direction="row" alignItems="center" gap={1.5} flexWrap="wrap"
            sx={{ p: '12px 14px', mb: 1.75, border: `1px solid ${palette.border}`, borderRadius: 3, background: 'linear-gradient(180deg, #f8fafc 0%, #fff 100%)' }}>
            <Typography sx={{ fontSize: 11, fontWeight: 700, color: palette.text3, letterSpacing: '.06em' }}>时间</Typography>
            <ToggleButtonGroup exclusive size="small" value={editingCustom ? 'custom' : mins} onChange={onPreset}>
              {PRESETS.map((p) => <ToggleButton key={p.v} value={p.v}>{p.label}</ToggleButton>)}
              <ToggleButton value="custom">自定义</ToggleButton>
            </ToggleButtonGroup>
            {editingCustom && (
              <Stack direction="row" alignItems="center" gap={0.75}>
                <TextField size="small" type="datetime-local" value={draft.start.replace(' ', 'T')}
                  onChange={(e) => setDraft((d) => ({ ...d, start: e.target.value.replace('T', ' ') }))} />
                <Typography sx={{ color: palette.text3 }}>~</Typography>
                <TextField size="small" type="datetime-local" value={draft.end.replace(' ', 'T')}
                  onChange={(e) => setDraft((d) => ({ ...d, end: e.target.value.replace('T', ' ') }))} />
                <Button size="small" variant="contained" disableElevation onClick={applyCustom}>应用</Button>
              </Stack>
            )}
            <Box sx={{ flex: 1 }} />
            {!custom && !editingCustom && <Typography sx={{ fontSize: 11.5, color: palette.text3 }}>{range.start} ~ {range.end}</Typography>}
            <Typography sx={{ fontSize: 11.5, color: palette.text3 }}>北京时间 (UTC+8)</Typography>
          </Stack>

          <Stack direction="row" flexWrap="wrap" gap={1} sx={{ mb: 2 }}>
            {nodes.map((n) => (
              <Chip key={n.Addr} label={n.Name} clickable onClick={() => setAgent(n.Addr)}
                color={n.Addr === agent ? 'primary' : 'default'} variant={n.Addr === agent ? 'filled' : 'outlined'}
                sx={n.Addr === agent ? {} : { bgcolor: '#fff', borderColor: palette.border, color: palette.text2 }} />
            ))}
          </Stack>

          {agentCfg && targets.length === 0 && (
            <Box sx={{ textAlign: 'center', py: 6, color: palette.text3 }}>当前节点没有配置监测目标，请到「系统配置 - 节点管理」中添加</Box>
          )}
          <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)', lg: 'repeat(3, 1fr)' } }}>
            {agentCfg && targets.map((t) => {
              const title = `${agentCfg.Name} → ${nodeName(agentCfg, t)}`;
              return (
                <PingCard key={`${agent}-${t}`} title={title} target={t} url={pingUrl} range={range}
                  onOpen={() => setDialog({ title: `${title} (${t})`, target: t })} />
              );
            })}
          </Box>
        </CardContent>
      </Card>

      <ChartDialog open={!!dialog} title={dialog ? dialog.title : ''} target={dialog ? dialog.target : ''}
        url={pingUrl} initialRange={range} onClose={() => setDialog(null)} />
      <Snackbar open={!!err} autoHideDuration={3200} onClose={() => setErr('')} anchorOrigin={{ vertical: 'top', horizontal: 'right' }}>
        <Alert severity="error" variant="filled" onClose={() => setErr('')}>{err}</Alert>
      </Snackbar>
    </>
  );
}
