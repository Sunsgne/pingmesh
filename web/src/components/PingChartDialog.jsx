import { useEffect, useMemo, useRef, useState } from 'react';
import { Box, Button, Dialog, DialogContent, DialogTitle, IconButton, Stack, TextField, Typography } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import EChart from './EChart';
import { Spinner } from './ui';
import { useToast } from './Feedback';
import { aggregationHint, bigChartOption } from '../charts';
import { fmtBeijing, fromInputTime, getJSON, pingUrlWithRange, rangeFromPreset, toInputTime } from '../api';
import { palette } from '../theme';

const timeGroup = { display: 'flex', alignItems: 'center', gap: 1.25, '@media (max-width:600px)': { width: '100%' } };
const timeInput = { width: 200, '@media (max-width:600px)': { flex: 1, width: 'auto' } };
const toMs = (s) => new Date(String(s).replace(' ', 'T') + '+08:00').getTime();
// 触屏上拖动留给页面滚动, 改用下方滑块缩放
const coarsePointer = () => typeof window !== 'undefined' && window.matchMedia && window.matchMedia('(pointer: coarse)').matches;

/**
 * 历史曲线弹窗, 对应旧版 SP.openPingChart(title, apiurl, start, end)。
 * chart = { title, apiurl, start, end } 打开, null 关闭; apiurl 与旧版相同(已含 ?ip=, 可为 proxy URL)。
 */
