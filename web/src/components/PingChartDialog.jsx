import { useEffect, useMemo, useState } from 'react';
import { Box, Button, Dialog, DialogContent, DialogTitle, IconButton, Stack, TextField, Typography } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import EChart from './EChart';
import { Spinner } from './ui';
import { useToast } from './Feedback';
import { aggregationHint, bigChartOption } from '../charts';
import { fromInputTime, getJSON, pingUrlWithRange, rangeFromPreset, toInputTime } from '../api';
import { palette } from '../theme';

const timeGroup = { display: 'flex', alignItems: 'center', gap: 1.25, '@media (max-width:600px)': { width: '100%' } };
const timeInput = { width: 200, '@media (max-width:600px)': { flex: 1, width: 'auto' } };

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

  const load = (apiurl, start, end) => {
    setLoading(true);
    setHint('');
    getJSON(pingUrlWithRange(apiurl, start, end), { timeout: 60000 })
      .then((d) => { setData(d); setHint(aggregationHint(d.step)); })
      .catch(() => toast('获取数据失败（时间跨度过大或节点超时）', 'err'))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    if (!chart) return;
    const r = chart.start && chart.end ? { start: chart.start, end: chart.end } : rangeFromPreset(120);
    setRange(r);
    setData(null);
    load(chart.apiurl, r.start, r.end);
  }, [chart]); // eslint-disable-line react-hooks/exhaustive-deps

  const option = useMemo(() => bigChartOption(data), [data]);

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
          <Button size="small" variant="contained" onClick={() => chart && load(chart.apiurl, range.start, range.end)}>查询</Button>
          <Box sx={{ flex: 1 }} />
          {hint && <Typography sx={{ fontSize: 12, color: palette.text3 }}>{hint}</Typography>}
          {loading && <Spinner />}
        </Stack>
        <Box sx={{ height: 420, minHeight: 260, '@media (max-width:900px)': { height: 300 }, '@media (max-width:480px)': { height: 260 } }}>
          <EChart option={option} height="100%" initOpts={{ renderer: 'canvas', useDirtyRect: true }} />
        </Box>
      </DialogContent>
    </Dialog>
  );
}
