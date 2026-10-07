import { useMemo, useRef, useState } from 'react';
import {
  Box, Button, Checkbox, Dialog, DialogContent, DialogTitle, IconButton, MenuItem, Table, TableBody, TableCell,
  TableHead, TableRow, TextField, Typography,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { Badge, Panel, Spinner, ToolbarBox, ToolbarLabel, ResponsiveTable } from '../components/ui';
import MtrTable from '../components/MtrTable';
import { useToast } from '../components/Feedback';
import { fmtLossPct, getText, HttpError } from '../api';
import { mono, palette } from '../theme';

const TYPE_META = {
  icmp: {
    label: 'ICMP PING',
    hint: 'ICMP PING：5 个探测包，统计延迟与丢包。目标填 IP 或域名',
    ph: '如 8.8.8.8 或 www.example.com',
    cols: ['解析 IP', '发送', '接收', '丢包率', '最大延迟', '最小延迟', '平均延迟', '状态'],
  },
  tcp: {
    label: 'TCP Ping',
    hint: 'TCP Ping：5 次 TCP 连接建立耗时，可探测被禁 PING 的主机与具体端口。目标填 host:端口（缺省 80）',
    ph: '如 www.example.com:443 或 10.0.0.1:22',
    cols: ['解析 IP', '端口', '发送', '成功', '失败率', '最大耗时', '最小耗时', '平均耗时', '状态'],
  },
  http: {
    label: 'HTTP (curl)',
    hint: 'HTTP(curl)：GET 请求并分段计时（DNS / 连接 / TLS / 首字节 / 总耗时），支持 http/https',
    ph: '如 https://www.example.com/api/health',
    cols: ['解析 IP', '状态码', 'DNS', '连接', 'TLS', '首字节', '总耗时', '大小', '状态'],
  },
  mtr: {
    label: 'MTR 路由追踪',
    hint: 'MTR：逐跳路由追踪（每跳 5 个探测包），定位丢包/高延迟发生在哪一跳。耗时约 10~30 秒',
    ph: '如 8.8.8.8 或 www.example.com',
    cols: ['解析 IP', '跳数', '末跳延迟', '路径详情', '状态'],
  },
  dns: {
    label: 'DNS 解析',
    hint: 'DNS 解析：各节点本地 DNS 解析结果与耗时，排查解析劫持 / 污染 / 区域差异',
    ph: '如 www.example.com',
    cols: ['解析结果', '耗时', '状态'],
  },
};

const ms1 = (v) => (v == null || Number.isNaN(Number(v)) ? '-' : Number(v).toFixed(1) + ' ms');
const lossTone = (v) => (v >= 20 ? 'red' : v >= 5 ? 'yellow' : 'green');
const cbSx = { p: 0, verticalAlign: 'middle', '& .MuiSvgIcon-root': { fontSize: 17 } };
const LossBadge = ({ v }) => <Badge tone={lossTone(v)}>{fmtLossPct(v)}</Badge>;

function resultCells(type, res, onMtr) {
  if (type === 'icmp') {
    const p = res.ping || {};
    return [res.ip || '-', p.SendPk, p.RevcPk, <LossBadge v={p.LossPk} />, ms1(p.MaxDelay), ms1(p.MinDelay), <b>{ms1(p.AvgDelay)}</b>];
  }
  if (type === 'tcp') {
    const p = res.ping || {};
    return [res.ip || '-', res.port || '-', p.SendPk, p.RevcPk, <LossBadge v={p.LossPk} />, ms1(p.MaxDelay), ms1(p.MinDelay), <b>{ms1(p.AvgDelay)}</b>];
  }
  if (type === 'http') {
    const h = res.http || {};
    return [
      res.ip || '-',
      <Badge tone={h.code >= 200 && h.code < 400 ? 'green' : 'red'}>{h.code}</Badge>,
      ms1(h.dnsms), ms1(h.connectms), h.tlsms > 0 ? ms1(h.tlsms) : '-', ms1(h.ttfbms), <b>{ms1(h.totalms)}</b>,
      h.size >= 1024 ? (h.size / 1024).toFixed(1) + ' KB' : h.size + ' B',
    ];
  }
  if (type === 'mtr') {
    const hops = res.mtr || [];
    return [
      res.ip || '-',
      hops.length + ' 跳',
      hops.length ? (hops[hops.length - 1].Avg / 1e6).toFixed(1) + ' ms' : '-',
      <Box component="a" onClick={onMtr} sx={{ cursor: 'pointer' }}>查看 {hops.length} 跳路径 →</Box>,
    ];
  }
  if (type === 'dns') {
    const d = res.dns || {};
    return [<Box component="span" sx={{ fontFamily: mono, fontSize: 12 }}>{(d.ips || []).join(', ')}</Box>, ms1(d.ms)];
  }
  return [];
}

function StatusCell({ row }) {
  if (!row || row.state === 'idle') return <Badge tone="gray">待检测</Badge>;
  if (row.state === 'skip') return <Badge tone="gray">已跳过</Badge>;
  if (row.state === 'run') return <Spinner />;
  if (row.state === 'fail') return <Badge tone="red" title={row.msg}>节点异常</Badge>;
  if (row.state === 'bad') return <Badge tone="yellow" title={row.msg || ''}>{(row.msg || '失败').substring(0, 18)}</Badge>;
  return <Badge tone="green" dot>完成</Badge>;
}

export default function Tools({ config: cfg }) {
  const toast = useToast();
  const nodes = useMemo(() => Object.values(cfg.Network || {}).filter((n) => n && n.Pingmesh), [cfg]);
  const [type, setType] = useState('icmp');
  const [target, setTarget] = useState(cfg.Addr || '');
  const [allChecked, setAllChecked] = useState(true);
  const [checked, setChecked] = useState({});
  const [rows, setRows] = useState({});
  const [mtr, setMtr] = useState(null);
  const runSeq = useRef(0);
  const meta = TYPE_META[type];

  const keyOf = (n) => `${n.Addr}:${cfg.Port}`;
  const isChecked = (k) => checked[k] !== false;
  const setRow = (k, v) => setRows((r) => ({ ...r, [k]: v }));

  const changeType = (t) => {
    runSeq.current++;
    setType(t);
    setRows({});
    setChecked({});
    setAllChecked(true);
  };

  const toggleAll = (v) => {
    setAllChecked(v);
    const next = {};
    nodes.forEach((n) => { next[keyOf(n)] = v; });
    setChecked(next);
  };

  const run = () => {
    const t = target.trim();
    const ttype = type;
    if (!t) { toast('请输入检测目标', 'err'); return; }
    const seq = ++runSeq.current;
    const timeout = ttype === 'mtr' ? 90000 : 35000;
    const proxyT = ttype === 'mtr' ? 80 : 30;
    nodes.forEach((n) => {
      const k = keyOf(n);
      if (!isChecked(k)) {
        setRows((r) => ({ ...r, [k]: { ...(r[k] || {}), state: 'skip' } }));
        return;
      }
      setRow(k, { state: 'run' });
      const url = '/api/proxy.json?t=' + proxyT + '&g=http://' + k + '/api/tools.json?t=' + encodeURIComponent(t) + '%26type=' + ttype;
      getText(url, { timeout })
        .then((text) => {
          let res;
          try { res = JSON.parse(text); } catch (e) { throw new HttpError(200, text); }
          return res;
        })
        .then((res) => {
          if (seq !== runSeq.current) return;
          if (res.status !== 'true') setRow(k, { state: 'bad', msg: res.error });
          else if (ttype === 'tcp' && res.error) setRow(k, { state: 'bad', msg: res.error, res });
          else setRow(k, { state: 'ok', res });
        })
        .catch((e) => {
          if (seq !== runSeq.current) return;
          const msg = e && e.name === 'AbortError' ? 'timeout' : (e && e.body) || (e && e.message) || '';
          setRow(k, { state: 'fail', msg });
        });
    });
  };

  const colCnt = meta.cols.length - 1;

  return (
    <>
      <Panel title="多节点网络检测" sub="从所有探测节点同时发起检测，对比各地连通性与质量">
        <ToolbarBox sx={{ flexWrap: 'nowrap', overflowX: 'auto', '@media (max-width:900px)': { p: '10px 12px', gap: 1, '& .tb-label': { display: 'none' } }, '@media (max-width:600px)': { flexWrap: 'wrap', overflowX: 'visible', '& .tb-group': { width: '100%', minWidth: 0 } } }}>
          <Box className="tb-group" sx={{ display: 'inline-flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', minWidth: 0 }}>
            <Box className="tb-label" component="span"><ToolbarLabel>类型</ToolbarLabel></Box>
            <TextField select size="small" value={type} onChange={(e) => changeType(e.target.value)} sx={{ width: 150, '@media (max-width:600px)': { width: '100%' } }}
              SelectProps={{ MenuProps: { disableScrollLock: true } }} inputProps={{ id: 'probe-type' }}>
              {Object.entries(TYPE_META).map(([v, m]) => <MenuItem key={v} value={v} sx={{ fontSize: 13.5 }}>{m.label}</MenuItem>)}
            </TextField>
          </Box>
          <Box className="tb-group" sx={{ display: 'inline-flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', minWidth: 200, flex: 1 }}>
            <Box className="tb-label" component="span"><ToolbarLabel>目标</ToolbarLabel></Box>
            <TextField size="small" value={target} placeholder={meta.ph} onChange={(e) => setTarget(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && !e.nativeEvent.isComposing) run(); }}
              inputProps={{ id: 'target' }} sx={{ minWidth: 180, flex: 1, '@media (max-width:600px)': { minWidth: 0 } }} />
            <Button variant="contained" onClick={run} id="check-btn">开始检测</Button>
          </Box>
        </ToolbarBox>
        <Typography sx={{ mb: 2, fontSize: 14 }}>{meta.hint}</Typography>

        <ResponsiveTable dense>
          <Table sx={{ '& td, & th': { lineHeight: 'normal' }, '& td': { verticalAlign: 'middle' }, '& tbody tr:last-child td': { borderBottom: 'none' } }}>
            <TableHead>
              <TableRow>
                <TableCell sx={{ width: 36, pr: '13px' }}>
                  <Checkbox size="small" checked={allChecked} onChange={(e) => toggleAll(e.target.checked)} sx={cbSx} inputProps={{ 'aria-label': '全选' }} />
                </TableCell>
                <TableCell>检测节点</TableCell>
                {meta.cols.map((c) => <TableCell key={c}>{c}</TableCell>)}
              </TableRow>
            </TableHead>
            <TableBody>
              {nodes.map((n) => {
                const k = keyOf(n);
                const row = rows[k];
                const res = row && row.res && row.state !== 'run' ? row.res : null;
                const cells = res ? resultCells(type, res, () => setMtr({ node: n.Name, hops: res.mtr || [], ip: res.ip })) : [];
                return (
                  <TableRow key={k} hover>
                    <TableCell sx={{ pr: '13px' }}>
                      <Checkbox size="small" checked={isChecked(k)} sx={cbSx}
                        onChange={(e) => setChecked((c) => ({ ...c, [k]: e.target.checked }))} />
                    </TableCell>
                    <TableCell>
                      <b>{n.Name}</b><br />
                      <Box component="span" sx={{ fontFamily: mono, fontSize: 11, color: palette.text3 }}>{n.Addr}</Box>
                    </TableCell>
                    {Array.from({ length: colCnt }, (_, i) => (
                      <TableCell key={i}>{cells[i] !== undefined ? cells[i] : '-'}</TableCell>
                    ))}
                    <TableCell><StatusCell row={row} /></TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </ResponsiveTable>
      </Panel>

      <Dialog open={!!mtr} onClose={() => setMtr(null)} fullWidth maxWidth={false} PaperProps={{ sx: { maxWidth: 1240 } }}>
        <DialogTitle sx={{ display: 'flex', alignItems: 'center', borderBottom: `1px solid ${palette.border}`, py: 2, px: 2.5 }}>
          <Box component="span" sx={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {mtr ? `MTR 路由追踪 - 从 ${mtr.node} 出发` : 'MTR 路由追踪'}
          </Box>
          <IconButton size="small" onClick={() => setMtr(null)} aria-label="关闭"><CloseIcon fontSize="small" /></IconButton>
        </DialogTitle>
        <DialogContent sx={{ p: 0, maxHeight: '70vh', overflowY: 'auto' }}>
          {mtr && <MtrTable hops={mtr.hops} targetIp={mtr.ip} />}
        </DialogContent>
      </Dialog>
    </>
  );
}
