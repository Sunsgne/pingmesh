import { useEffect, useMemo, useState } from 'react';
import { Box, Chip, Stack } from '@mui/material';
import { EmptyState, Panel, Spinner } from '../components/ui';
import { cardGrid, gridCols, PingCard } from '../components/PingCard';
import { TimeToolbar, useTimeWindow } from '../components/TimeFilter';
import PingChartDialog from '../components/PingChartDialog';
import { useToast } from '../components/Feedback';
import { getJSON, nodeName, sourceNodes } from '../api';
import { palette } from '../theme';

export default function Reverse({ config: rootCfg }) {
  const toast = useToast();
  const tw = useTimeWindow('reverse-time', 120);
  const nodes = useMemo(() => sourceNodes(rootCfg), [rootCfg]);
  const [agent, setAgent] = useState(rootCfg.Addr);
  // 与旧版一致: 新节点配置到达前保留上一组卡片
  const [loaded, setLoaded] = useState(null);
  const [loading, setLoading] = useState(true);
  const [chart, setChart] = useState(null);

  useEffect(() => {
    let alive = true;
    const isSelf = agent === rootCfg.Addr;
    const base = isSelf ? '' : `/api/proxy.json?g=http://${agent}:${rootCfg.Port}`;
    const range = tw.range;
    setLoading(true);
    getJSON(base + '/api/config.json')
      .then((cfg) => alive && setLoaded({ addr: agent, cfg, base, range }))
      .catch(() => alive && toast('无法连接节点 ' + agent, 'err'))
      .finally(() => alive && setLoading(false));
    return () => { alive = false; };
  }, [agent, tw.range.start, tw.range.end]); // eslint-disable-line react-hooks/exhaustive-deps

  const srcName = loaded ? loaded.cfg.Name || nodeName(rootCfg, loaded.addr) : '';
  const targets = loaded ? nodes.filter((n) => n.Addr !== loaded.addr) : [];

  return (
    <>
      <Panel title="反向 Ping 监控" sub="选择一个探测节点，查看它到其他节点的网络质量（源节点 → 目标节点）" actions={loading ? <Spinner /> : null}>
        <TimeToolbar tw={tw} />
        <Stack direction="row" flexWrap="wrap" gap={1} sx={{ mb: 2, '@media (max-width:900px)': { gap: '6px' } }}>
          {nodes.map((n) => {
            const active = n.Addr === agent;
            return (
              <Chip key={n.Addr} label={n.Name} clickable onClick={() => setAgent(n.Addr)}
                sx={{
                  height: 30, fontSize: 12.5, borderRadius: 999, '& .MuiChip-label': { px: '14px' },
                  '@media (max-width:900px)': { height: 28, fontSize: 12, '& .MuiChip-label': { px: '11px' } },
                  ...(active
                    ? { bgcolor: palette.primary, border: `1px solid ${palette.primary}`, color: '#fff', '&:hover': { bgcolor: palette.primary } }
                    : { bgcolor: '#fff', border: `1px solid ${palette.border}`, color: palette.text2, '&:hover': { bgcolor: '#fff', borderColor: palette.primary2, color: palette.primary } }),
                }} />
            );
          })}
        </Stack>
        <Box sx={cardGrid}>
          {loaded && targets.map((t) => {
            const apiurl = loaded.base ? `${loaded.base}/api/ping.json?ip=${t.Addr}` : `/api/ping.json?ip=${t.Addr}`;
            const { range } = loaded;
            return (
              <PingCard key={`${loaded.addr}|${t.Addr}`} title={`${srcName} → ${t.Name}`} target={t.Addr} apiurl={apiurl} range={range}
                onOpen={() => setChart({ title: `${srcName} → ${t.Name} (${t.Addr})`, apiurl, start: range.start, end: range.end })} />
            );
          })}
        </Box>
        {loaded && targets.length === 0 && <EmptyState icon="📥">没有其他探测节点可对比</EmptyState>}
      </Panel>
      <PingChartDialog chart={chart} onClose={() => setChart(null)} />
    </>
  );
}
