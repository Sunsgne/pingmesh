import { useEffect, useMemo, useRef, useState } from 'react';
import { useMediaQuery } from '@mui/material';
import {
  Autocomplete, Box, Button, Checkbox, FormControlLabel, MenuItem, Select, Stack, Switch, TextField,
} from '@mui/material';
import { Badge, EmptyState, Mono, Panel, Spinner, ToolbarBox, ToolbarLabel } from '../components/ui';
import { TimeToolbar, useTimeWindow } from '../components/TimeFilter';
import EChart from '../components/EChart';
import { asnLookup, getJSON, proxyTopologyUrl, rangeQueryString } from '../api';
import { palette } from '../theme';
import { applyStatus, baseOption, buildGraph, buildModel } from './topology/graph';
import DiagDialog from './topology/DiagDialog';

const STATE_KEY = 'sp-topo-state';
const NARROW = '@media (max-width: 960px)';
const chartBoxSx = { height: 'calc(100vh - 270px)', minHeight: 420, [NARROW]: { height: '56vh', minHeight: 360 } };

function initialState(model) {
  let state = null;
  try { state = JSON.parse(sessionStorage.getItem(STATE_KEY) || 'null'); } catch (e) { /* ignore */ }
  if (!state || typeof state !== 'object') {
    // 首次进入: 节点很多且有分组时默认聚合总览, 否则平铺全部
    state = { view: (model.count > 40 && model.groupNames.length >= 2) ? 'group' : 'all', group: '', qAddr: '', badOnly: false };
  }
  state = { view: state.view, group: state.group || '', qAddr: state.qAddr || '', badOnly: !!state.badOnly };
  if (model.groupNames.length < 2 && state.view === 'group') state.view = 'all';
  if (state.qAddr && !model.nodes[state.qAddr]) state.qAddr = '';
  if (state.group && model.groupNames.indexOf(state.group) < 0) state.group = '';
  return state;
}

function persistState(s) {
  try {
    sessionStorage.setItem(STATE_KEY, JSON.stringify({ view: s.view, group: s.group, qAddr: s.qAddr || '', badOnly: !!s.badOnly }));
  } catch (e) { /* ignore */ }
}

const selectSx = {
  fontSize: 13.5,
  '& .MuiSelect-select': { py: '5px', pl: '8px', pr: '28px !important', minHeight: 'unset' },
};

/* 旧版 .switch: 38×22 圆角开关 */
const switchSx = {
  width: 38, height: 22, p: 0,
  '& .MuiSwitch-switchBase': {
    p: '2px', '&.Mui-checked': { transform: 'translateX(16px)', color: '#fff' },
    '&.Mui-checked + .MuiSwitch-track': { bgcolor: palette.primary, opacity: 1 },
  },
  '& .MuiSwitch-thumb': { width: 18, height: 18, boxShadow: '0 1px 3px rgba(0,0,0,.25)' },
  '& .MuiSwitch-track': { borderRadius: 999, bgcolor: '#cbd5e1', opacity: 1 },
};

function stopAlarm() {
  if (window.__alertAudio) {
    window.__alertAudio.pause();
    window.__alertAudio = null;
  }
}

function startAlarm(src) {
  // 柔和双音提示, 降低音量避免刺耳
  const a = new Audio(src);
  a.loop = true; a.volume = 0.35;
  const p = a.play();
  if (p && p.catch) {
    p.catch(() => {
      // 浏览器自动播放策略: 等待首次交互后再播放
      const resume = () => {
        document.removeEventListener('click', resume);
        document.removeEventListener('keydown', resume);
        if (window.__alertAudio) window.__alertAudio.play().catch(() => {});
      };
      document.addEventListener('click', resume);
      document.addEventListener('keydown', resume);
    });
  }
  window.__alertAudio = a;
}

