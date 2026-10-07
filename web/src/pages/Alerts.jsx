import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, Link, MenuItem,
  Table, TableBody, TableCell, TableHead, TableRow, TextField, ToggleButton, ToggleButtonGroup, Typography,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { Badge, EmptyState, Panel, Spinner, ToolbarBox, ToolbarLabel, ResponsiveTable } from '../components/ui';
import MtrTable from '../components/MtrTable';
import { useToast } from '../components/Feedback';
import { dlocal, fmtBeijing, getJSON, nestedQs, nodeBase } from '../api';
import { mono, palette } from '../theme';

const RANGES = [
  { v: 'today', label: '今天' }, { v: '24h', label: '近24小时' }, { v: '7d', label: '近7天' }, { v: 'custom', label: '自定义' },
];
const MUTE_OPTIONS = [
  { v: '60', label: '1 小时' }, { v: '360', label: '6 小时' }, { v: '1440', label: '24 小时' },
  { v: '4320', label: '3 天' }, { v: '10080', label: '7 天' },
];
const TYPE_TAGS = {
  delay: ['yellow', '延迟'], loss: ['red', '丢包率'], jitter: ['indigo', '抖动'], quality: ['gray', '质量异常'],
};

const fmt = (d) => fmtBeijing(d, { seconds: true });
function rangeWindow(range, crStart, crEnd) {
  const now = new Date();
  if (range === 'today') return [fmt(now).substring(0, 10) + ' 00:00:00', fmt(now)];
  if (range === '24h') return [fmt(new Date(now.getTime() - 24 * 3600e3)), fmt(now)];
  if (range === '7d') return [fmt(new Date(now.getTime() - 7 * 24 * 3600e3)), fmt(now)];
  return [crStart.replace('T', ' ') + ':00', crEnd.replace('T', ' ') + ':59'];
}

// 告警数据存储在各源节点上: 只取配置了监测拓扑的节点
function sourceAgents(cfg) {
  return Object.values(cfg.Network || {}).filter((n) => (n.Topology || []).length > 0);
}

function typeLabel(t) {
  if (!t) return '未知';
  return String(t).split('+').map((p) => (TYPE_TAGS[p] ? TYPE_TAGS[p][1] : p)).join('+');
}
function AlertTypes({ type }) {
  if (!type) return <Badge tone="gray">未知</Badge>;
  return String(type).split('+').map((p, i) => {
    const [tone, label] = TYPE_TAGS[p] || ['gray', p];
    return <Badge key={i} tone={tone} sx={{ m: '0 4px 2px 0' }}>{label}</Badge>;
  });
}
function reasonText(r) {
  let reason = r.Reason || '';
  if (!reason && (r.AvgDelay || r.Loss || r.Jitter)) {
    reason = '延迟 ' + Number(r.AvgDelay || 0).toFixed(1) + 'ms / 丢包率 ' + Number(r.Loss || 0).toFixed(1) + '% / 抖动 ' + Number(r.Jitter || 0).toFixed(1) + 'ms';
  }
  return reason;
}
function parseHops(s) {
  try { return JSON.parse(s); } catch (e) { return null; }
}

const muted = { color: palette.text3 };
const tableSx = {
  '& tbody tr:last-child td': { borderBottom: 'none' },
  '& td': { verticalAlign: 'middle', lineHeight: 'normal' },
  '& th': { textTransform: 'uppercase' },
};
const btnSm = {
  px: '10px', py: '5px', fontSize: 12.5, lineHeight: 'normal', borderRadius: '7px', minWidth: 0,
  borderColor: palette.border, color: palette.text2, bgcolor: '#fff',
};
const textareaSx = { '& textarea': { fontFamily: mono, fontSize: 12.5, lineHeight: 1.6, height: '38px !important', resize: 'vertical' }, '& .MuiOutlinedInput-root': { p: '9px 12px' } };

function FieldLabel({ children }) {
  return <Typography component="label" sx={{ display: 'block', fontSize: 13, fontWeight: 600, color: palette.text2, mb: '6px' }}>{children}</Typography>;
}

