import { useEffect, useMemo, useState } from 'react';
import { Box, Card, CardActionArea, Stack, Typography } from '@mui/material';
import EChart from './EChart';
import { Badge, cardHover } from './ui';
import { miniChartOption } from '../charts';
import { asnLookup, asnShort, getJSON, lastMetric, pingUrlWithRange } from '../api';

export function lossTone(v) {
  if (v == null) return 'green';
  if (v >= 20) return 'red';
  if (v >= 5) return 'yellow';
  return 'green';
}

/** 迷你曲线卡片(概览/反向 Ping 共用), 对应旧版 .ping-card */
export function PingCard({ title, target, apiurl, range, onOpen }) {
  const [data, setData] = useState(null);
  const [failed, setFailed] = useState(false);
  const [asn, setAsn] = useState(null);

  useEffect(() => {
    let alive = true;
    setFailed(false);
    getJSON(pingUrlWithRange(apiurl, range.start, range.end))
      .then((d) => alive && setData(d))
      .catch(() => alive && setFailed(true));
    return () => { alive = false; };
  }, [apiurl, range.start, range.end]);

  useEffect(() => {
    let alive = true;
    asnLookup(target).then((info) => alive && setAsn(info));
    return () => { alive = false; };
  }, [target]);

  const option = useMemo(() => miniChartOption(data), [data]);
  const m = data ? lastMetric(data.avgdelay, data.losspk) : { delay: '-', loss: '-', lossRaw: null };

  return (
    <Card sx={{ ...cardHover, cursor: 'pointer' }}>
      <CardActionArea onClick={onOpen} disableRipple sx={{ '& .MuiCardActionArea-focusHighlight': { display: 'none' } }}>
        <Typography noWrap sx={{ px: 2, pt: 1.75, pb: 0.5, fontSize: 13.5, fontWeight: 600 }}>{title}</Typography>
        <Box sx={{ px: 0.5 }}><EChart option={option} height={130} /></Box>
        <Stack direction="row" flexWrap="wrap" gap={0.75} sx={{ px: 2, pb: 1.75, pt: 0.5 }}>
          <Badge tone="gray" mono>{target}</Badge>
          <Badge tone="indigo">{failed ? '失败' : `${m.delay} ms`}</Badge>
          <Badge tone={lossTone(m.lossRaw)}>丢包率 {m.loss}</Badge>
          {asn && asn.asn && <Badge tone="gray" title={`AS${asn.asn} ${asn.holder} ${asn.prefix || ''}`}>{asnShort(asn)}</Badge>}
        </Stack>
      </CardActionArea>
    </Card>
  );
}

// 旧版 .grid.cols-N: ≤720px 单列, ≤1100px 两列
export const gridCols = (n) => ({
  display: 'grid', gap: 2, gridTemplateColumns: `repeat(${n}, 1fr)`,
  '@media (max-width:1100px)': { gridTemplateColumns: n >= 3 ? 'repeat(2, 1fr)' : `repeat(${n}, 1fr)` },
  '@media (max-width:720px)': { gridTemplateColumns: '1fr' },
});