export default function Topology({ config: cfg }) {
  const topoCfg = useMemo(() => cfg.Topology || {}, [cfg]);
  const tw = useTimeWindow('topo-time', 120);
  const model = useMemo(() => buildModel(cfg), [cfg]);
  const empty = model.count === 0;
  const [state, setState] = useState(() => initialState(model));
  const [status, setStatus] = useState({});
  const [sound, setSound] = useState(true);
  const soundRef = useRef(sound);
  soundRef.current = sound;
  const [diag, setDiag] = useState(null);
  const lastGood = useRef({ data: [], links: [] });

  const sources = useMemo(() => Object.keys(model.nodes).filter((a) => model.nodes[a].source), [model]);

  const update = (patch) => {
    setState((s) => {
      const next = { ...s, ...patch };
      persistState(next);
      return next;
    });
  };

  /* ----- 探测节点状态: 每个探测节点的 topology.json(所选时间窗口) ----- */
  useEffect(() => {
    if (empty) return undefined;
    let alive = true;
    const init = {};
    sources.forEach((a) => { init[a] = { st: 'pending' }; });
    setStatus(init);
    const range = tw.range;
    const timeout = ((cfg.Base || {}).Timeout || 5) * 1000;
    sources.forEach((addr) => {
      const url = addr === cfg.Addr ? '/api/topology.json' + rangeQueryString(range) : proxyTopologyUrl(addr, cfg.Port, range);
      getJSON(url, { timeout })
        .then((data) => alive && setStatus((s) => ({ ...s, [addr]: { st: 'ok', data } })))
        .catch(() => alive && setStatus((s) => ({ ...s, [addr]: { st: 'fail' } })));
    });
    return () => { alive = false; };
  }, [tw.range]); // eslint-disable-line react-hooks/exhaustive-deps

  // 自动刷新链路状态(只重新拉数据, 不整页重载, 诊断弹窗和筛选保持不变)
  useEffect(() => {
    if (empty) return undefined;
    const t = setInterval(() => {
      if (!document.hidden) tw.refresh();
    }, ((cfg.Base || {}).Refresh || 1) * 60 * 1000);
    return () => clearInterval(t);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const st = useMemo(() => applyStatus(model, status), [model, status]);
  const phone = useMediaQuery('(max-width:600px)');
  // 手机屏幕节点缩小到约 60%, 环形布局放得下
  const drawCfg = useMemo(() => (phone
    ? { ...topoCfg, Tsymbolsize: String(Math.round((parseInt(topoCfg.Tsymbolsize, 10) || 70) * 0.6)) }
    : topoCfg), [topoCfg, phone]);
  const graph = useMemo(() => buildGraph(model, st, state, drawCfg), [model, st, state, drawCfg]);
  if (!graph.error) lastGood.current = graph;
  const shown = graph.error ? lastGood.current : graph;

  const option = useMemo(() => {
    const o = baseOption();
    o.series[0].data = shown.data;
    o.series[0].links = shown.links;
    // 手机屏幕: 给环形布局留出标签空间, 避免边缘节点被裁掉
    if (phone) Object.assign(o.series[0], { left: 44, right: 44, top: 44, bottom: 28 });
    return o;
  }, [shown, phone]);

  useEffect(() => {
    if (empty) return;
    if (st.hasAlert && soundRef.current && !window.__alertAudio) startAlarm(topoCfg.Tsound || '/alert-soft.wav');
  }, [graph]); // eslint-disable-line react-hooks/exhaustive-deps

  const onSound = (e) => {
    setSound(e.target.checked);
    if (!e.target.checked) stopAlarm();
  };

  const onEvents = {
    // 点击: 连线 → 告警判定诊断; 聚合视图的分组 → 下钻
    click: (p) => {
      if (p.dataType === 'edge') {
        if (!p.data || !p.data._diag) return;
        setDiag({ from: p.data._from, to: p.data._to, fromName: p.data._fromName, toName: p.data._toName });
        return;
      }
      if (p.dataType === 'node' && p.data && p.data._group) update({ view: 'all', group: p.data._group });
    },
    // 悬浮时按需查询 ASN(有缓存), 避免大规模拓扑一次性发起上百个查询
    mouseover: (p) => {
      if (p.dataType === 'node' && p.data && p.data._addr) asnLookup(p.data._addr);
    },
  };

  const grouped = state.view === 'group';
  const nodeOptions = useMemo(() => [''].concat(model.nodeOpts.map((o) => o.addr)), [model]);
  const overlayText = graph.error || (shown.data && shown.data.length === 0 ? '当前筛选条件下没有匹配的节点 / 链路' : '');

  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 280px', gap: 2, alignItems: 'start', [NARROW]: { gridTemplateColumns: '1fr' } }}>
      <Panel title="拓扑" sub="实线 = 正常，红色虚线 = 告警；点击连线查看诊断 · 链路颜色按所选时间窗口内的告警判定" sx={{ minWidth: 0 }}>
        <ToolbarBox>
          <TimeToolbar tw={tw} inline sx={{ flex: '1 1 auto', minWidth: 0 }} />
          <Stack direction="row" alignItems="center" gap={1} flexWrap="wrap" sx={{ minWidth: 0 }}>
            <ToolbarLabel>视图</ToolbarLabel>
            {model.groupNames.length >= 2 && (
              <Select size="small" value={state.view} sx={{ ...selectSx, minWidth: 120 }} title="节点很多时建议用「按分组聚合」"
                onChange={(e) => update({ view: e.target.value })}>
                <MenuItem value="all">全部节点</MenuItem>
                <MenuItem value="group">按分组聚合</MenuItem>
              </Select>
            )}
            <Select size="small" displayEmpty value={state.group} disabled={grouped} sx={{ ...selectSx, maxWidth: 150 }} title="只看某个分组"
              onChange={(e) => update({ group: e.target.value })}>
              <MenuItem value="">全部分组</MenuItem>
              {model.groupNames.map((g) => <MenuItem key={g} value={g}>{g}</MenuItem>)}
            </Select>
            <Autocomplete size="small" disableClearable autoHighlight disabled={grouped}
              options={nodeOptions} value={state.qAddr}
              getOptionLabel={(a) => (a ? model.nodes[a].name : '全部节点')}
              onChange={(_, v) => update({ qAddr: v || '' })}
              renderOption={(props, a) => {
                const { key, ...rest } = props;
                return <li key={a || '__all'} {...rest}>{a ? model.nodes[a].name : '全部节点'}</li>;
              }}
              slotProps={{ popper: { sx: { minWidth: 200 } } }}
              sx={{ width: 180, '& .MuiOutlinedInput-root': { py: '1px !important', fontSize: 13.5 } }}
              renderInput={(params) => <TextField {...params} title="只看某个节点" placeholder="搜索节点" />} />
          </Stack>
          <Stack direction="row" alignItems="center" gap={1} flexWrap="wrap">
            <FormControlLabel
              control={<Checkbox size="small" checked={state.badOnly} onChange={(e) => update({ badOnly: e.target.checked })}
                sx={{ p: 0, mr: '7px', '& .MuiSvgIcon-root': { fontSize: 18 } }} />}
              label="只看异常"
              sx={{ m: 0, py: '4px', '& .MuiFormControlLabel-label': { fontSize: 12.5, fontWeight: 600, color: palette.text2 },
                '&:hover .MuiFormControlLabel-label': { color: palette.primary } }} />
            <Stack direction="row" alignItems="center" sx={{ gap: '7px', fontSize: 12.5, color: palette.text2 }}>
              <Switch checked={sound} onChange={onSound} disableRipple sx={switchSx} inputProps={{ 'aria-label': '声音报警' }} />
              声音报警
            </Stack>
          </Stack>
        </ToolbarBox>
        <Box sx={{ position: 'relative' }}>
          {empty ? (
            <Box sx={chartBoxSx}>
              <EmptyState icon="🔗" sx={{ pt: '200px', fontSize: 14 }}>
                尚无拓扑数据：请到「系统配置 → 节点管理」添加节点并配置监测关系，或检查本节点配置是否完整（系统配置 → 高级 JSON）
              </EmptyState>
            </Box>
          ) : (
            <Box sx={chartBoxSx}>
              <EChart option={option} notMerge={false} height="100%" onEvents={onEvents} />
            </Box>
          )}
          {!empty && overlayText && (
            <EmptyState sx={{ position: 'absolute', left: 0, right: 0, top: '45%', pointerEvents: 'none', fontSize: 14 }}>{overlayText}</EmptyState>
          )}
          {!empty && graph.hint && (
            <Box sx={{ position: 'absolute', left: 18, bottom: 10, fontSize: 14 }}>{graph.hint}</Box>
          )}
        </Box>
      </Panel>

      <Box sx={{ minWidth: 0 }}>
        <Panel title="节点状态" sub={empty ? '' : sources.length + ' 个探测节点'} sx={{ mb: 2 }}
          bodySx={{ p: '8px 0', maxHeight: 430, overflowY: 'auto' }}>
          <Box component="table" sx={{
            width: '100%', borderCollapse: 'collapse', fontSize: 13.5, lineHeight: 'normal',
            '& td': { p: '11px 16px', borderBottom: '1px solid #f1f5f9', verticalAlign: 'middle' },
            '& tr:hover': { background: '#f8fafc' },
            '& tr:last-child td': { borderBottom: 'none' },
          }}>
            <tbody>
              {!empty && sources.map((addr) => {
                const s = status[addr] || { st: 'pending' };
                return (
                  <tr key={addr}>
                    <td>
                      {model.nodes[addr].name}<br />
                      <Mono sx={{ fontSize: 11, color: palette.text3 }}>{addr}</Mono>
                    </td>
                    <Box component="td" sx={{ textAlign: 'right' }}>
                      {s.st === 'pending' && <Spinner />}
                      {s.st === 'ok' && <Badge tone="green" dot>在线</Badge>}
                      {s.st === 'fail' && <Badge tone="red" dot>不可达</Badge>}
                    </Box>
                  </tr>
                );
              })}
            </tbody>
          </Box>
        </Panel>
        <Button component="a" href="alerts.html" variant="outlined" fullWidth sx={{ py: '8px', fontSize: 13.5 }}>
          查看报警记录 →
        </Button>
      </Box>

      <DiagDialog diag={diag} selfAddr={cfg.Addr} port={cfg.Port} onClose={() => setDiag(null)} />
    </Box>
  );
}
