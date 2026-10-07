import { useEffect, useRef, useState } from 'react';
import { Box, Table, TableBody, TableCell, TableHead, TableRow } from '@mui/material';
import { Badge, EmptyState, Spinner } from '../../components/ui';
import { reachClear, reachLookup } from '../../api';
import { mono, palette } from '../../theme';
import { gmEntries, parseIps, removeGmEntry, trim } from './logic';
import { Btn, Chk, Field, Input, Modal, Spacer, TextArea, flexRow, plainSx, tableSx } from './ui';

const COUNTRIES = ['China', 'United States', 'Japan', 'South Korea', 'Singapore', 'India', 'Thailand', 'Vietnam', 'Indonesia',
  'Malaysia', 'Philippines', 'Australia', 'Germany', 'France', 'United Kingdom', 'Netherlands', 'Italy', 'Spain', 'Russia',
  'Turkey', 'Brazil', 'Mexico', 'Canada', 'Argentina', 'South Africa', 'Egypt', 'Saudi Arabia', 'United Arab Emirates'];
const IMPORT_PH = 'United States,骨干网,8.8.8.8\nJapan,骨干网,129.250.35.250\nSingapore,精品网,1.1.1.1';
const key = (r, l) => r + '\u0000' + l;

function GmReach({ v }) {
  if (v === undefined || v === null) return <Box component="span" sx={{ color: palette.text3 }}>—</Box>;
  if (v === 'loading') return <Spinner size={13} />;
  const { ok, total } = v;
  if (ok === 0) return <Badge tone="red" title="全部探测 IP 无应答">不可达</Badge>;
  if (ok === total) return <Badge tone="green" dot title="探测 IP 全部可达">可达{total > 1 ? ' ' + ok + '/' + total : ''}</Badge>;
  return <Badge tone="yellow" title="部分探测 IP 可达">{ok}/{total} 可达</Badge>;
}

