import { useEffect, useState } from 'react';
import { Box, Table, TableBody, TableCell, TableRow } from '@mui/material';
import { Badge } from '../../components/ui';
import { mono, palette } from '../../theme';
import { groupOf, topoEntry, trim, validAddr } from './logic';
import { Btn, Chk, Modal, Spacer, TextArea, flexRow, plainSx, labelSx, tableSx } from './ui';

const PH = '上海网关,10.1.1.1,华东\n广州DNS,114.114.114.114,华南\n美西节点,us-west.example.com,海外';

export default function NodeImport({ openSeq, cfg, onChanged, toast }) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState('');
  const [result, setResult] = useState(null);
  const [watchers, setWatchers] = useState([]);

  useEffect(() => {
    if (!openSeq) return;
    setText('');
    setResult(null);
    setWatchers(Object.keys(cfg.Network).filter((a) => cfg.Network[a].Pingmesh)
      .map((a) => ({ addr: a, name: cfg.Network[a].Name, group: groupOf(cfg.Network[a]), on: true })));
    setOpen(true);
  }, [openSeq]); // eslint-disable-line react-hooks/exhaustive-deps

  const close = () => setOpen(false);

  const save = () => {
    const t = trim(text);
    if (!t) { toast('请粘贴导入内容', 'err'); return; }
    const items = [];
    const errors = [];
    if (t.charAt(0) === '[' || t.charAt(0) === '{') {
      try {
        let arr = JSON.parse(t);
        if (!Array.isArray(arr)) arr = [arr];
        arr.forEach((o) => {
          items.push({ name: trim(o.name || o.Name || ''), addr: trim(o.addr || o.Addr || ''), group: trim(o.group || o.Group || '') });
        });
      } catch (e) { setResult({ err: 'JSON 解析失败: ' + e.message }); return; }
    } else {
      t.split('\n').forEach((raw, i) => {
        const line = trim(raw);
        if (line === '') return;
        const parts = line.split(/[,，]/);
        if (parts.length < 2) { errors.push('第' + (i + 1) + '行: 至少需要 名称,地址'); return; }
        items.push({ name: trim(parts[0]), addr: trim(parts[1]), group: trim(parts[2] || '') });
      });
    }
    const ws = watchers.filter((w) => w.on).map((w) => w.addr);
    let added = 0;
    let skipped = 0;
    items.forEach((it, i) => {
      if (!it.name || !it.addr) { errors.push('第' + (i + 1) + '条: 名称或地址为空'); return; }
      if (!validAddr(it.addr)) { errors.push('第' + (i + 1) + '条: 地址非法 ' + it.addr); return; }
      if (cfg.Network[it.addr]) { skipped++; return; }
      cfg.Network[it.addr] = { Name: it.name, Addr: it.addr, Group: it.group, Pingmesh: false, Ping: [], Topology: [] };
      ws.forEach((pa) => {
        const probe = cfg.Network[pa];
        if (!probe || !probe.Pingmesh) return;
        probe.Ping = probe.Ping || [];
        probe.Topology = probe.Topology || [];
        if (probe.Ping.indexOf(it.addr) < 0) {
          probe.Ping.push(it.addr);
          probe.Topology.push(topoEntry(it.addr, it.name, null));
        }
      });
      added++;
    });
    const msg = '成功导入 ' + added + ' 个目标' + (skipped ? '，跳过已存在 ' + skipped + ' 个' : '');
    onChanged();
    if (errors.length > 0) {
      setResult({ err: errors.join('；'), msg: added ? msg : '' });
      if (added > 0) toast(msg + '（部分行有错误，详见弹窗）', 'err');
      return;
    }
    if (added > 0) {
      close();
      toast(msg + '，记得点击「保存配置」', 'ok');
    } else {
      setResult({ msg: '内容均已存在，无新增' });
    }
  };

  return (
    <Modal open={open} onClose={close} title="批量导入被测目标" width={1240}
      actions={<><Btn onClick={close}>取消</Btn><Btn primary onClick={save}>解析并导入</Btn></>}>
      <Box sx={{ color: palette.text3, mb: 1, fontSize: 12.5, lineHeight: 1.8 }}>
        支持两种格式，自动识别、与现有节点去重（重复地址自动跳过）：<br />
        1. <b>CSV</b>（每行一条）：<Box component="span" sx={{ fontFamily: mono }}>名称,IP或域名[,分组]</Box>　例如 <Box component="span" sx={{ fontFamily: mono }}>上海网关,10.1.1.1,华东</Box><br />
        2. <b>JSON</b>：<Box component="span" sx={{ fontFamily: mono }}>{'[{"name":"上海网关","addr":"10.1.1.1","group":"华东"}, ...]'}</Box>
      </Box>
      <TextArea rows={10} value={text} onChange={setText} placeholder={PH} />
      <Box sx={{ mt: 2 }}>
        <Box sx={{ ...flexRow, mb: 1 }}>
          <Box component="label" sx={{ ...labelSx, m: 0 }}>导入后由哪些探测节点监测（默认阈值 200ms/30%）</Box>
          <Spacer />
          <Btn sm onClick={() => setWatchers(watchers.map((w) => ({ ...w, on: true })))}>全选</Btn>
          <Btn sm onClick={() => setWatchers(watchers.map((w) => ({ ...w, on: false })))}>清空</Btn>
        </Box>
        <Box sx={{ maxHeight: 180, overflowY: 'auto', border: `1px solid ${palette.border}`, borderRadius: '10px', py: '6px' }}>
          <Table sx={tableSx}>
            <TableBody>
              {watchers.length === 0 && <TableRow><TableCell sx={{ color: palette.text3, p: '12px !important' }}>暂无探测节点，导入后可稍后再配置监测关系</TableCell></TableRow>}
              {watchers.map((w) => (
                <TableRow key={w.addr}>
                  <TableCell sx={{ width: 40, textAlign: 'center' }}>
                    <Chk checked={w.on} onChange={(on) => setWatchers(watchers.map((x) => (x.addr === w.addr ? { ...x, on } : x)))} />
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
      </Box>
      <Box sx={plainSx}>
        {result && result.err && <Box component="span" sx={{ color: palette.red }}>{result.err}</Box>}
        {result && result.err && result.msg && <br />}
        {result && result.msg}
      </Box>
    </Modal>
  );
}

