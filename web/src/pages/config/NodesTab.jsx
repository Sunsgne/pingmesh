import { useEffect, useRef, useState } from 'react';
import { Box, Table, TableBody, TableCell, TableHead, TableRow } from '@mui/material';
import { Badge, Spinner } from '../../components/ui';
import { asnLookup, asnShort, nodeName, reachClear, reachLookup } from '../../api';
import { mono, palette } from '../../theme';
import { groupNames, removeNodes } from './logic';
import { Btn, Chk, Field, Input, Modal, Spacer, flexRow, softBox, tableSx } from './ui';
import NodeModal from './NodeModal';
import PlaneWizard from './PlaneWizard';
import NodeImport from './NodeImport';

const wrap = { whiteSpace: 'normal' };

export function ReachBadge({ r }) {
  if (r === 'loading') return <Spinner size={13} />;
  if (r === undefined) return <Box component="span" sx={{ color: palette.text3 }}>—</Box>;
  if (!r) return <Badge tone="gray" title="无法解析或探测出错">检测失败</Badge>;
  if (r.reachable) {
    const ms = r.rtt != null && r.rtt > 0 ? ' ' + Math.round(r.rtt) + 'ms' : '';
    return <Badge tone="green" dot sx={wrap} title="ICMP 可达">可达{ms}</Badge>;
  }
  return <Badge tone="red" title="ICMP 无应答(可能离线或屏蔽 ICMP)">不可达</Badge>;
}

function AsnCell({ info }) {
  if (info === undefined) return <Box component="span" sx={{ color: palette.text3 }}>…</Box>;
  if (!info) return <Box component="span" sx={{ color: palette.text3 }} title="内网地址或未查询到">—</Box>;
  return <Badge tone="indigo" sx={{ fontSize: 10.5, ...wrap }} title={'AS' + info.asn + ' ' + (info.holder || '') + ' ' + (info.prefix || '')}>{asnShort(info)}</Badge>;
}

