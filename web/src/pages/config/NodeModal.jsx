import { useEffect, useRef, useState } from 'react';
import { Box, Link, Radio, Table, TableBody, TableCell, TableHead, TableRow } from '@mui/material';
import { Badge, Spinner } from '../../components/ui';
import { getJSON } from '../../api';
import { mono, palette } from '../../theme';
import { buildTargetRows, buildWatcherRows, groupNames, probeSummary, saveNode, trim } from './logic';
import { Btn, Chk, Field, Input, Modal, Spacer, flexRow, gridCols, hintSx, plainSx, tableSx } from './ui';

const TA_DEFAULT = { delay: '200', loss: '30', jitter: '', sec: '900', num: '3', src: '', pint: '', pcnt: '' };
const sectionLabel = { fontSize: 13, fontWeight: 600, color: palette.text2, m: 0 };
const cellIn = { '& .MuiOutlinedInput-input': { padding: '4px 8px' } };

function TypeOpt({ value, cur, onPick, title, desc }) {
  const on = cur === value;
  return (
    <Box component="label" onClick={() => onPick(value)} sx={{
      display: 'block', border: `1px solid ${on ? palette.primary : palette.border}`, borderRadius: '10px', p: '12px 14px',
      cursor: 'pointer', transition: 'all .15s', fontSize: 13.5, background: on ? palette.primarySoft : 'transparent',
      '&:hover': { borderColor: on ? palette.primary : palette.primary2 },
    }}>
      <Radio size="small" checked={on} value={value} name="n-type" disableRipple sx={{ p: 0, mr: 1, '& .MuiSvgIcon-root': { fontSize: 16 } }} />
      <b>{title}</b>
      <Box component="span" sx={{ ...hintSx, display: 'block', mt: '3px' }}>{desc}</Box>
    </Box>
  );
}

