import { useRef, useState } from 'react';
import { Box, Card, Tab, Tabs } from '@mui/material';
import { EmptyState } from '../components/ui';
import { useConfirm, useToast } from '../components/Feedback';
import { getJSON } from '../api';
import { palette } from '../theme';
import { collectFromForms, deepCopy, fillForm } from './config/logic';
import { postJq } from './config/post';
import { Btn, TextArea } from './config/ui';
import BasicTab from './config/BasicTab';
import AuthTab from './config/AuthTab';
import NodesTab from './config/NodesTab';
import JoinTab from './config/JoinTab';
import AlertTab from './config/AlertTab';
import GmapTab from './config/GmapTab';

const TABS = [
  ['basic', '基础设置'], ['auth', '登录认证'], ['nodes', '节点管理'], ['join', '节点接入'],
  ['alert', '报警通道'], ['chinamap', '全球延迟'], ['raw', '高级 (JSON)'],
];
const TAB_IDS = TABS.map((t) => t[0]);

function initState(config) {
  const cfg = deepCopy(config);
  const hash = (window.location.hash || '').replace('#', '');
  const tab = hash && TAB_IDS.indexOf(hash) >= 0 ? hash : 'basic';
  // 旧版在 fill() 之前就处理 #raw 直达: 此时表单为空, collectFromForms 在地址校验处抛错, 原文即未改动的配置
  const raw = tab === 'raw' ? JSON.stringify(cfg, null, 2) : '';
  const form = fillForm(cfg);
  return { cfg, tab, raw, form };
}

function Pane({ show, children, sx }) {
  return <Box sx={{ display: show ? 'block' : 'none', ...sx }}>{children}</Box>;
}
const bodySx = { p: '18px', '@media (max-width:900px)': { p: '14px' } };