export default function NodesTab({ cfg, ver, onChanged, active, reachSeq, toast, confirm }) {
  const [search, setSearch] = useState('');
  const [sel, setSel] = useState(() => new Set());
  const [asn, setAsn] = useState({});
  const [reach, setReach] = useState({});
  const [groupDlg, setGroupDlg] = useState(null);
  const [nodeDlg, setNodeDlg] = useState(null);
  const [wizardOpen, setWizardOpen] = useState(0);
  const [importOpen, setImportOpen] = useState(0);
  const activeRef = useRef(active);
  activeRef.current = active;

  const addrs = Object.keys(cfg.Network || {});

  const fillReach = () => {
    const list = Object.keys(cfg.Network || {});
    setReach(Object.fromEntries(list.map((a) => [a, 'loading'])));
    list.forEach((a) => reachLookup(a).then((r) => setReach((m) => ({ ...m, [a]: r }))));
  };

  useEffect(() => {
    setSel(new Set());
    setAsn({});
    addrs.forEach((a) => asnLookup(a).then((info) => setAsn((m) => ({ ...m, [a]: info }))));
    if (activeRef.current) fillReach();
    else setReach({});
  }, [ver]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => { if (reachSeq > 0) fillReach(); }, [reachSeq]); // eslint-disable-line react-hooks/exhaustive-deps

  const watchedBy = {};
  addrs.forEach((a) => {
    const n = cfg.Network[a];
    if (!n.Pingmesh) return;
    (n.Ping || []).forEach((t) => { watchedBy[t] = (watchedBy[t] || 0) + 1; });
  });

  const q = search.trim().toLowerCase();
  const isVis = (a) => {
    const n = cfg.Network[a];
    return q === '' || (n.Name + ' ' + a + ' ' + (n.Group || '')).toLowerCase().indexOf(q) >= 0;
  };
  const visCheckable = addrs.filter((a) => a !== cfg.Addr && isVis(a));
  const allChecked = visCheckable.length > 0 && visCheckable.every((a) => sel.has(a));
  const selList = addrs.filter((a) => sel.has(a));

  const toggle = (a, on) => setSel((s) => { const n = new Set(s); if (on) n.add(a); else n.delete(a); return n; });
  const checkAll = (on) => setSel((s) => { const n = new Set(s); visCheckable.forEach((a) => (on ? n.add(a) : n.delete(a))); return n; });

  const delOne = async (a) => {
    if (a === cfg.Addr) { toast('不能删除本机节点', 'err'); return; }
    if (!(await confirm('删除节点', '确定删除节点「' + nodeName(cfg, a) + ' (' + a + ')」吗？其他节点对它的监测也会一并移除。'))) return;
    removeNodes(cfg, [a]);
    onChanged();
    toast('已从工作副本删除，记得点击「保存配置」', 'ok');
  };
  const delSel = async () => {
    if (selList.length === 0) { toast('请先勾选要删除的节点', 'err'); return; }
    const list = selList;
    if (!(await confirm('批量删除节点', '确定删除勾选的 ' + list.length + ' 个节点吗？其他节点对它们的监测关系会一并移除。'))) return;
    removeNodes(cfg, list);
    onChanged();
    toast('已删除 ' + list.length + ' 个节点（工作副本），记得点击「保存配置」', 'ok');
  };
  const openGroup = () => {
    if (selList.length === 0) { toast('请先勾选要设置分组的节点', 'err'); return; }
    setGroupDlg({ name: '', groups: groupNames(cfg) });
  };
  const saveGroup = () => {
    const gname = groupDlg.name.trim();
    selList.forEach((a) => { if (cfg.Network[a]) cfg.Network[a].Group = gname; });
    setGroupDlg(null);
    onChanged();
    toast((gname ? '已将 ' + selList.length + ' 个节点分组设置为「' + gname + '」' : '已清除 ' + selList.length + ' 个节点的分组') + '（工作副本），记得点击「保存配置」', 'ok');
  };

  return (
    <Box>
      <Box sx={{ p: '18px 18px 0', '@media (max-width:900px)': { p: '14px 14px 0' } }}>
        <Box sx={{ ...flexRow, mb: 1, '@media (max-width:900px)': { flexWrap: 'wrap' } }}>
          <Box component="span" sx={{ color: palette.text3 }}>共 <b>{addrs.length}</b> 个节点</Box>
          <Box component="span" sx={{ color: palette.primary }}>{selList.length > 0 ? '已勾选 ' + selList.length + ' 个' : ''}</Box>
          <Spacer />
          <Input dense width={220} value={search} onChange={setSearch} placeholder="搜索名称 / IP / 分组..." sx={{ '& .MuiOutlinedInput-input': { padding: '6px 10px' } }} />
          <Btn sm disabled={selList.length === 0} title="为勾选的节点统一设置分组" onClick={openGroup}>批量分组</Btn>
          <Btn sm danger disabled={selList.length === 0} title="删除勾选的节点" onClick={delSel}>批量删除</Btn>
          <Btn sm title="重新检测所有节点 IP 的可达状态" onClick={() => { reachClear(); fillReach(); }}>刷新可达状态</Btn>
          <Btn sm onClick={() => setWizardOpen((x) => x + 1)}>双网口组网向导</Btn>
          <Btn sm onClick={() => setImportOpen((x) => x + 1)}>批量导入被测目标</Btn>
          <Btn sm primary onClick={() => setNodeDlg({ addr: null, seq: Date.now() })}>+ 添加节点</Btn>
        </Box>
        <Box sx={{ ...softBox, mb: 2 }}>
          <b>探测节点</b>=部署了 PingMesh 的机器，主动 PING 别人；<b>被测目标</b>=网关/DNS/服务器等，只被 PING。<br />
          <b>单向监测</b>：编辑探测节点 → 勾选「我监测它」。<b>互 PING</b>：两个探测节点之间再勾选「互PING」即可，无需分别编辑两个节点。<br />
          <b>双网口(公网+专线)</b>：把对端的两个 IP 注册为两个被测目标（分组建议设为 公网/专线），并在监测规则的「源IP」里分别填本机公网口/专线口 IP，两条路径即可完全分开监控。
        </Box>
      </Box>
      <Box sx={{ overflowX: 'auto' }}>
        <Table sx={tableSx}>
          <TableHead>
            <TableRow>
              <TableCell sx={{ width: 36, textAlign: 'center !important' }}>
                <Chk checked={allChecked} onChange={checkAll} title="全选 / 取消全选（当前筛选结果）" />
              </TableCell>
              <TableCell>节点名称</TableCell><TableCell>IP 地址</TableCell><TableCell>ASN</TableCell><TableCell>可达状态</TableCell>
              <TableCell>分组</TableCell><TableCell>类型</TableCell><TableCell>监测目标</TableCell><TableCell>被谁监测</TableCell>
              <TableCell sx={{ width: 170 }}>操作</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {addrs.map((a) => {
              const n = cfg.Network[a];
              const self = a === cfg.Addr;
              return (
                <TableRow key={a} sx={{ display: isVis(a) ? undefined : 'none' }}>
                  <TableCell sx={{ textAlign: 'center' }}>
                    {self
                      ? <Chk disabled title="本机节点不可批量操作" />
                      : <Chk checked={sel.has(a)} onChange={(on) => toggle(a, on)} />}
                  </TableCell>
                  <TableCell><b>{n.Name}</b>{self && <> <Badge tone="indigo" sx={wrap}>本机</Badge></>}</TableCell>
                  <TableCell sx={{ fontFamily: mono, fontSize: '12.5px !important' }}>{a}</TableCell>
                  <TableCell><AsnCell info={asn[a]} /></TableCell>
                  <TableCell><ReachBadge r={reach[a]} /></TableCell>
                  <TableCell>{n.Group ? <Badge tone="gray" sx={wrap}>{n.Group}</Badge> : <Box component="span" sx={{ color: palette.text3 }}>—</Box>}</TableCell>
                  <TableCell>{n.Pingmesh ? <Badge tone="green" dot sx={wrap}>探测节点</Badge> : <Badge tone="gray" sx={wrap}>被测目标</Badge>}</TableCell>
                  <TableCell>{n.Pingmesh ? (n.Ping || []).length + ' 个' : <Box component="span" sx={{ color: palette.text3 }}>—</Box>}</TableCell>
                  <TableCell>{watchedBy[a] ? watchedBy[a] + ' 个节点' : <Badge tone="yellow" sx={wrap} title="没有任何探测节点监测它">未被监测</Badge>}</TableCell>
                  <TableCell sx={{ whiteSpace: 'nowrap' }}>
                    <Btn sm onClick={() => setNodeDlg({ addr: a, seq: Date.now() })}>编辑</Btn>{' '}
                    <Btn sm danger onClick={() => delOne(a)}>删除</Btn>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </Box>

      <Modal open={!!groupDlg} onClose={() => setGroupDlg(null)} title="批量设置分组" width={420}
        actions={<><Btn onClick={() => setGroupDlg(null)}>取消</Btn><Btn primary onClick={saveGroup}>确定</Btn></>}>
        {groupDlg && (
          <Field sx={{ mb: 0 }} label={<>分组名称（将应用到已勾选的 <b>{selList.length}</b> 个节点，留空表示清除分组）</>}>
            <Input value={groupDlg.name} onChange={(v) => setGroupDlg({ ...groupDlg, name: v })} placeholder="如: 华东 / 海外 / 核心网" inputProps={{ list: 'ngroup-list' }} autoFocus />
            <datalist id="ngroup-list">{groupDlg.groups.map((g) => <option key={g} value={g} />)}</datalist>
          </Field>
        )}
      </Modal>

      <NodeModal req={nodeDlg} onClose={() => setNodeDlg(null)} cfg={cfg} onChanged={onChanged} toast={toast}
        onGotoImport={() => { setNodeDlg(null); setImportOpen((x) => x + 1); }} />
      <PlaneWizard openSeq={wizardOpen} cfg={cfg} onChanged={onChanged} toast={toast} />
      <NodeImport openSeq={importOpen} cfg={cfg} onChanged={onChanged} toast={toast} />
    </Box>
  );
}