function SpDialog({ open, onClose, title, children, actions, wide = false, bodySx }) {
  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth={false} PaperProps={{ sx: { maxWidth: wide ? 1240 : 580, m: { xs: '3vh 10px', md: '6vh 16px' } } }}>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', borderBottom: `1px solid ${palette.border}`, py: 2, px: 2.5 }}>
        <Box component="span" sx={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{title}</Box>
        <IconButton size="small" onClick={onClose} aria-label="关闭"><CloseIcon fontSize="small" /></IconButton>
      </DialogTitle>
      <DialogContent sx={{ p: { xs: '14px !important', md: '20px !important' }, ...bodySx }}>{children}</DialogContent>
      {actions && <DialogActions sx={{ px: 2.5, py: 1.75, gap: 1.25, borderTop: `1px solid ${palette.border}`, '& > :not(style) ~ :not(style)': { ml: 0 } }}>{actions}</DialogActions>}
    </Dialog>
  );
}

export default function Alerts({ user, config: cfg }) {
  const toast = useToast();
  const timeout = ((cfg.Base && cfg.Base.Timeout) || 5) * 1000;
  const [range, setRange] = useState('today');
  const [crStart, setCrStart] = useState('');
  const [crEnd, setCrEnd] = useState('');
  const [rows, setRows] = useState([]);
  const [count, setCount] = useState(null);
  const [loading, setLoading] = useState(false);
  const [query, setQuery] = useState('');
  const [mutes, setMutes] = useState([]);
  const [mtr, setMtr] = useState(null);
  const [ackRow, setAckRow] = useState(null);
  const [ackReason, setAckReason] = useState('');
  const [muteRow, setMuteRow] = useState(null);
  const [muteReason, setMuteReason] = useState('');
  const [muteMinutes, setMuteMinutes] = useState('1440');
  const alertSeq = useRef(0);
  const muteSeq = useRef(0);
  const crRef = useRef({ crStart, crEnd });
  crRef.current = { crStart, crEnd };

  const loadMutes = useCallback(() => {
    const seq = ++muteSeq.current;
    const agents = sourceAgents(cfg);
    Promise.all(agents.map((n) => {
      const base = nodeBase(cfg, n.Addr);
      return getJSON(base + '/api/mute.json' + nestedQs(base, '?action=list'), { timeout })
        .then((res) => (res.mutes || []).map((m) => ({ ...m, _from: res.from, _fromname: res.fromname })))
        .catch(() => []);
    })).then((lists) => {
      if (seq === muteSeq.current) setMutes(lists.flat());
    });
  }, [cfg, timeout]);

  const getdata = useCallback((rg) => {
    const seq = ++alertSeq.current;
    const agents = sourceAgents(cfg);
    setRows([]);
    setLoading(true);
    const win = rangeWindow(rg, crRef.current.crStart, crRef.current.crEnd);
    const qs = '?start=' + encodeURIComponent(win[0]) + '&end=' + encodeURIComponent(win[1]);
    Promise.all(agents.map((n) => {
      const base = nodeBase(cfg, n.Addr);
      return getJSON(base + '/api/alert.json' + nestedQs(base, qs), { timeout })
        .then((result) => (result && result[1]) || [])
        .catch(() => []);
    })).then((lists) => {
      if (seq !== alertSeq.current) return;
      const all = lists.flat();
      all.sort((a, b) => (a.Logtime < b.Logtime ? 1 : -1));
      setRows(all);
      setCount(all.length);
      setLoading(false);
    });
    loadMutes();
  }, [cfg, timeout, loadMutes]);

  useEffect(() => { getdata('today'); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const pickRange = (v) => {
    setRange(v);
    if (v === 'custom') {
      if (!crStart) {
        setCrStart(dlocal(new Date(Date.now() - 24 * 3600e3)));
        setCrEnd(dlocal(new Date()));
      }
      return;
    }
    getdata(v);
  };
  const applyCustom = () => {
    if (!crStart || !crEnd || crStart >= crEnd) { toast('请选择正确的起止时间', 'err'); return; }
    getdata('custom');
  };

  const view = useMemo(() => rows.map((r) => {
    const reason = reasonText(r);
    const blob = [r.Fromname, r.Fromip, r.Targetname, r.Targetip, r.Ackreason, r.AlertType, typeLabel(r.AlertType), reason, r.Tag || '']
      .map((x) => (x == null ? '' : x)).join(' ').toLowerCase();
    return { r, reason, blob };
  }), [rows]);
  const q = query.trim().toLowerCase();

  const openAck = (r) => { setAckRow(r); setAckReason(''); };
  const saveAck = () => {
    const reason = ackReason.trim();
    if (!reason) { toast('请填写处理说明', 'err'); return; }
    const base = nodeBase(cfg, ackRow.Fromip);
    const qs = '?id=' + ackRow.Id + '&reason=' + encodeURIComponent(reason) + '&by=' + encodeURIComponent(user.username);
    getJSON(base + '/api/alertack.json' + nestedQs(base, qs)).then((res) => {
      if (res.status === 'true') {
        toast('已确认', 'ok');
        setAckRow(null);
        getdata(range);
      } else {
        toast(res.info || '确认失败', 'err');
      }
    }).catch(() => toast('请求失败', 'err'));
  };

  const openMute = (r) => { setMuteRow(r); setMuteReason(''); };
  const saveMute = () => {
    const reason = muteReason.trim();
    if (!reason) { toast('请填写屏蔽原因', 'err'); return; }
    const base = nodeBase(cfg, muteRow.Fromip);
    const qs = '?action=add&target=' + encodeURIComponent(muteRow.Targetip) +
      '&minutes=' + muteMinutes +
      '&reason=' + encodeURIComponent(reason) + '&by=' + encodeURIComponent(user.username);
    getJSON(base + '/api/mute.json' + nestedQs(base, qs)).then((res) => {
      if (res.status === 'true') {
        toast('已屏蔽至 ' + res.muteduntil, 'ok');
        setMuteRow(null);
        loadMutes();
      } else {
        toast(res.info || '屏蔽失败', 'err');
      }
    }).catch(() => toast('请求失败', 'err'));
  };

  const unmute = (m) => {
    const base = nodeBase(cfg, m._from);
    const qs = '?action=del&target=' + encodeURIComponent(m.target);
    getJSON(base + '/api/mute.json' + nestedQs(base, qs)).then((res) => {
      if (res.status === 'true') { toast('已解除屏蔽', 'ok'); loadMutes(); } else toast(res.info || '操作失败', 'err');
    }).catch(() => {});
  };

  return (
    <>
      {mutes.length > 0 && (
        <Panel flat sx={{ mb: 2 }} title={<>屏蔽中的目标 <Badge tone="yellow">{mutes.length} 个</Badge></>}
          sub="屏蔽期间仍记录告警，但不发送任何通知；到期自动恢复">
          <ResponsiveTable sx={{ px: { xs: '10px', sm: 0 } }}>
            <Table sx={tableSx}>
              <TableHead>
                <TableRow>
                  <TableCell>源节点</TableCell><TableCell>目标</TableCell><TableCell>屏蔽至</TableCell>
                  <TableCell>原因</TableCell><TableCell>操作人</TableCell><TableCell sx={{ width: 90 }}>操作</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {mutes.map((m, i) => (
                  <TableRow hover key={`${m._from}|${m.target}|${i}`}>
                    <TableCell>{m._fromname || m._from}</TableCell>
                    <TableCell><b>{m.targetname}</b> <Box component="span" sx={{ ...muted, fontFamily: mono, fontSize: 11 }}>{m.target}</Box></TableCell>
                    <TableCell sx={{ whiteSpace: 'nowrap' }}>{m.muteduntil}</TableCell>
                    <TableCell>{m.reason || '-'}</TableCell>
                    <TableCell>{m.createdby || '-'}</TableCell>
                    <TableCell>
                      <Button variant="outlined" size="small" onClick={() => unmute(m)}
                        sx={{ ...btnSm, color: palette.red, borderColor: '#fecaca', '&:hover': { bgcolor: palette.redSoft, color: palette.red, borderColor: '#fecaca' } }}>解除</Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </ResponsiveTable>
        </Panel>
      )}

      <Panel
        title={<>报警记录 {count > 0 && <Badge tone="red">{count} 条</Badge>}</>}
        sub="确认与屏蔽需管理员权限"
        actions={loading ? <Spinner /> : null}
        bodySx={{ pt: '14px', pb: { xs: '14px', md: 2.25 }, px: { xs: '14px', md: 2.25 } }}
      >
        <ToolbarBox sx={{ flexWrap: 'nowrap', overflowX: 'auto', '@media (max-width:600px)': { flexWrap: 'wrap', overflowX: 'visible', '& > *': { width: '100%' } } }}>
          <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', minWidth: 0 }}>
            <Box sx={{ display: { xs: 'none', md: 'inline' } }}><ToolbarLabel>范围</ToolbarLabel></Box>
            <ToggleButtonGroup exclusive size="small" value={range} sx={{ flexShrink: 0, flexWrap: 'nowrap' }}>
              {RANGES.map((o) => <ToggleButton key={o.v} value={o.v} onClick={() => pickRange(o.v)} sx={{ py: '6px', lineHeight: 'normal' }}>{o.label}</ToggleButton>)}
            </ToggleButtonGroup>
            {range === 'custom' && (
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25 }}>
                <TextField size="small" type="datetime-local" value={crStart} onChange={(e) => setCrStart(e.target.value)}
                  sx={{ width: 185, '& input': { p: '6px 8px' } }} />
                <Box component="span" sx={muted}>~</Box>
                <TextField size="small" type="datetime-local" value={crEnd} onChange={(e) => setCrEnd(e.target.value)}
                  sx={{ width: 185, '& input': { p: '6px 8px' } }} />
                <Button variant="contained" size="small" onClick={applyCustom} sx={{ ...btnSm, color: '#fff', flexShrink: 0 }}>查询</Button>
              </Box>
            )}
          </Box>
          <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
            <Box sx={{ display: { xs: 'none', md: 'inline' } }}><ToolbarLabel>筛选</ToolbarLabel></Box>
            <TextField size="small" value={query} placeholder="节点 / IP / 类型 / Tag..." onChange={(e) => setQuery(e.target.value)}
              sx={{ width: 'min(230px,100%)', '@media (max-width:600px)': { width: '100%' } }} />
          </Box>
        </ToolbarBox>

        <ResponsiveTable sx={{ mx: { xs: '-14px', md: '-18px' } }}>
          <Table sx={tableSx}>
            <TableHead>
              <TableRow>
                <TableCell>报警时间</TableCell><TableCell>链路</TableCell><TableCell>类型 / 原因</TableCell><TableCell>Tag</TableCell>
                <TableCell>状态</TableCell><TableCell sx={{ width: 70 }}>MTR</TableCell><TableCell sx={{ width: 180 }}>操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {view.map(({ r, reason, blob }, i) => {
                if (q !== '' && blob.indexOf(q) < 0) return null;
                const occur = parseInt(r.Occur, 10) || 0;
                return (
                  <TableRow hover key={`${r.Fromip}|${r.Id}|${i}`}>
                    <TableCell sx={{ whiteSpace: 'nowrap' }}>{r.Logtime}</TableCell>
                    <TableCell>
                      <b>{r.Fromname}</b> <Box component="span" sx={muted}>&rarr;</Box> <b>{r.Targetname}</b>
                      <Box sx={{ ...muted, fontFamily: mono, fontSize: 11 }}>{r.Fromip} &rarr; {r.Targetip}</Box>
                    </TableCell>
                    <TableCell sx={{ minWidth: 160 }}>
                      <AlertTypes type={r.AlertType} />
                      {reason && <Box sx={{ ...muted, fontSize: 12, mt: '4px', maxWidth: 320, lineHeight: 1.35 }}>{reason}</Box>}
                    </TableCell>
                    <TableCell sx={{ maxWidth: 220 }}>
                      {r.Tag ? (
                        <>
                          <Box component="span" title="相同链路+相同类型的故障共用此 Tag" sx={{
                            display: 'inline-block', maxWidth: '100%', p: '3px 8px', borderRadius: '6px', bgcolor: '#f1f5f9', color: '#334155',
                            fontFamily: mono, fontSize: 11.5, lineHeight: 1.35, wordBreak: 'break-all', border: `1px solid ${palette.border}`,
                          }}>{r.Tag}</Box>
                          {occur > 1 && <Box sx={{ ...muted, fontSize: 11, mt: '3px' }}>复发 <b>{occur}</b> 次</Box>}
                        </>
                      ) : <Box component="span" sx={muted}>-</Box>}
                    </TableCell>
                    <TableCell>
                      {r.Ack ? (
                        <>
                          <Badge tone="green" title={(r.Ackby || '') + ' ' + (r.Acktime || '')}>已确认</Badge>
                          {r.Ackreason && <Box sx={{ ...muted, fontSize: 11, maxWidth: 260 }}>{r.Ackreason}</Box>}
                        </>
                      ) : <Badge tone="red" dot>未确认</Badge>}
                    </TableCell>
                    <TableCell>
                      <Link component="button" underline="none" onClick={() => setMtr({ hops: parseHops(r.Tracert), targetIp: r.Targetip })}
                        sx={{ fontSize: 13.5, color: palette.primary, verticalAlign: 'baseline', '&:hover': { color: palette.primary2 } }}>查看</Link>
                    </TableCell>
                    <TableCell sx={{ whiteSpace: 'nowrap' }}>
                      {!r.Ack && <><Button variant="outlined" size="small" sx={btnSm} onClick={() => openAck(r)}>确认</Button>{' '}</>}
                      <Button variant="outlined" size="small" sx={btnSm} onClick={() => openMute(r)}>屏蔽</Button>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
          {count === 0 && (
            <EmptyState>
              <Box sx={{ fontSize: 28, opacity: 0.5, mb: 1.25 }}>○</Box>
              该时间范围内没有报警记录
            </EmptyState>
          )}
        </ResponsiveTable>
      </Panel>

      <SpDialog open={!!mtr} onClose={() => setMtr(null)} title="MTR 路由追踪快照" wide
        bodySx={{ p: '0 !important', maxHeight: '70vh', overflowY: 'auto' }}>
        {mtr && <MtrTable hops={mtr.hops} targetIp={mtr.targetIp} />}
      </SpDialog>

      <SpDialog open={!!ackRow} onClose={() => setAckRow(null)} title="确认告警"
        actions={<>
          <Button variant="outlined" onClick={() => setAckRow(null)}>取消</Button>
          <Button variant="contained" onClick={saveAck}>确认</Button>
        </>}>
        {ackRow && (
          <Typography sx={{ ...muted, fontSize: 13, mb: 1 }}>
            链路: <b>{ackRow.Fromname} → {ackRow.Targetname}</b>　时间: {ackRow.Logtime}
          </Typography>
        )}
        <Box sx={{ mb: 2 }}>
          <FieldLabel>处理说明 / 原因</FieldLabel>
          <TextField fullWidth multiline rows={3} autoFocus value={ackReason} onChange={(e) => setAckReason(e.target.value)}
            placeholder="如: 机房线路割接, 已联系运营商, 预计 2 小时恢复" sx={textareaSx} />
        </Box>
      </SpDialog>

      <SpDialog open={!!muteRow} onClose={() => setMuteRow(null)} title="屏蔽告警"
        actions={<>
          <Button variant="outlined" onClick={() => setMuteRow(null)}>取消</Button>
          <Button variant="contained" onClick={saveMute}>屏蔽</Button>
        </>}>
        {muteRow && (
          <Typography sx={{ ...muted, fontSize: 13, mb: 1 }}>
            屏蔽目标: <b>{muteRow.Targetname}</b> <Box component="span" sx={{ fontFamily: mono, fontSize: 12.5 }}>{muteRow.Targetip}</Box>　(源节点 {muteRow.Fromname} 上不再通知)
          </Typography>
        )}
        <Box sx={{ mb: 2 }}>
          <FieldLabel>暂停通知时长</FieldLabel>
          <TextField select fullWidth value={muteMinutes} onChange={(e) => setMuteMinutes(e.target.value)}>
            {MUTE_OPTIONS.map((o) => <MenuItem key={o.v} value={o.v}>{o.label}</MenuItem>)}
          </TextField>
        </Box>
        <Box sx={{ mb: 2 }}>
          <FieldLabel>原因（必填，便于团队知晓）</FieldLabel>
          <TextField fullWidth multiline rows={3} autoFocus value={muteReason} onChange={(e) => setMuteReason(e.target.value)}
            placeholder="如: 已知故障, 等待硬件更换" sx={textareaSx} />
        </Box>
        <Typography sx={{ fontSize: 14 }}>屏蔽期间该目标的告警与恢复均不再通知（仍会记录），到期自动恢复通知。</Typography>
      </SpDialog>
    </>
  );
}
