import { useEffect, useState } from 'react';
import { Box, Table, TableBody, TableCell, TableHead, TableRow } from '@mui/material';
import { mono, palette } from '../../theme';
import { trim, validAddr } from './logic';
import { Btn, Chk, Field, Input, Modal, TextArea, flexRow, gridCols, plainSx, softBox, tableSx } from './ui';

const PH = '机房A,1.1.1.1,192.168.1.1\n机房B,1.1.1.2,192.168.1.2\n仅公网的机器C,1.1.1.3,\n仅专线的机器D,,192.168.1.4';

export default function PlaneWizard({ openSeq, cfg, onChanged, toast }) {
  const [open, setOpen] = useState(false);
  const [opt, setOpt] = useState({ enPub: true, namePub: '公网', enLine: true, nameLine: '专线', delay: '200', loss: '30' });
  const [machines, setMachines] = useState('');
  const [result, setResult] = useState(null);
  const [probes, setProbes] = useState([]);

  useEffect(() => {
    if (!openSeq) return;
    setMachines('');
    setResult(null);
    setProbes(Object.keys(cfg.Network).filter((a) => cfg.Network[a].Pingmesh)
      .map((a) => ({ addr: a, name: cfg.Network[a].Name, on: true, srcPub: '', srcLine: '' })));
    setOpen(true);
  }, [openSeq]); // eslint-disable-line react-hooks/exhaustive-deps

  const close = () => setOpen(false);
  const updProbe = (addr, patch) => setProbes((ps) => ps.map((p) => (p.addr === addr ? { ...p, ...patch } : p)));

  const save = () => {
    const planes = [];
    if (opt.enPub) planes.push({ name: trim(opt.namePub) || '公网', col: 1 });
    if (opt.enLine) planes.push({ name: trim(opt.nameLine) || '专线', col: 2 });
    if (planes.length === 0) { toast('请至少启用一个平面', 'err'); return; }
    const text = trim(machines);
    if (!text) { toast('请填写机器列表', 'err'); return; }
    const delay = opt.delay || 200;
    const loss = opt.loss || 30;
    const sel = probes.filter((p) => p.on).map((p) => ({ addr: p.addr, srcPub: trim(p.srcPub), srcLine: trim(p.srcLine) }));
    if (sel.length === 0) { toast('请至少勾选一个探测节点', 'err'); return; }

    const errors = [];
    let addedNodes = 0;
    let addedLinks = 0;
    text.split('\n').forEach((rawLine, i) => {
      const line = trim(rawLine);
      if (line === '') return;
      const parts = line.split(/[,，]/);
      const mname = trim(parts[0] || '');
      const ips = [null, trim(parts[1] || ''), trim(parts[2] || '')];
      if (!mname) { errors.push('第' + (i + 1) + '行: 缺少机器名'); return; }
      planes.forEach((pl) => {
        const addr = ips[pl.col];
        if (!addr) return;
        if (!validAddr(addr)) { errors.push('第' + (i + 1) + '行: ' + pl.name + ' 地址非法 ' + addr); return; }
        const nodeName = mname + '-' + pl.name;
        if (!cfg.Network[addr]) {
          cfg.Network[addr] = { Name: nodeName, Addr: addr, Group: pl.name, Pingmesh: false, Ping: [], Topology: [] };
          addedNodes++;
        } else {
          cfg.Network[addr].Group = cfg.Network[addr].Group || pl.name;
        }
        sel.forEach((pb) => {
          if (pb.addr === addr) return;
          const probe = cfg.Network[pb.addr];
          if (!probe || !probe.Pingmesh) return;
          probe.Ping = probe.Ping || [];
          probe.Topology = probe.Topology || [];
          const srcip = pl.col === 1 ? pb.srcPub : pb.srcLine;
          let exist = null;
          probe.Topology.forEach((t) => { if (t.Addr === addr) exist = t; });
          if (exist) {
            if (srcip) exist.Srcip = srcip;
            return;
          }
          probe.Ping.push(addr);
          probe.Topology.push({
            Addr: addr, Name: cfg.Network[addr].Name,
            Thdavgdelay: String(delay), Thdloss: String(loss),
            Thdjitter: '', Thdchecksec: '900', Thdoccnum: '3',
            Srcip: srcip, Pinterval: '', Pcount: '', Ptimeout: '', Psize: '',
          });
          addedLinks++;
        });
      });
    });
    const msg = '生成完成: 新增 ' + addedNodes + ' 个目标节点, ' + addedLinks + ' 条监测链路';
    onChanged();
    if (errors.length > 0) {
      setResult({ err: errors.join('；'), msg });
      if (addedNodes === 0 && addedLinks === 0) return;
      toast(msg + '（部分行有错误，详见弹窗）', 'err');
      return;
    }
    close();
    toast(msg + '，记得点击「保存配置」', 'ok');
  };

  const labelChk = { fontSize: 13, fontWeight: 600, color: palette.text2, mb: '6px', display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer' };
  return (
    <Modal open={open} onClose={close} title="双网口组网向导(公网 + 专线)" width={1420}
      actions={<><Btn onClick={close}>取消</Btn><Btn primary onClick={save}>生成组网配置</Btn></>}>
      <Box sx={{ ...softBox, mb: 2 }}>
        每台机器有公网、专线两个网口时：向导会为每台机器生成 <b>名称-公网</b> / <b>名称-专线</b> 两个目标节点（按平面分组），
        并为勾选的探测节点自动建立<b>同平面</b>监测链路（公网口→对端公网IP、专线口→对端专线IP），两条路径完全分开监控、互不串路。
      </Box>
      <Box sx={{ ...gridCols(2), mb: 1 }}>
        <Box sx={{ mb: 2 }}>
          <Box component="label" sx={labelChk}><Chk checked={opt.enPub} onChange={(v) => setOpt({ ...opt, enPub: v })} />启用平面 1（分组名）</Box>
          <Input value={opt.namePub} onChange={(v) => setOpt({ ...opt, namePub: v })} />
        </Box>
        <Box sx={{ mb: 2 }}>
          <Box component="label" sx={labelChk}><Chk checked={opt.enLine} onChange={(v) => setOpt({ ...opt, enLine: v })} />启用平面 2（分组名）</Box>
          <Input value={opt.nameLine} onChange={(v) => setOpt({ ...opt, nameLine: v })} />
        </Box>
      </Box>
      <Field label="哪些探测节点监测这批机器（每个平面填该探测节点对应网口的源IP；留空 = 走系统默认路由）">
        <Box sx={{ maxHeight: 200, overflowY: 'auto', border: `1px solid ${palette.border}`, borderRadius: '10px' }}>
          <Table sx={tableSx}>
            <TableHead><TableRow><TableCell sx={{ width: 40 }}>参与</TableCell><TableCell>探测节点</TableCell><TableCell>平面1 源IP（公网口）</TableCell><TableCell>平面2 源IP（专线口）</TableCell></TableRow></TableHead>
            <TableBody>
              {probes.length === 0 && <TableRow><TableCell colSpan={4} sx={{ color: palette.text3, p: '12px !important' }}>暂无探测节点</TableCell></TableRow>}
              {probes.map((p) => (
                <TableRow key={p.addr}>
                  <TableCell sx={{ textAlign: 'center' }}><Chk checked={p.on} onChange={(on) => updProbe(p.addr, { on })} /></TableCell>
                  <TableCell><b>{p.name}</b> <Box component="span" sx={{ color: palette.text3, fontFamily: mono, fontSize: 11 }}>{p.addr}</Box></TableCell>
                  <TableCell><Input dense isMono value={p.srcPub} placeholder="默认路由" onChange={(v) => updProbe(p.addr, { srcPub: v })} /></TableCell>
                  <TableCell><Input dense isMono value={p.srcLine} placeholder="默认路由" onChange={(v) => updProbe(p.addr, { srcLine: v })} /></TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Box>
      </Field>
      <Field label={<>机器列表（每行一台：<Box component="span" sx={{ fontFamily: mono }}>机器名,公网IP,专线IP</Box>，某项留空表示该机器没有该平面）</>}>
        <TextArea rows={7} value={machines} onChange={setMachines} placeholder={PH} />
      </Field>
      <Box sx={{ ...flexRow, flexWrap: 'wrap', fontSize: 14 }}>
        <Box component="span" sx={{ color: palette.text3, fontSize: 12.5 }}>告警阈值:</Box>
        延迟 <Input dense type="number" width={85} value={opt.delay} onChange={(v) => setOpt({ ...opt, delay: v })} /> ms
        丢包 <Input dense type="number" width={70} value={opt.loss} onChange={(v) => setOpt({ ...opt, loss: v })} /> %
        <Box component="span" sx={{ color: palette.text3, fontSize: 12 }}>（窗口 900 秒 / 次数 3，生成后可在监测规则里逐条调整）</Box>
      </Box>
      <Box sx={plainSx}>
        {result && <><Box component="span" sx={{ color: palette.red }}>{result.err}</Box><br />{result.msg}</>}
      </Box>
    </Modal>
  );
}
