import { useCallback, useEffect, useRef, useState } from 'react';
import { Box, Button, Switch, Table, TableBody, TableCell, TableHead, TableRow, TextField, Typography } from '@mui/material';
import { Badge, EmptyState, Hint, Panel, Spinner, StatCard, ResponsiveTable } from '../components/ui';
import { gridCols } from '../components/PingCard';
import { useToast } from '../components/Feedback';
import { getJSON, postForm } from '../api';
import { mono, palette } from '../theme';

const svg = (children) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">{children}</svg>
);
const ICONS = {
  master: svg(<><path d="M3 7l9-4 9 4-9 4-9-4z" /><path d="M3 7v10l9 4 9-4V7" /></>),
  online: svg(<><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" /><polyline points="22 4 12 14.01 9 11.01" /></>),
  epoch: svg(<><path d="M21 12a9 9 0 1 1-6.219-8.56" /><polyline points="21 3 21 9 15 9" /></>),
  mode: svg(<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />),
};

const muted = { color: palette.text3 };
const dash = <Box component="span" sx={muted}>—</Box>;

function apiErrMsg(prefix, e, ver) {
  if (e && e.name === 'AbortError') return prefix + '：请求超时，集群节点较多时请稍后重试';
  const status = e && e.status ? e.status : (e instanceof SyntaxError ? 200 : 0);
  if (status === 404) {
    return prefix + '：端口上实际运行的后端是 v' + (ver || '?') + '（需 v2.0.0+ 才有集群接口）。' +
      '通常是旧进程未被停掉仍占用端口：请在本节点执行 ' +
      'pkill -f /opt/pingmesh/pingmesh && systemctl restart pingmesh，' +
      '再用 curl http://127.0.0.1:端口/healthz 确认版本';
  }
  if (status === 401) return prefix + '：登录已过期，请刷新页面重新登录';
  if (status) return prefix + ' (HTTP ' + status + ')';
  return prefix + '：无法连接服务';
}

function RoleBadge({ n, acting }) {
  if (n.endpoint === acting) return <Badge tone="indigo" dot>代理主节点</Badge>;
  if (n.candidate) return <Badge tone="gray">备选主 #{n.priority + 1}</Badge>;
  return <Badge tone="gray">Agent</Badge>;
}
function StatusBadge({ n }) {
  if (n.self) return <Badge tone="green" dot>本机在线</Badge>;
  if (!n.online) return <Badge tone="red" dot>离线</Badge>;
  if (n.legacy) return <Badge tone="yellow" dot>在线 · 旧版本</Badge>;
  return <Badge tone="green" dot>在线</Badge>;
}

function FieldLabel({ children, htmlFor }) {
  return <Typography component="label" htmlFor={htmlFor} sx={{ display: 'block', fontSize: 13, fontWeight: 600, color: palette.text2, mb: '6px' }}>{children}</Typography>;
}

/* 旧版 .switch: 38x22 圆角轨道 */
const switchSx = {
  width: 38, height: 22, p: 0, flex: 'none',
  '& .MuiSwitch-switchBase': { p: '2px', '&.Mui-checked': { transform: 'translateX(16px)', color: '#fff', '& + .MuiSwitch-track': { bgcolor: palette.primary, opacity: 1 } } },
  '& .MuiSwitch-thumb': { width: 18, height: 18, boxShadow: '0 1px 3px rgba(0,0,0,.25)', bgcolor: '#fff' },
  '& .MuiSwitch-track': { borderRadius: 999, bgcolor: '#cbd5e1', opacity: 1 },
};

function ClusterAdmin({ config }) {
  const toast = useToast();
  const md = (config && config.Mode) || {};
  const [master, setMaster] = useState(md.Master || '');
  const [standbys, setStandbys] = useState(md.Standbys || '');
  const [auto, setAuto] = useState(md.MasterAuto !== 'false');
  const [res, setRes] = useState(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const loadingRef = useRef(false);
  const alive = useRef(true);

  const load = useCallback(() => {
    if (loadingRef.current) return;
    loadingRef.current = true;
    setLoading(true);
    getJSON('/api/cluster/status.json', { timeout: 30000 })
      .then((r) => {
        if (!alive.current) return;
        if (!r || r.status !== 'true') { toast('获取集群状态失败', 'err'); return; }
        setRes(r);
        setMaster(r.primary || '');
        setStandbys(r.standbys || '');
        setAuto(!!r.masterauto);
      })
      .catch((e) => alive.current && toast(apiErrMsg('获取集群状态失败', e, config.Ver), 'err'))
      .finally(() => {
        loadingRef.current = false;
        if (alive.current) setLoading(false);
      });
  }, [toast, config.Ver]);

  useEffect(() => {
    alive.current = true;
    load();
    const timer = setInterval(load, 15000);
    return () => { alive.current = false; clearInterval(timer); };
  }, [load]);

  const save = () => {
    setSaving(true);
    postForm('/api/cluster/masters.json', {
      master: master.trim(), standbys: standbys.trim(), masterauto: auto ? 'true' : 'false',
    }, { timeout: 15000 })
      .then((r) => {
        if (r.status === 'true') { toast(r.info || '已保存', 'ok'); load(); } else toast(r.info || '保存失败', 'err');
      })
      .catch((e) => toast(apiErrMsg('保存失败', e, config.Ver), 'err'))
      .finally(() => setSaving(false));
  };

  const nodes = (res && res.nodes) || [];
  const total = nodes.length;
  const online = nodes.filter((n) => n.online).length;
  const actingNode = res ? nodes.find((n) => n.endpoint === res.acting) || null : null;
  const selfNode = nodes.find((n) => n.self) || null;
  const single = total <= 1;
  const showBanner = res && selfNode && !single && res.acting && res.acting !== res.self;

  return (
    <>
      <Box sx={{ ...gridCols(4), mb: 2 }}>
        <StatCard tone="indigo" icon={ICONS.master} label="当前代理主节点"
          value={res ? (actingNode ? actingNode.name : (res.acting || '本机')) : '-'} />
        <StatCard tone="green" icon={ICONS.online} label="在线节点 / 总数"
          value={res ? <>{online} <Box component="span" sx={{ ...muted, fontSize: 15 }}>/ {total}</Box></> : '-'} />
        <StatCard tone="yellow" icon={ICONS.epoch} label="配置版本(纪元)"
          value={res ? <>{'#' + res.maxepoch}{' '}{res.converged
            ? <Badge tone="green" sx={{ fontSize: 11 }}>已收敛</Badge>
            : <Badge tone="yellow" sx={{ fontSize: 11 }}>同步中</Badge>}</> : '-'} />
        <StatCard tone="red" icon={ICONS.mode} label="容灾模式"
          value={res ? (res.masterauto ? <Badge tone="green">自动</Badge> : <Badge tone="gray">手动</Badge>) : '-'} />
      </Box>

      <Panel flat sx={{ mb: 2 }} title="集群节点态势" sub="主挂自动接管 · 配置全网同步收敛 · 任意节点持有完整副本永不丢失"
        actions={<>
          {loading && <Spinner />}
          <Button variant="outlined" size="small" onClick={load}
            sx={{ px: '10px', py: '5px', lineHeight: 'normal', fontSize: 12.5, borderRadius: '7px', minWidth: 0 }}>刷新</Button>
        </>}>
        <ResponsiveTable sx={{ '@media (max-width:600px)': { px: '10px' } }}>
        <Table sx={{ '& tbody tr:last-child td': { borderBottom: 'none' }, '& td': { lineHeight: 'normal' }, '& th': { textTransform: 'uppercase' } }}>
          <TableHead>
            <TableRow>
              <TableCell>节点</TableCell><TableCell>角色</TableCell><TableCell>状态</TableCell>
              <TableCell>运行模式</TableCell><TableCell>配置纪元</TableCell><TableCell>最近变更</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {nodes.map((n, i) => {
              const live = n.online && !n.legacy;
              return (
                <TableRow hover key={n.endpoint || i}>
                  <TableCell>
                    <b>{n.name || n.addr}</b>{n.self && <> <Box component="span" sx={muted}>(当前)</Box></>}
                    <Box sx={{ ...muted, fontFamily: mono, fontSize: 11.5 }}>{n.endpoint}</Box>
                  </TableCell>
                  <TableCell><RoleBadge n={n} acting={res.acting} /></TableCell>
                  <TableCell><StatusBadge n={n} /></TableCell>
                  <TableCell>{live ? <Badge tone="gray">{n.mode || '-'}</Badge> : dash}</TableCell>
                  <TableCell>
                    {live ? <>
                      <Box component="span" sx={{ fontFamily: mono, fontSize: 12.5 }}>#{n.epoch}</Box>
                      {n.epoch < res.maxepoch && <> <Badge tone="yellow" sx={{ fontSize: 10 }}>落后</Badge></>}
                    </> : dash}
                  </TableCell>
                  <TableCell sx={muted}>{n.epochtime || '—'}</TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
        </ResponsiveTable>
        {res && single && (
          <EmptyState icon="🔌">当前为单机模式，尚未组建集群。在其他机器上以 Agent 身份加入即可启用容灾。</EmptyState>
        )}
      </Panel>

      <Panel title="主节点策略" sub="指定主节点与备选，开启自动容灾后任一节点可在主挂时自动接管写入"
        bodySx={{ p: { xs: '14px', md: 2.25 } }}>
        {showBanner && (
          <Box sx={{
            background: `linear-gradient(90deg, ${palette.yellowSoft}, #fff)`, border: '1px solid #fde68a', color: '#92400e',
            borderRadius: '10px', p: '11px 14px', fontSize: 13, mb: 2, '& b': { color: '#b45309' },
          }}>
            提示：当前节点不是代理主节点，建议在 <b>{actingNode ? actingNode.name : res.acting}</b> ({res.acting}) 上修改配置，以确保改动以最高优先级在全网生效。
          </Box>
        )}
        <Box sx={gridCols(2)}>
          <Box sx={{ mb: 2 }}>
            <FieldLabel htmlFor="p-master">主节点 (host:port)</FieldLabel>
            <TextField id="p-master" fullWidth value={master} onChange={(e) => setMaster(e.target.value)} placeholder="10.0.0.1:8899"
              sx={{ '& input': { fontFamily: mono, fontSize: 12.5 } }} />
            <Hint sx={{ mt: '5px' }}>权威写入节点。集群组网后自动确立，可手动调整。</Hint>
          </Box>
          <Box sx={{ mb: 2 }}>
            <FieldLabel htmlFor="p-standbys">备选主节点（逗号分隔，按优先级排序）</FieldLabel>
            <TextField id="p-standbys" fullWidth value={standbys} onChange={(e) => setStandbys(e.target.value)} placeholder="10.0.0.2:8899, 10.0.0.3:8899"
              sx={{ '& input': { fontFamily: mono, fontSize: 12.5 } }} />
            <Hint sx={{ mt: '5px' }}>主节点不可达时按顺序接管。开启自动容灾时可留空。</Hint>
          </Box>
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25, mt: '4px' }}>
          <Switch checked={auto} onChange={(e) => setAuto(e.target.checked)} disableRipple sx={switchSx} inputProps={{ 'aria-label': '自动容灾' }} />
          <Box>
            <Typography sx={{ fontWeight: 600, fontSize: 13.5 }}>自动容灾（推荐）</Typography>
            <Typography sx={{ ...muted, fontSize: 12 }}>开启后所有探测节点都是主候选，按地址优先级自动接管，无需人工指定备选。</Typography>
          </Box>
          <Box sx={{ flex: 1 }} />
          <Button variant="outlined" sx={{ flexShrink: 0 }} onClick={() => { window.location.href = '/api/config/export.json'; }}>导出配置备份</Button>
          <Button variant="contained" sx={{ flexShrink: 0 }} disabled={saving} onClick={save}>{saving ? '保存中...' : '保存策略'}</Button>
        </Box>
      </Panel>

      <Panel title="容灾说明" sx={{ mt: 2 }} bodySx={{ fontSize: 13, color: palette.text2, lineHeight: 2, p: { xs: '14px', md: 2.25 } }}>
        <b>配置永不丢失</b>：每个节点本地都持有一份完整配置副本，任意单点（含主节点）宕机都不会丢失。<br />
        <b>主挂自动接管</b>：主节点不可达时，下一优先级的在线候选自动成为代理主节点，继续接受配置变更与新节点加入。<br />
        <b>全网收敛</b>：每条配置带单调递增的纪元；任何节点上的权威改动都会在下一同步周期（≤60秒）被全网采纳（纪元高者胜出）。<br />
        <b>监控数据去中心化</b>：探测数据分布存储在各自节点，全网矩阵由各节点实时汇总，无中心数据库单点。
      </Panel>
    </>
  );
}

export default function Cluster({ config, isAdmin }) {
  if (!isAdmin) {
    return <EmptyState icon="🔒">没有权限访问该页面</EmptyState>;
  }
  return <ClusterAdmin config={config} />;
}
