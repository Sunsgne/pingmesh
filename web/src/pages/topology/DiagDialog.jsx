import { useEffect, useState } from 'react';
import { Box, Dialog, DialogContent, DialogTitle, IconButton } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { Badge, EmptyState, Spinner } from '../../components/ui';
import { getJSON, nestedQs, proxy } from '../../api';
import { palette } from '../../theme';

const muted = { color: palette.text3 };

const lossTxt = (v) => Number(v).toFixed(2) + '%';
const msTxt = (v) => Number(v).toFixed(1) + 'ms';

// 拓扑页按所选时间窗口着色: 这里给出该窗口的判定和超标采样点, 与连线颜色对应
function RangeSection({ rng, rule }) {
  const res = rng.result || {};
  const ev = (res.events || []).slice().reverse();
  const over = (b) => {
    const parts = [];
    if (Number(b.delay) >= Number(rule.Thdavgdelay)) parts.push('延迟 ' + msTxt(b.delay));
    if (Number(b.loss) >= Number(rule.Thdloss)) parts.push('丢包 ' + lossTxt(b.loss));
    if (rule.Thdjitter && Number(b.jitter) >= Number(rule.Thdjitter)) parts.push('抖动 ' + msTxt(b.jitter));
    return parts.join(' · ');
  };
  return (
    <Box sx={{ mb: 2.5, p: 1.5, borderRadius: 1.5, border: `1px solid ${res.alerted ? '#fecdd3' : palette.border}`, background: res.alerted ? '#fff1f2' : '#f8fafc' }}>
      <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 1, mb: 0.75 }}>
        <b>所选时间窗口</b>
        <Box component="span" sx={muted}>{rng.start} ~ {rng.end}</Box>
        {res.alerted
          ? <Badge tone="red" dot>期间触发过告警</Badge>
          : <Badge tone="green" dot>期间未触发</Badge>}
      </Box>
      <Box sx={{ fontSize: 13 }}>
        {res.alerted
          ? <>首次 {res.first_hit}，最近 {res.last_hit}；窗口内共 {res.bad} 个超标采样点。连线标红即由此而来。</>
          : <>窗口内共 {res.bad} 个超标采样点，未在任何 {rule.Thdchecksec} 秒内累计到触发次数 {rule.Thdoccnum}。</>}
      </Box>
      {ev.length > 0 && (
        <Box component="table" sx={{ width: '100%', mt: 1, borderCollapse: 'collapse', fontSize: 12.5, '& td': { py: '4px', pr: 2, borderTop: '1px solid #f1f5f9' } }}>
          <tbody>
            {ev.map((b) => (
              <tr key={b.time}>
                <Box component="td" sx={{ ...muted, whiteSpace: 'nowrap' }}>{b.time}</Box>
                <td>{over(b)}</td>
              </tr>
            ))}
          </tbody>
        </Box>
      )}
      {res.truncated && <Box sx={{ ...muted, fontSize: 12, mt: 0.5 }}>仅显示最近 {ev.length} 个</Box>}
    </Box>
  );
}

