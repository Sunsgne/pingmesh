import { useEffect, useState } from 'react';
import { Box, Table, TableBody, TableCell, TableHead, TableRow } from '@mui/material';
import { Badge, EmptyState, Mono } from './ui';
import { asnLookup, asnShort } from '../api';
import { palette } from '../theme';

function HopAsn({ host }) {
  const [label, setLabel] = useState('');
  useEffect(() => {
    let alive = true;
    asnLookup(host).then((info) => alive && info && setLabel(asnShort(info)));
    return () => { alive = false; };
  }, [host]);
  return label ? <Badge tone="indigo" sx={{ fontSize: 10, ml: 0.5 }}>{label}</Badge> : null;
}

const sub = { fontSize: 11, color: palette.text3, fontVariantNumeric: 'tabular-nums' };
const muted = <Box component="span" sx={{ color: palette.text3 }}>—</Box>;

/** MTR 逐跳结果, 对应旧版 SP.renderMtr + SP.fillMtrAsn(时间单位: 纳秒) */
export default function MtrTable({ hops, targetIp }) {
  hops = hops || [];
  if (hops.length === 0) return <EmptyState icon="🚧">无 MTR 数据</EmptyState>;
  let lastResp = -1;
  hops.forEach((h, i) => { if (h.Host && h.Host !== '???') lastResp = i; });
  const trailing = hops.length - 1 - lastResp;
  const shown = trailing > 1 ? hops.slice(0, lastResp + 1) : hops;
  let maxAvg = 0.001;
  shown.forEach((h) => { const a = h.Avg / 1e6; if (h.Host !== '???' && a > maxAvg) maxAvg = a; });

  return (
    <Table size="small" sx={{ '& td, & th': { px: 1.75, py: 1.1, fontSize: 13 } }}>
      <TableHead>
        <TableRow>
          <TableCell sx={{ width: 46 }}>#</TableCell>
          <TableCell>节点</TableCell>
          <TableCell sx={{ width: 90 }}>丢包率</TableCell>
          <TableCell>平均延迟</TableCell>
          <TableCell sx={{ width: 150 }}>最近 / 最优 / 最差</TableCell>
          <TableCell sx={{ width: 80 }}>抖动</TableCell>
        </TableRow>
      </TableHead>
      <TableBody>
        {shown.map((h, i) => {
          const timeout = h.Host === '???';
          const loss = h.Send > 0 ? (h.Loss / h.Send) * 100 : 0;
          const avg = h.Avg / 1e6;
          const reached = !timeout && targetIp && h.Host === targetIp;
          const pct = Math.max(2, Math.round((avg / maxAvg) * 100));
          const bar = avg >= 150 ? 'linear-gradient(90deg, #fb7185, #f43f5e)' : avg >= 50 ? 'linear-gradient(90deg, #f59e0b, #fb923c)' : 'linear-gradient(90deg, #6366f1, #8b5cf6)';
          return (
            <TableRow key={i} sx={{ ...(reached ? { '& td': { bgcolor: palette.greenSoft } } : {}), ...(timeout ? { '& td': { color: '#b6c2d2' } } : {}) }}>
              <TableCell>
                <Box sx={{ width: 24, height: 24, borderRadius: '50%', display: 'grid', placeItems: 'center', fontSize: 11.5, fontWeight: 700,
                  bgcolor: timeout ? '#f1f5f9' : palette.primarySoft, color: timeout ? '#b6c2d2' : palette.primary }}>{i + 1}</Box>
              </TableCell>
              <TableCell>
                {timeout ? <Box component="span" sx={{ color: palette.text3 }}>* * * 无响应(不回 TTL 超时报文)</Box> : (
                  <>
                    <Mono>{h.Host}</Mono>
                    <HopAsn host={h.Host} />
                    {reached && <Badge tone="green" dot sx={{ fontSize: 10, ml: 0.5 }}>到达目标</Badge>}
                  </>
                )}
              </TableCell>
              <TableCell>{timeout ? muted : <Badge tone={loss >= 50 ? 'red' : loss >= 10 ? 'yellow' : 'green'}>{loss.toFixed(0)}%</Badge>}</TableCell>
              <TableCell>
                {timeout ? muted : (
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 170 }}>
                    <Box sx={{ flex: 1, height: 7, bgcolor: '#f1f5f9', borderRadius: 99, overflow: 'hidden' }}>
                      <Box sx={{ height: '100%', width: `${pct}%`, borderRadius: 99, background: bar, minWidth: 2 }} />
                    </Box>
                    <Box component="span" sx={{ fontVariantNumeric: 'tabular-nums', fontWeight: 700, fontSize: 12.5, minWidth: 64, textAlign: 'right' }}>{avg.toFixed(2)} ms</Box>
                  </Box>
                )}
              </TableCell>
              <TableCell>{timeout ? muted : <Box component="span" sx={sub}>{(h.Last / 1e6).toFixed(1)} / {(h.Best / 1e6).toFixed(1)} / {(h.Wrst / 1e6).toFixed(1)}</Box>}</TableCell>
              <TableCell>{timeout ? muted : <Box component="span" sx={sub}>{Number(h.StDev).toFixed(2)} ms</Box>}</TableCell>
            </TableRow>
          );
        })}
        {trailing > 1 && (
          <TableRow>
            <TableCell sx={{ color: '#b6c2d2' }}>⋯</TableCell>
            <TableCell colSpan={5} sx={{ color: palette.text3 }}>后续 {trailing} 跳均无响应(目标或沿途设备不回 TTL 超时报文, 属常见现象)</TableCell>
          </TableRow>
        )}
      </TableBody>
    </Table>
  );
}