export default function PingChartDialog({ chart, onClose }) {
  const toast = useToast();
  const [range, setRange] = useState({ start: '', end: '' });
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(false);
  const [hint, setHint] = useState('');
  // 放大历史: 每次框选前的时间段, 用于「返回上一级」; origin 为打开弹窗时的时间段
  const [stack, setStack] = useState([]);
  const origin = useRef(null);
  const chartRef = useRef(null);
  const dataRef = useRef(null);
  const rangeRef = useRef(range);
  rangeRef.current = range;
  const touch = useMemo(coarsePointer, []);

  // 只采用最近一次请求的结果: 关掉 A 再打开 B 时, A 迟到的响应不能画到 B 的标题下
  const seq = useRef(0);
  const load = (apiurl, start, end) => {
    const my = ++seq.current;
    setLoading(true);
    setHint('');
    getJSON(pingUrlWithRange(apiurl, start, end), { timeout: 60000 })
      .then((d) => { if (my === seq.current) { dataRef.current = d; setData(d); setHint(aggregationHint(d.step)); } })
      .catch(() => { if (my === seq.current) toast('获取数据失败（时间跨度过大或节点超时）', 'err'); })
      .finally(() => { if (my === seq.current) setLoading(false); });
  };

  useEffect(() => {
    if (!chart) { seq.current++; return; }
    const r = chart.start && chart.end ? { start: chart.start, end: chart.end } : rangeFromPreset(120);
    origin.current = r;
    setStack([]);
    setRange(r);
    setData(null);
    dataRef.current = null;
    load(chart.apiurl, r.start, r.end);
  }, [chart]); // eslint-disable-line react-hooks/exhaustive-deps

  const option = useMemo(() => bigChartOption(data), [data]);

  // 每次重绘后重新进入框选模式(setOption notMerge 会重置鼠标模式)
  useEffect(() => {
    const c = chartRef.current && chartRef.current.instance;
    if (!c || touch) return;
    c.dispatchAction({ type: 'takeGlobalCursor', key: 'brush', brushOption: { brushType: 'lineX', brushMode: 'single' } });
  }, [option, touch]);

  // 按横轴下标区间放大: 起点向下取整到分钟, 终点包含最后一个桶并向上取整到分钟
  const zoomTo = (i0, i1) => {
    const d = dataRef.current;
    const lc = (d && d.lastcheck) || [];
    if (!chart || lc.length < 2) return;
    const a = Math.max(0, Math.min(i0, i1));
    const b = Math.min(lc.length - 1, Math.max(i0, i1));
    if (b - a < 1 || (a === 0 && b === lc.length - 1)) return;
    const step = (parseInt(d.step, 10) || 10) * 1000;
    const startMs = Math.floor(toMs(lc[a]) / 60000) * 60000;
    let endMs = Math.ceil((toMs(lc[b]) + step) / 60000) * 60000;
    if (endMs - startMs < 2 * 60000) endMs = startMs + 2 * 60000;
    const next = { start: fmtBeijing(new Date(startMs)), end: fmtBeijing(new Date(endMs)) };
    const cur = rangeRef.current;
    if (next.start === cur.start && next.end === cur.end) return;
    setStack((s) => [...s, cur]);
    setRange(next);
    load(chart.apiurl, next.start, next.end);
  };
  const goto = (r) => { setRange(r); if (chart) load(chart.apiurl, r.start, r.end); };
  const back = () => {
    if (!stack.length) return;
    goto(stack[stack.length - 1]);
    setStack(stack.slice(0, -1));
  };
  const reset = () => { setStack([]); if (origin.current) goto(origin.current); };

  const onEvents = useMemo(() => ({
    brushEnd: (p, c) => {
      const area = p && p.areas && p.areas[0];
      c.dispatchAction({ type: 'brush', areas: [] });
      if (area && area.coordRange) zoomTo(Math.round(area.coordRange[0]), Math.round(area.coordRange[1]));
    },
    // 滑块缩放(松手才触发)同样按选中区间重新查询, 拿到更细的数据
    datazoom: (p, c) => {
      const lc = (dataRef.current && dataRef.current.lastcheck) || [];
      const z = (c.getOption().dataZoom || [])[0];
      if (!z || lc.length < 2) return;
      const n = lc.length - 1;
      const i0 = z.startValue != null ? z.startValue : Math.round((z.start / 100) * n);
      const i1 = z.endValue != null ? z.endValue : Math.round((z.end / 100) * n);
      zoomTo(i0, i1);
    },
  }), [chart]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <Dialog open={!!chart} onClose={onClose} fullWidth maxWidth={false} PaperProps={{ sx: { maxWidth: 1240 } }}>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', borderBottom: `1px solid ${palette.border}`, py: 2, px: 2.5 }}>
        <Box component="span" sx={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{chart ? chart.title : '历史曲线'}</Box>
        <IconButton size="small" onClick={onClose} aria-label="关闭"><CloseIcon fontSize="small" /></IconButton>
      </DialogTitle>
      <DialogContent sx={{ p: '20px !important', '@media (max-width:900px)': { p: '14px !important' } }}>
        <Stack direction="row" alignItems="center" gap={1.25} flexWrap="wrap" sx={{ mb: 2 }}>
          <Box sx={timeGroup}>
            <Typography sx={{ fontSize: 12.5, color: palette.text3, flex: 'none' }}>开始</Typography>
            <TextField size="small" type="datetime-local" value={toInputTime(range.start)} sx={timeInput}
              onChange={(e) => setRange((r) => ({ ...r, start: fromInputTime(e.target.value) }))} />
          </Box>
          <Box sx={timeGroup}>
            <Typography sx={{ fontSize: 12.5, color: palette.text3, flex: 'none' }}>结束</Typography>
            <TextField size="small" type="datetime-local" value={toInputTime(range.end)} sx={timeInput}
              onChange={(e) => setRange((r) => ({ ...r, end: fromInputTime(e.target.value) }))} />
          </Box>
          <Button size="small" variant="contained" onClick={() => {
            if (!chart) return;
            // 手动查询的时间段作为新的「重置」起点
            origin.current = range;
            setStack([]);
            load(chart.apiurl, range.start, range.end);
          }}>查询</Button>
          {stack.length > 0 && <>
            <Button size="small" variant="outlined" onClick={back}>返回上一级</Button>
            <Button size="small" variant="text" onClick={reset}>重置</Button>
          </>}
          <Box sx={{ flex: 1 }} />
          <Typography sx={{ fontSize: 12, color: palette.text3 }}>{touch ? '拖动下方滑块可放大' : '在图上按住拖动可框选放大'}</Typography>
          {hint && <Typography sx={{ fontSize: 12, color: palette.text3 }}>{hint}</Typography>}
          {loading && <Spinner />}
        </Stack>
        <Box sx={{ height: 420, minHeight: 260, '@media (max-width:900px)': { height: 300 }, '@media (max-width:480px)': { height: 260 } }}>
          <EChart ref={chartRef} option={option} onEvents={onEvents} height="100%" initOpts={{ renderer: 'canvas', useDirtyRect: true }} />
        </Box>
      </DialogContent>
    </Dialog>
  );
}