function DiagBody({ d }) {
  const r = d.rule || {};
  const rows = [
    ['规则阈值', '延迟 ≥ ' + r.Thdavgdelay + 'ms 或 丢包率 ≥ ' + r.Thdloss + '%' + (r.Thdjitter ? ' 或 抖动 ≥ ' + r.Thdjitter + 'ms' : '')],
    ['检测窗口', d.window_sec + ' 秒(最多容纳 ' + d.capacity + ' 个数据点) · 触发次数 ' + d.occ],
    ['窗口内数据点', d.total + ' 个'],
    ['异常采样点数', (
      <>
        <b style={{ color: d.bad >= d.occ ? palette.red : palette.green }}>{d.bad}</b>
        {' / 触发线 ' + d.occ + '　'}
        <Box component="span" sx={muted}>
          {'(延迟超标 ' + d.bad_delay + ' · 丢包率超标 ' + d.bad_loss + (r.Thdjitter ? ' · 抖动超标 ' + d.bad_jitter : '') + ')'}
        </Box>
      </>
    )],
  ];
  if (d.muted) {
    rows.push(['屏蔽状态', (
      <>
        <Badge tone="yellow">屏蔽中</Badge>
        {' 至 ' + d.mute_until + (d.mute_reason ? ' · ' + d.mute_reason : '') + '(不发通知)'}
      </>
    )]);
  }
  return (
    <>
      {d.range && <RangeSection rng={d.range} rule={r} />}
      <Box sx={{ mb: 2, display: 'flex', alignItems: 'center', gap: 1 }}>
        {d.range && <b>当前（最近 {d.window_sec} 秒）</b>}
        {d.verdict === 'alerting'
          ? <Badge tone="red" dot>判定: 告警中</Badge>
          : <Badge tone="green" dot>判定: 正常</Badge>}
      </Box>
      <Box component="table" sx={{
        width: '100%', borderCollapse: 'collapse', fontSize: 13.5, lineHeight: 'normal',
        '& td': { p: '11px 16px', borderBottom: '1px solid #f1f5f9', verticalAlign: 'middle' },
        '& tr:hover': { background: '#f8fafc' },
        '& tr:last-child td': { borderBottom: 'none' },
      }}>
        <tbody>
          {rows.map(([k, v]) => (
            <tr key={k}>
              <Box component="td" sx={{ ...muted, whiteSpace: 'nowrap', width: 120 }}>{k}</Box>
              <td>{v}</td>
            </tr>
          ))}
        </tbody>
      </Box>
      {d.hint && <Box sx={{ mt: 1, color: palette.red }}>⚠ {d.hint}</Box>}
      <Box sx={{ mt: 1 }}>说明: 每 10 秒产生一个数据点; 窗口内异常采样点数达到「触发次数」即判定告警。</Box>
    </>
  );
}

/** 告警判定诊断: diag = { from, to, fromName, toName } 打开, null 关闭 */
export default function DiagDialog({ diag, range, selfAddr, port, onClose }) {
  const [res, setRes] = useState(null);

  useEffect(() => {
    if (!diag) return undefined;
    let alive = true;
    setRes({ loading: true });
    let qs = '?target=' + encodeURIComponent(diag.to);
    if (range && range.start && range.end) qs += '&start=' + encodeURIComponent(range.start) + '&end=' + encodeURIComponent(range.end);
    const url = diag.from === selfAddr ? '/api/alertdiag.json' + qs : proxy(diag.from, port, '/api/alertdiag.json' + nestedQs('x', qs));
    getJSON(url)
      .then((d) => {
        if (!alive) return;
        if (d.status !== 'true') setRes({ msg: d.info || '查询失败' });
        else setRes({ data: d });
      })
      .catch(() => alive && setRes({ msg: '源节点不可达' }));
    return () => { alive = false; };
  }, [diag, selfAddr, port]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <Dialog open={!!diag} onClose={onClose} fullWidth maxWidth={false} PaperProps={{ sx: { maxWidth: 580 } }}>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', borderBottom: `1px solid ${palette.border}`, py: 2, px: 2.5 }}>
        <Box component="span" sx={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {diag ? '告警判定诊断: ' + diag.fromName + ' → ' + diag.toName : '告警判定诊断'}
        </Box>
        <IconButton size="small" onClick={onClose} aria-label="关闭"><CloseIcon fontSize="small" /></IconButton>
      </DialogTitle>
      <DialogContent sx={{ p: { xs: '14px !important', sm: '20px !important' }, fontSize: 14 }}>
        {res && res.loading && <EmptyState sx={{ fontSize: 14 }}><Spinner /> 正在向源节点查询...</EmptyState>}
        {res && res.msg && <EmptyState sx={{ fontSize: 14 }}>{res.msg}</EmptyState>}
        {res && res.data && <DiagBody d={res.data} />}
      </DialogContent>
    </Dialog>
  );
}