export default function GmapTab({ cfg, ver, onChanged, active, reachSeq, toast, confirm }) {
  const [sel, setSel] = useState(() => new Set());
  const [reach, setReach] = useState({});
  const [dlg, setDlg] = useState(null);
  const [imp, setImp] = useState(null);
  const activeRef = useRef(active);
  activeRef.current = active;
  const entries = gmEntries(cfg);

  const fillReach = () => {
    const list = gmEntries(cfg);
    const init = {};
    list.forEach((e) => { init[key(e.region, e.line)] = e.ips.filter((x) => x !== '').length ? 'loading' : null; });
    setReach(init);
    list.forEach((e) => {
      const ips = e.ips.join(',').split(',').filter((x) => x !== '');
      if (ips.length === 0) return;
      let done = 0;
      let ok = 0;
      ips.forEach((ip) => reachLookup(ip).then((r) => {
        done++;
        if (r && r.reachable) ok++;
        if (done === ips.length) setReach((m) => ({ ...m, [key(e.region, e.line)]: { ok, total: ips.length } }));
      }));
    });
  };

  useEffect(() => {
    setSel(new Set());
    if (activeRef.current) fillReach();
    else setReach({});
  }, [ver]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { if (reachSeq > 0) fillReach(); }, [reachSeq]); // eslint-disable-line react-hooks/exhaustive-deps

  const allChecked = entries.length > 0 && entries.every((e) => sel.has(key(e.region, e.line)));
  const selList = entries.filter((e) => sel.has(key(e.region, e.line)));
  const toggle = (k, on) => setSel((s) => { const n = new Set(s); if (on) n.add(k); else n.delete(k); return n; });

  const delOne = async (r, l) => {
    if (!(await confirm('删除探测点', '确定删除「' + r + ' / ' + l + '」吗？'))) return;
    removeGmEntry(cfg, r, l);
    onChanged();
    toast('已从工作副本删除，记得点击「保存配置」', 'ok');
  };
  const delSel = async () => {
    const list = selList;
    if (list.length === 0) { toast('请先勾选要删除的探测点', 'err'); return; }
    if (!(await confirm('批量删除探测点', '确定删除勾选的 ' + list.length + ' 个探测点吗？'))) return;
    list.forEach((e) => removeGmEntry(cfg, e.region, e.line));
    onChanged();
    toast('已删除 ' + list.length + ' 个探测点（工作副本），记得点击「保存配置」', 'ok');
  };

  const openGm = (region, line) => setDlg({
    open: true,
    editing: region ? [region, line] : null,
    title: region ? '编辑探测点 - ' + region + ' / ' + line : '添加探测点',
    region: region || '', line: line || '骨干网',
    ips: region ? (cfg.Chinamap[region][line] || []).join(', ') : '',
  });
  const saveGm = () => {
    const region = trim(dlg.region);
    const line = trim(dlg.line);
    if (!region || !line) { toast('请填写地区与线路', 'err'); return; }
    let ips;
    try { ips = parseIps(dlg.ips); } catch (e) { toast(e.message, 'err'); return; }
    if (ips.length === 0) { toast('请至少填写一个探测 IP', 'err'); return; }
    const ed = dlg.editing;
    if (ed && (ed[0] !== region || ed[1] !== line)) {
      delete cfg.Chinamap[ed[0]][ed[1]];
      if (Object.keys(cfg.Chinamap[ed[0]]).length === 0) delete cfg.Chinamap[ed[0]];
    }
    if (!cfg.Chinamap[region]) cfg.Chinamap[region] = {};
    cfg.Chinamap[region][line] = ips;
    setDlg({ ...dlg, open: false });
    onChanged();
    toast('已更新工作副本，记得点击「保存配置」', 'ok');
  };

  const saveImport = () => {
    const text = trim(imp.text);
    if (!text) { toast('请粘贴导入内容', 'err'); return; }
    let added = 0;
    const errors = [];
    const mergeOne = (region, line, ips) => {
      if (!cfg.Chinamap[region]) cfg.Chinamap[region] = {};
      if (!cfg.Chinamap[region][line]) cfg.Chinamap[region][line] = [];
      ips.forEach((ip) => {
        if (cfg.Chinamap[region][line].indexOf(ip) < 0) {
          cfg.Chinamap[region][line].push(ip);
          added++;
        }
      });
    };
    if (text.charAt(0) === '{') {
      try {
        const obj = JSON.parse(text);
        Object.keys(obj).forEach((region) => {
          const lines = obj[region];
          Object.keys(lines).forEach((line) => { mergeOne(region, line, parseIps((lines[line] || []).join(','))); });
        });
      } catch (e) { setImp({ ...imp, result: { err: 'JSON 解析失败: ' + e.message } }); onChanged(); return; }
    } else {
      text.split('\n').forEach((raw, i) => {
        const lineTxt = trim(raw);
        if (lineTxt === '') return;
        const parts = lineTxt.split(/[,，]/);
        if (parts.length < 3) { errors.push('第' + (i + 1) + '行: 字段不足(需 地区,线路,IP...)'); return; }
        try {
          const ips = parseIps(parts.slice(2).join(','));
          if (ips.length === 0) { errors.push('第' + (i + 1) + '行: 无有效 IP'); return; }
          mergeOne(trim(parts[0]), trim(parts[1]), ips);
        } catch (e) { errors.push('第' + (i + 1) + '行: ' + e.message); }
      });
    }
    let result = null;
    if (errors.length > 0) {
      result = { err: errors.join('；') };
      if (added === 0) { setImp({ ...imp, result }); return; }
    }
    onChanged();
    if (added > 0) {
      setImp({ ...imp, open: false, result });
      toast('成功导入 ' + added + ' 个探测 IP，记得点击「保存配置」', 'ok');
    } else if (errors.length === 0) {
      setImp({ ...imp, result: { msg: '内容均已存在，无新增' } });
    }
  };

  return (
    <Box>
      <Box sx={{ p: '18px 18px 0', '@media (max-width:900px)': { p: '14px 14px 0' } }}>
        <Box sx={{ ...flexRow, mb: 2, '@media (max-width:900px)': { flexWrap: 'wrap' } }}>
          <Box component="span" sx={{ color: palette.text3, fontSize: 12.5 }}>
            地区使用<b>英文国家/地区名</b>（United States、Japan、Singapore...）才能在世界地图着色，线路名称任意（骨干网 / 精品网 / CN2...）
          </Box>
          <Box component="span" sx={{ color: palette.primary }}>{selList.length > 0 ? '已勾选 ' + selList.length + ' 项' : ''}</Box>
          <Spacer />
          <Btn sm danger disabled={selList.length === 0} title="删除勾选的探测点" onClick={delSel}>批量删除</Btn>
          <Btn sm title="重新检测所有探测 IP 的可达状态" onClick={() => { reachClear(); fillReach(); }}>刷新可达状态</Btn>
          <Btn sm onClick={() => setImp({ open: true, text: '', result: null })}>批量导入</Btn>
          <Btn sm primary onClick={() => openGm(null, null)}>+ 添加探测点</Btn>
        </Box>
      </Box>
      <Box sx={{ overflowX: 'auto' }}>
        <Table sx={tableSx}>
          <TableHead>
            <TableRow>
              <TableCell sx={{ width: 36, textAlign: 'center !important' }}>
                <Chk checked={allChecked} title="全选 / 取消全选"
                  onChange={(on) => setSel(on ? new Set(entries.map((e) => key(e.region, e.line))) : new Set())} />
              </TableCell>
              <TableCell>地区</TableCell><TableCell>线路</TableCell><TableCell>探测 IP</TableCell><TableCell>可达状态</TableCell>
              <TableCell sx={{ width: 150 }}>操作</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {entries.map((e) => {
              const k = key(e.region, e.line);
              return (
                <TableRow key={k}>
                  <TableCell sx={{ textAlign: 'center' }}><Chk checked={sel.has(k)} onChange={(on) => toggle(k, on)} /></TableCell>
                  <TableCell><b>{e.region}</b></TableCell>
                  <TableCell><Badge tone="indigo">{e.line}</Badge></TableCell>
                  <TableCell sx={{ fontFamily: mono, fontSize: '12px !important' }}>{e.ips.join(', ')}</TableCell>
                  <TableCell><GmReach v={reach[k]} /></TableCell>
                  <TableCell sx={{ whiteSpace: 'nowrap' }}>
                    <Btn sm onClick={() => openGm(e.region, e.line)}>编辑</Btn>{' '}
                    <Btn sm danger onClick={() => delOne(e.region, e.line)}>删除</Btn>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </Box>
      {entries.length === 0 && <EmptyState icon="🌍">尚未配置探测点，点击「添加探测点」或「批量导入」</EmptyState>}

      <Modal open={!!dlg && dlg.open} onClose={() => setDlg({ ...dlg, open: false })} title={dlg ? dlg.title : ''}
        actions={<><Btn onClick={() => setDlg({ ...dlg, open: false })}>取消</Btn><Btn primary onClick={saveGm}>确定</Btn></>}>
        {dlg && (
          <>
            <Field label="地区（英文国家/地区名，可着色到世界地图）">
              <Input value={dlg.region} onChange={(v) => setDlg({ ...dlg, region: v })} placeholder="如: United States" inputProps={{ list: 'gm-countries' }} />
              <datalist id="gm-countries">{COUNTRIES.map((c) => <option key={c} value={c} />)}</datalist>
            </Field>
            <Field label="线路名称"><Input value={dlg.line} onChange={(v) => setDlg({ ...dlg, line: v })} placeholder="如: 骨干网 / 精品网 / CN2" /></Field>
            <Field label="探测 IP（多个用逗号、空格或换行分隔，取多 IP 平均值）">
              <TextArea rows={3} value={dlg.ips} onChange={(v) => setDlg({ ...dlg, ips: v })} placeholder="8.8.8.8, 8.8.4.4" />
            </Field>
          </>
        )}
      </Modal>

      <Modal open={!!imp && imp.open} onClose={() => setImp({ ...imp, open: false })} title="批量导入探测点" width={1240}
        actions={<><Btn onClick={() => setImp({ ...imp, open: false })}>取消</Btn><Btn primary onClick={saveImport}>解析并导入</Btn></>}>
        {imp && (
          <>
            <Box sx={{ color: palette.text3, mb: 1, fontSize: 12.5, lineHeight: 1.8 }}>
              支持两种格式，自动识别、与现有配置合并去重：<br />
              1. <b>CSV</b>（每行一条）：<Box component="span" sx={{ fontFamily: mono }}>地区,线路,IP1,IP2...</Box>　例如 <Box component="span" sx={{ fontFamily: mono }}>United States,骨干网,8.8.8.8,8.8.4.4</Box><br />
              2. <b>JSON</b>：<Box component="span" sx={{ fontFamily: mono }}>{'{ "地区": { "线路": ["IP", ...] } }'}</Box>
            </Box>
            <TextArea rows={12} value={imp.text} onChange={(v) => setImp({ ...imp, text: v })} placeholder={IMPORT_PH} />
            <Box sx={plainSx}>
              {imp.result && imp.result.err && <Box component="span" sx={{ color: palette.red }}>{imp.result.err}</Box>}
              {imp.result && imp.result.msg}
            </Box>
          </>
        )}
      </Modal>
    </Box>
  );
}