export default function NodeModal({ req, onClose, cfg, onChanged, toast, onGotoImport }) {
  const [st, setSt] = useState(null);
  const [ta, setTa] = useState(TA_DEFAULT);
  const [asnBox, setAsnBox] = useState(null);
  const asnSeq = useRef(0);

  const lookupAsn = (raw) => {
    setAsnBox(null);
    const addr = trim(raw || '');
    if (!addr || addr === '127.0.0.1') { asnSeq.current++; return; }
    const seq = ++asnSeq.current;
    setAsnBox({ loading: true });
    getJSON('/api/asn.json?ip=' + encodeURIComponent(addr))
      .then((res) => { if (seq === asnSeq.current) setAsnBox({ res, addr }); })
      .catch(() => { if (seq === asnSeq.current) setAsnBox(null); });
  };

  useEffect(() => {
    if (!req) return;
    const addr = req.addr;
    const n = addr ? cfg.Network[addr] : { Name: '', Addr: '', Group: '', Pingmesh: false, Ping: [], Topology: [] };
    setSt({
      seq: req.seq,
      editingAddr: addr,
      title: addr ? '编辑节点 - ' + n.Name : '添加节点',
      name: n.Name || '', addr: n.Addr || '', group: n.Group || '',
      selfNode: addr === cfg.Addr,
      showHint: !!addr && addr !== cfg.Addr,
      type: addr ? (n.Pingmesh ? 'probe' : 'target') : 'probe',
      groups: buildTargetRows(cfg, n),
      watchers: buildWatcherRows(cfg, n),
      groupList: groupNames(cfg),
      search: '',
    });
    lookupAsn(n.Addr);
  }, [req]); // eslint-disable-line react-hooks/exhaustive-deps

  const open = !!req && !!st && st.seq === req.seq;
  if (!st) return <Modal open={false} onClose={onClose} title="" />;

  const set = (patch) => setSt((s) => ({ ...s, ...patch }));
  const q = st.search.trim().toLowerCase();
  const vis = (r) => q === '' || r.s.indexOf(q) >= 0;
  const mapRows = (fn) => setSt((s) => ({ ...s, groups: s.groups.map((g) => ({ ...g, rows: g.rows.map((r) => fn(r, g) || r) })) }));
  const updRow = (addr, patch) => mapRows((r) => (r.addr === addr ? { ...r, ...patch } : null));
  const visOp = (fn) => mapRows((r) => (vis(r) ? { ...r, ...fn(r) } : null));

  const allRows = [];
  st.groups.forEach((g) => g.rows.forEach((r) => allRows.push(r)));
  const onCount = allRows.filter((r) => r.on).length;

  const onSearch = (v) => {
    const nq = v.trim().toLowerCase();
    setSt((s) => ({
      ...s, search: v,
      groups: s.groups.map((g) => ({ ...g, rows: g.rows.map((r) => (nq === '' || r.s.indexOf(nq) >= 0 || !r.open ? r : { ...r, open: false })) })),
    }));
  };

  const applyTa = () => {
    const j = ta.jitter.trim();
    const sip = ta.src.trim();
    const pint = ta.pint.trim();
    const pcnt = ta.pcnt.trim();
    mapRows((r) => {
      if (!r.on) return null;
      const p = { delay: ta.delay, loss: ta.loss, sec: ta.sec, num: ta.num };
      if (j !== '') p.jitter = j;
      if (sip !== '') p.src = sip;
      if (pint !== '') p.pint = pint;
      if (pcnt !== '') p.pcnt = pcnt;
      return { ...r, ...p };
    });
    toast('已套用阈值到 ' + onCount + ' 个目标', 'ok');
  };

  const save = () => {
    const err = saveNode(cfg, st.editingAddr, st, st.groups, st.watchers);
    if (err) { toast(err, 'err'); return; }
    onClose();
    onChanged();
    toast('已更新工作副本，记得点击「保存配置」', 'ok');
  };

  const g = cfg.Base || {};
  const asnRes = asnBox && asnBox.res;

  return (
    <Modal open={open} onClose={onClose} title={st.title} width={1420}
      actions={<><Btn onClick={onClose}>取消</Btn><Btn primary onClick={save}>确定</Btn></>}>
      <Box sx={{ ...gridCols(3), mb: 1 }}>
        <Field label="节点名称" sx={{ mb: 0 }}>
          <Input value={st.name} onChange={(v) => set({ name: v })} placeholder="如: 北京机房 / 核心网关" />
        </Field>
        <Field label="IP / 域名" sx={{ mb: 0 }}>
          <Input isMono value={st.addr} onChange={(v) => set({ addr: v })} placeholder="如: 10.0.0.1 或 gw.example.com"
            disabled={st.selfNode} title={st.selfNode ? '本机 IP 请在「基础设置」中修改' : ''} onBlur={(e) => lookupAsn(e.target.value)} />
          {st.showHint && <Box sx={hintSx}>修改地址后，其他节点对它的监测关系会自动跟随更新</Box>}
          <Box sx={{ ...hintSx, minHeight: 16 }}>
            {asnBox && asnBox.loading && <><Spinner size={11} /> 查询 ASN...</>}
            {asnRes && asnRes.status === 'true' && (
              <>🌐 <b>AS{asnRes.asn}</b> · {asnRes.holder} <Box component="span" sx={{ fontFamily: mono, fontSize: 11 }}>{asnRes.prefix}</Box>
                {asnRes.ip !== asnBox.addr && <Box component="span" sx={{ color: palette.text3 }}> (解析为 {asnRes.ip})</Box>}</>
            )}
            {asnRes && asnRes.status !== 'true' && <Box component="span" sx={{ color: palette.text3 }}>ASN: {asnRes.info || '未知'}</Box>}
          </Box>
        </Field>
        <Field label="分组（可选）" sx={{ mb: 0 }}>
          <Input value={st.group} onChange={(v) => set({ group: v })} placeholder="如: 华东 / 海外 / 核心网" inputProps={{ list: 'n-group-list' }} />
          <datalist id="n-group-list">{st.groupList.map((x) => <option key={x} value={x} />)}</datalist>
        </Field>
      </Box>

      <Field label="节点类型">
        <Box sx={{ ...gridCols(2), gap: '10px' }}>
          <TypeOpt value="probe" cur={st.type} onPick={(t) => set({ type: t })} title="探测节点" desc="部署了 PingMesh 程序，会主动 PING 监测目标，并出现在 Pingmesh 矩阵的「源」中" />
          <TypeOpt value="target" cur={st.type} onPick={(t) => set({ type: t })} title="被测目标" desc="仅作为被 PING 的对象（网关 / DNS / 服务器等），不需要部署任何程序" />
        </Box>
      </Field>

      <Box sx={{ display: st.type === 'probe' ? 'block' : 'none' }}>
        <Box sx={{ ...flexRow, mb: 1, flexWrap: 'wrap' }}>
          <Box component="label" sx={sectionLabel}>本节点监测哪些目标</Box>
          <Spacer />
          <Input dense width={200} value={st.search} onChange={onSearch} placeholder="搜索名称 / IP / 分组..." sx={{ '& .MuiOutlinedInput-input': { padding: '5px 10px' } }} />
          <Btn sm onClick={() => visOp(() => ({ on: true }))}>全选</Btn>
          <Btn sm onClick={() => visOp(() => ({ on: false }))}>全不选</Btn>
          <Btn sm onClick={() => visOp((r) => ({ on: !r.on }))}>反选</Btn>
          <Btn sm onClick={() => visOp((r) => ({ on: r.probe }))}>仅探测节点</Btn>
          <Btn sm onClick={() => visOp((r) => (r.mutual !== null ? { mutual: true } : {}))}>互PING全选</Btn>
        </Box>
        <Box sx={{ ...flexRow, mb: 1, flexWrap: 'wrap', background: palette.bg, borderRadius: '8px', p: '8px 12px' }}>
          <Box component="span" sx={{ color: palette.text3, fontSize: 12.5 }}>阈值批量套用:</Box>
          <Input dense type="number" width={90} value={ta.delay} onChange={(v) => setTa({ ...ta, delay: v })} title="延迟阈值(ms)" />
          <Box component="span" sx={{ color: palette.text3, fontSize: 11 }}>ms</Box>
          <Input dense type="number" width={75} value={ta.loss} onChange={(v) => setTa({ ...ta, loss: v })} title="丢包阈值(%)" />
          <Box component="span" sx={{ color: palette.text3, fontSize: 11 }}>%</Box>
          <Input dense type="number" width={75} value={ta.jitter} onChange={(v) => setTa({ ...ta, jitter: v })} placeholder="抖动" title="抖动阈值(ms), 留空不套用" />
          <Box component="span" sx={{ color: palette.text3, fontSize: 11 }}>ms抖</Box>
          <Input dense type="number" width={90} value={ta.sec} onChange={(v) => setTa({ ...ta, sec: v })} title="检测窗口(秒)" />
          <Box component="span" sx={{ color: palette.text3, fontSize: 11 }}>秒</Box>
          <Input dense type="number" width={70} value={ta.num} onChange={(v) => setTa({ ...ta, num: v })} title="触发次数" />
          <Box component="span" sx={{ color: palette.text3, fontSize: 11 }}>次</Box>
          <Input dense isMono width={120} value={ta.src} onChange={(v) => setTa({ ...ta, src: v })} placeholder="源IP" title="探测源IP(双网口分路), 留空不套用" />
          <Input dense type="number" width={70} value={ta.pint} onChange={(v) => setTa({ ...ta, pint: v })} placeholder="间隔" title="链路探测间隔(ms), 留空不套用" />
          <Input dense type="number" width={65} value={ta.pcnt} onChange={(v) => setTa({ ...ta, pcnt: v })} placeholder="包数" title="链路探测包数, 留空不套用" />
          <Btn sm primary onClick={applyTa}>套用到已勾选</Btn>
          <Box component="span" sx={{ color: palette.text3, fontSize: 12, ml: 'auto' }}>已勾选 {onCount} / {allRows.length}</Box>
        </Box>
        <Box sx={{ maxHeight: 360, overflowY: 'auto', border: `1px solid ${palette.border}`, borderRadius: '10px' }}>
          <Table sx={tableSx}>
            <TableHead>
              <TableRow>
                <TableCell sx={{ width: 64 }}>我监测它</TableCell><TableCell>目标节点</TableCell>
                <TableCell sx={{ width: 100 }} title="勾选后对方节点也会监测本节点, 形成互PING">互PING</TableCell>
                <TableCell>延迟(ms)</TableCell><TableCell>丢包率(%)</TableCell>
                <TableCell title="抖动阈值, 留空不检查; IPLC/IEPL 专线建议 5~20ms">抖动(ms)</TableCell>
                <TableCell title="判定告警时回看多久的数据(秒), 数据每分钟1个点, 需 ≥ 次数×60; 与探测频率无关">窗口(秒) ⓘ</TableCell>
                <TableCell title="窗口内出现几个异常分钟才告警">次数</TableCell>
                <TableCell title="双网口分路监控: 指定从本节点哪个网口IP发起探测(公网口/专线口), 留空走系统默认路由">源IP</TableCell>
                <TableCell title="按链路覆盖探测间隔/包数/超时/包大小, 未设置时使用基础设置中的全局默认">探测参数</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {st.groups.length === 0 && (
                <TableRow><TableCell colSpan={10} sx={{ color: palette.text3, p: '14px !important' }}>暂无其他节点，请先添加</TableCell></TableRow>
              )}
              {st.groups.map((gr) => {
                const visRows = gr.rows.filter(vis);
                const gOn = gr.rows.filter((r) => r.on).length;
                return [
                  <TableRow key={'g|' + gr.group} sx={{ background: '#f8fafc', display: visRows.length ? undefined : 'none' }}>
                    <TableCell sx={{ textAlign: 'center' }}>
                      <Chk checked={gOn > 0 && gOn === gr.rows.length} title="勾选整组"
                        onChange={(on) => mapRows((r) => (r.g === gr.group && vis(r) ? { ...r, on } : null))} />
                    </TableCell>
                    <TableCell colSpan={9}><b style={{ fontSize: 12 }}>{gr.group}</b> <Box component="span" sx={{ color: palette.text3, fontSize: 11 }}>{gr.rows.length} 个节点</Box></TableCell>
                  </TableRow>,
                  ...gr.rows.map((r) => {
                    const sum = probeSummary(r);
                    const shown = vis(r);
                    return [
                      <TableRow key={r.addr} sx={{ display: shown ? undefined : 'none' }}>
                        <TableCell sx={{ textAlign: 'center' }}><Chk checked={r.on} onChange={(on) => updRow(r.addr, { on })} /></TableCell>
                        <TableCell>
                          <b>{r.name}</b>{' '}
                          {r.probe && <><Badge tone="indigo" sx={{ fontSize: 10 }}>探测节点</Badge>{' '}</>}
                          <Box component="span" sx={{ color: palette.text3, fontFamily: mono, fontSize: 11 }}>{r.addr}</Box>
                        </TableCell>
                        <TableCell sx={{ textAlign: 'center' }}>
                          {r.mutual !== null
                            ? <Chk checked={r.mutual} onChange={(on) => updRow(r.addr, { mutual: on })} />
                            : <Box component="span" sx={{ color: palette.text3 }} title="对方是被测目标, 无法反向监测">—</Box>}
                        </TableCell>
                        <TableCell><Input dense type="number" value={r.delay} onChange={(v) => updRow(r.addr, { delay: v })} sx={cellIn} /></TableCell>
                        <TableCell><Input dense type="number" value={r.loss} onChange={(v) => updRow(r.addr, { loss: v })} sx={cellIn} /></TableCell>
                        <TableCell><Input dense type="number" value={r.jitter} placeholder="可空" onChange={(v) => updRow(r.addr, { jitter: v })} sx={cellIn} /></TableCell>
                        <TableCell><Input dense type="number" value={r.sec} inputProps={{ min: 60, step: 60 }} onChange={(v) => updRow(r.addr, { sec: v })} sx={cellIn} /></TableCell>
                        <TableCell><Input dense type="number" value={r.num} onChange={(v) => updRow(r.addr, { num: v })} sx={cellIn} /></TableCell>
                        <TableCell><Input dense isMono value={r.src} placeholder="默认" onChange={(v) => updRow(r.addr, { src: v })} sx={{ ...cellIn, minWidth: 110 }} /></TableCell>
                        <TableCell sx={{ whiteSpace: 'nowrap' }}>
                          <Link component="button" type="button" underline="none" onClick={() => updRow(r.addr, { open: !r.open })} sx={{ fontSize: 13.5, verticalAlign: 'baseline' }}>
                            {sum ? <><Badge tone="indigo" sx={{ fontSize: 10 }}>{sum}</Badge> ▾</> : <Box component="span" sx={{ color: palette.text3 }}>默认 ▾</Box>}
                          </Link>
                        </TableCell>
                      </TableRow>,
                      <TableRow key={'p|' + r.addr} sx={{ display: r.open ? undefined : 'none', background: '#f8fafc' }}>
                        <TableCell />
                        <TableCell colSpan={9} sx={{ p: '8px 12px !important' }}>
                          <Box sx={{ '& .MuiFormControl-root': { verticalAlign: 'middle' } }}>
                            <Box component="span" sx={{ color: palette.text3, fontSize: 12, mr: '6px' }}>本链路探测参数(留空=全局默认):</Box>
                            间隔 <Input dense type="number" width={90} value={r.pint} placeholder={String(g.Pinginterval || 2500)} onChange={(v) => updRow(r.addr, { pint: v })} sx={{ '& .MuiOutlinedInput-input': { padding: '3px 8px' } }} /> ms　
                            包数 <Input dense type="number" width={75} value={r.pcnt} placeholder={String(g.Pingcount || 20)} onChange={(v) => updRow(r.addr, { pcnt: v })} sx={{ '& .MuiOutlinedInput-input': { padding: '3px 8px' } }} />　
                            超时 <Input dense type="number" width={90} value={r.ptimeout} placeholder={String(g.Pingtimeout || 3000)} onChange={(v) => updRow(r.addr, { ptimeout: v })} sx={{ '& .MuiOutlinedInput-input': { padding: '3px 8px' } }} /> ms　
                            包大小 <Input dense type="number" width={80} value={r.psize} placeholder={String(g.Pingsize || 56)} onChange={(v) => updRow(r.addr, { psize: v })} sx={{ '& .MuiOutlinedInput-input': { padding: '3px 8px' } }} /> 字节
                          </Box>
                        </TableCell>
                      </TableRow>,
                    ];
                  }),
                ];
              })}
            </TableBody>
          </Table>
        </Box>
        <Box sx={plainSx}>
          按分组批量勾选：点击分组行的复选框。单向监测：勾选「我监测它」。互 PING：目标也是探测节点时再勾选「互PING」（对方用默认阈值）。<br />
          报警规则：「窗口」= 判定告警时回看多久的数据（与探测频率无关，探测频率在「探测参数」里调）。窗口内延迟/丢包/抖动<b>达到阈值</b>的分钟数<b>达到「次数」</b>即报警；每分钟 1 个数据点，故 窗口秒数 ≥ 次数×60；丢包阈值设 100 表示完全不通才报。
        </Box>
      </Box>

      <Box sx={{ display: st.type === 'probe' ? 'none' : 'block' }}>
        <Box sx={{ ...flexRow, mb: 1 }}>
          <Box component="label" sx={sectionLabel}>哪些探测节点监测它</Box>
          <Spacer />
          <Btn sm onClick={() => set({ watchers: st.watchers.map((w) => ({ ...w, on: true })) })}>全选</Btn>
          <Btn sm onClick={() => set({ watchers: st.watchers.map((w) => ({ ...w, on: false })) })}>清空</Btn>
        </Box>
        <Box sx={{ maxHeight: 320, overflowY: 'auto', border: `1px solid ${palette.border}`, borderRadius: '10px', py: '6px' }}>
          <Table sx={tableSx}>
            <TableBody>
              {st.watchers.length === 0 && <TableRow><TableCell sx={{ color: palette.text3, p: '14px !important' }}>暂无探测节点</TableCell></TableRow>}
              {st.watchers.map((w) => (
                <TableRow key={w.addr}>
                  <TableCell sx={{ width: 40, textAlign: 'center' }}>
                    <Chk checked={w.on} onChange={(on) => set({ watchers: st.watchers.map((x) => (x.addr === w.addr ? { ...x, on } : x)) })} />
                  </TableCell>
                  <TableCell>
                    <b>{w.name}</b> <Badge tone="indigo" sx={{ fontSize: 10 }}>{w.group}</Badge>{' '}
                    <Box component="span" sx={{ color: palette.text3, fontFamily: mono, fontSize: 11 }}>{w.addr}</Box>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Box>
        <Box sx={plainSx}>
          在这里直接选择由哪些探测节点来监测本目标（新勾选的使用默认阈值 200ms/30%），不必再逐个编辑探测节点。
          需要一次添加多个目标？<Link component="button" type="button" underline="none" onClick={onGotoImport} sx={{ fontSize: 12, verticalAlign: 'baseline' }}>批量导入 CSV / JSON →</Link>
        </Box>
      </Box>
    </Modal>
  );
}