function ConfigEditor({ config }) {
  const toast = useToast();
  const confirm = useConfirm();
  const init = useRef(null);
  if (!init.current) init.current = initState(config);
  const cfgRef = useRef(init.current.cfg);
  const [tab, setTab] = useState(init.current.tab);
  const [form, setForm] = useState(init.current.form);
  const [raw, setRaw] = useState(init.current.raw);
  const [, setVer] = useState(0);
  const [nodesVer, setNodesVer] = useState(0);
  const [gmVer, setGmVer] = useState(0);
  const [nodesReach, setNodesReach] = useState(0);
  const [gmReach, setGmReach] = useState(0);
  const [saving, setSaving] = useState(false);
  const cfg = cfgRef.current;

  // bump: 仅重绘(对应旧版改了 cfg 但未重新渲染列表的场景, 保留勾选状态);
  // nodesChanged / gmChanged: 对应旧版 renderNodes() / renderGmap(), 会清空勾选并刷新 ASN/可达状态
  const bump = () => setVer((v) => v + 1);
  const nodesChanged = () => setNodesVer((v) => v + 1);
  const gmChanged = () => setGmVer((v) => v + 1);
  const setF = (k, v) => setForm((f) => ({ ...f, [k]: v }));
  const fill = () => {
    setForm(fillForm(cfgRef.current));
    nodesChanged();
    gmChanged();
  };

  const onTab = (t) => {
    setTab(t);
    window.location.hash = t;
    if (t === 'raw') {
      try { collectFromForms(cfgRef.current, form); } catch (e) { /* 保持原值 */ }
      setRaw(JSON.stringify(cfgRef.current, null, 2));
      bump();
    }
    if (t === 'nodes') setNodesReach((x) => x + 1);
    if (t === 'chinamap') setGmReach((x) => x + 1);
  };

  const reloadCfg = () => {
    getJSON('/api/config.json').then((ncfg) => {
      cfgRef.current = deepCopy(ncfg);
      fill();
    }).catch(() => {});
  };

  const formatRaw = () => {
    try { setRaw(JSON.stringify(JSON.parse(raw), null, 2)); } catch (e) { toast('JSON 格式错误: ' + e.message, 'err'); }
  };

  const save = () => {
    let finalCfg;
    if (tab === 'raw') {
      try { finalCfg = JSON.parse(raw); } catch (e) { toast('JSON 格式错误: ' + e.message, 'err'); return; }
    } else {
      try { finalCfg = collectFromForms(cfgRef.current, form); } catch (e) { bump(); toast(e.message, 'err'); return; }
      bump();
    }
    if (!finalCfg || !finalCfg.Network || !finalCfg.Network[finalCfg.Addr]) {
      toast('节点列表中必须包含本机节点 ' + (finalCfg ? finalCfg.Addr : ''), 'err');
      return;
    }
    setSaving(true);
    postJq('/api/saveconfig.json', { config: JSON.stringify(finalCfg) }).then((res) => {
      if (res.status === 'true') {
        toast('配置保存成功', 'ok');
        // 保存后版本号已变, 页面副本要跟上, 否则下一次保存会被当作过期副本拒绝
        if (res.epoch) finalCfg.Mode = { ...(finalCfg.Mode || {}), Epoch: res.epoch };
        cfgRef.current = finalCfg;
        fill();
      } else {
        toast(res.info || '保存失败', 'err');
      }
    }).catch(() => toast('保存请求失败', 'err')).finally(() => setSaving(false));
  };

  const common = { cfg, toast, confirm };
  return (
    <Card sx={{ overflow: 'visible' }}>
      <Tabs value={tab} onChange={(e, t) => onTab(t)} variant="scrollable" scrollButtons="auto" allowScrollButtonsMobile
        sx={{
          minHeight: 0, borderBottom: `1px solid ${palette.border}`, px: '18px',
          '& .MuiTabs-flexContainer': { gap: '4px' },
          '& .MuiTabs-indicator': { backgroundColor: palette.primary, height: 2 },
        }}>
        {TABS.map(([id, label]) => (
          <Tab key={id} value={id} label={label} disableRipple
            onClick={() => { if (id === tab) onTab(id); }}
            sx={{
              minHeight: 0, minWidth: 0, p: '13px 14px', fontSize: 13.5, fontWeight: 600, color: palette.text3,
              '&:hover': { color: palette.text }, '&.Mui-selected': { color: palette.primary },
            }} />
        ))}
      </Tabs>

      <Pane show={tab === 'basic'} sx={{ ...bodySx, maxWidth: 760 }}><BasicTab form={form} setF={setF} toast={toast} /></Pane>
      <Pane show={tab === 'auth'} sx={{ ...bodySx, maxWidth: 760 }}><AuthTab form={form} setF={setF} /></Pane>
      <Pane show={tab === 'nodes'}>
        <NodesTab {...common} ver={nodesVer} onChanged={nodesChanged} active={tab === 'nodes'} reachSeq={nodesReach} />
      </Pane>
      <Pane show={tab === 'join'} sx={{ ...bodySx, maxWidth: 820 }}>
        <JoinTab {...common} form={form} setF={setF} onReload={reloadCfg} />
      </Pane>
      <Pane show={tab === 'alert'} sx={bodySx}><AlertTab {...common} form={form} setF={setF} onChanged={bump} /></Pane>
      <Pane show={tab === 'chinamap'}>
        <GmapTab {...common} ver={gmVer} onChanged={gmChanged} active={tab === 'chinamap'} reachSeq={gmReach} />
      </Pane>
      <Pane show={tab === 'raw'} sx={bodySx}>
        <Box sx={{ color: palette.text3, mb: 2, fontSize: 13 }}>直接编辑完整配置 JSON（保存时以此为准，请谨慎修改）</Box>
        <TextArea rows={24} value={raw} onChange={setRaw} id="f-raw" />
        <Box sx={{ mt: 1 }}><Btn sm onClick={formatRaw}>格式化</Btn></Box>
      </Pane>

      <Box sx={{
        position: 'sticky', bottom: 0, zIndex: 30, display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap',
        mt: '18px', p: '12px 16px', background: 'rgba(255,255,255,.92)', backdropFilter: 'blur(8px)',
        border: `1px solid ${palette.border}`, borderRadius: '12px', boxShadow: '0 -8px 24px rgba(15,23,42,.06)',
      }}>
        <Btn primary disabled={saving} onClick={save}>{saving ? '保存中...' : '保存配置'}</Btn>
        <Box component="span" sx={{ fontSize: 12.5, color: palette.text3 }}>保存后约 1 分钟内生效；互 PING 节点会按各自配置同步</Box>
      </Box>
    </Card>
  );
}

export default function Config({ config, isAdmin }) {
  if (!isAdmin) return <EmptyState icon="🔒">没有权限访问该页面</EmptyState>;
  return <ConfigEditor config={config} />;
}
