import { useEffect, useMemo, useState } from 'react';
import { Box, Chip, Stack } from '@mui/material';
import { EmptyState, Panel, Spinner, StatCard } from '../components/ui';
import { STAT_ICONS } from '../components/icons';
import { gridCols, PingCard } from '../components/PingCard';
import { TimeToolbar, useTimeWindow } from '../components/TimeFilter';
import PingChartDialog from '../components/PingChartDialog';
import { useToast } from '../components/Feedback';
import { delayLevel, fmtMs, getJSON, meshQueryString, nodeName, proxyMeshUrl, sourceNodes } from '../api';
import { palette } from '../theme';

export default function Overview({ config: rootCfg }) {
  const toast = useToast();
  const tw = useTimeWindow('index-time', 120);
  const nodes = useMemo(() => sourceNodes(rootCfg), [rootCfg]);
  const [agent, setAgent] = useState(rootCfg.Addr);
  const [agentCfg, setAgentCfg] = useState(null);
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState({ delay: '-', bad: '-' });
  const [chart, setChart] = useState(null);

  const isSelf = agent === rootCfg.Addr;
  const base = isSelf ? '' : `/api/proxy.json?g=http://${agent}:${rootCfg.Port}`;

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setAgentCfg(null);
    setStats({ delay: '-', bad: '-' });
    getJSON(base + '/api/config.json')
      .then((cfg) => alive && setAgentCfg(cfg))
      .catch(() => alive && toast('无法连接节点 ' + agent, 'err'))
      .finally(() => alive && setLoading(false));
    return () => { alive = false; };
  }, [agent]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!agentCfg) return undefined;
    let alive = true;
    const mins = tw.custom ? null : tw.mins;
    const url = isSelf ? '/api/pingmesh.json' + meshQueryString(mins, tw.custom) : proxyMeshUrl(agent, rootCfg.Port, mins, tw.custom);
    getJSON(url).then((row) => {
      if (!alive) return;
      const cells = Object.values(row.cells || {});
      let sum = 0; let bad = 0;
      cells.forEach((c) => {
        sum += c.avgdelay;
        if (delayLevel(c.avgdelay, c.loss, c.baseline) >= 3) bad++;
      });
      setStats({ delay: cells.length ? fmtMs(sum / cells.length) + ' ms' : '-', bad: cells.length ? bad : '-' });
    }).catch(() => {});
    return () => { alive = false; };
  }, [agentCfg, tw.range.start, tw.range.end]); // eslint-disable-line react-hooks/exhaustive-deps

  const self = agentCfg ? (agentCfg.Network || {})[agentCfg.Addr] || {} : {};
  const targets = self.Ping || [];

  return (
    <>
      <Box sx={{ ...gridCols(4), mb: 2 }}>
        <StatCard tone="indigo" icon={STAT_ICONS.nodes} value={agentCfg ? nodes.length : '-'} label="监测节点" />
        <StatCard tone="green" icon={STAT_ICONS.globe} value={agentCfg ? targets.length : '-'} label="监测目标" />
        <StatCard tone="yellow" icon={STAT_ICONS.clock} value={stats.delay} label={`平均延迟 (${tw.label})`} />
        <StatCard tone="red" icon={STAT_ICONS.warn} value={stats.bad} label={`异常目标 (${tw.label})`} />
      </Box>

      <Panel title="正向 Ping 监控" sub="点击图表查看历史曲线，支持自定义时间范围" actions={loading ? <Spinner /> : null}>
        <TimeToolbar tw={tw} />
        <Stack direction="row" flexWrap="wrap" gap={1} sx={{ mb: 2 }}>
          {nodes.map((n) => {
            const active = n.Addr === agent;
            return (
              <Chip key={n.Addr} label={n.Name} clickable onClick={() => setAgent(n.Addr)}
                sx={{
                  height: 30, fontSize: 12.5, borderRadius: 999, '@media (max-width:900px)': { height: 28, fontSize: 12 },
                  ...(active
                    ? { bgcolor: palette.primary, color: '#fff', '&:hover': { bgcolor: palette.primary } }
                    : { bgcolor: '#fff', border: `1px solid ${palette.border}`, color: palette.text2, '&:hover': { bgcolor: '#fff', borderColor: palette.primary2, color: palette.primary } }),
                }} />
            );
          })}
        </Stack>
        {agentCfg && targets.length === 0 && (
          <EmptyState icon="📡">当前节点没有配置监测目标，请到「系统配置 - 节点管理」中添加</EmptyState>
        )}
        <Box sx={gridCols(3)}>
          {agentCfg && targets.map((t) => {
            const tname = nodeName(agentCfg, t);
            const apiurl = base ? `${base}/api/ping.json?ip=${t}` : `/api/ping.json?ip=${t}`;
            return (
              <PingCard key={`${agent}|${t}`} title={`${agentCfg.Name} → ${tname}`} target={t} apiurl={apiurl} range={tw.range}
                onOpen={() => setChart({ title: `${agentCfg.Name} → ${tname} (${t})`, apiurl, start: tw.range.start, end: tw.range.end })} />
            );
          })}
        </Box>
      </Panel>
      <PingChartDialog chart={chart} onClose={() => setChart(null)} />
    </>
  );
}
