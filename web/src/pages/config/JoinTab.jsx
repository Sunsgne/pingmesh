import { useRef, useState } from 'react';
import { Box, Typography } from '@mui/material';
import { Badge } from '../../components/ui';
import { mono, palette } from '../../theme';
import { randomToken, trim } from './logic';
import { postJq } from './post';
import { Btn, Field, Hint, Input, SwBlock, TextArea } from './ui';

export default function JoinTab({ cfg, form, setF, toast, confirm, onReload }) {
  const cmdRef = useRef(null);
  const [master, setMaster] = useState('');
  const [token, setToken] = useState('');
  const [joining, setJoining] = useState(false);

  const t = form.jointoken || '<接入令牌>';
  const cmd = './pingmesh -join http://' + cfg.Addr + ':' + cfg.Port + ' -token ' + t + ' -name <节点名> [-group <分组>]';
  const isCloud = cfg.Mode && cfg.Mode.Type === 'cloud';

  const regen = () => {
    setF('jointoken', randomToken());
    toast('已生成新令牌，记得点击「保存配置」', 'ok');
  };
  const copy = () => {
    const el = cmdRef.current;
    el.select();
    try { document.execCommand('copy'); toast('已复制', 'ok'); } catch (e) { toast('复制失败，请手动复制', 'err'); }
  };
  const standalone = async () => {
    if (!(await confirm('切换为独立模式', '确定脱离主节点吗？本节点将停止同步配置，已有监控配置保持不变。'))) return;
    postJq('/api/mode.json', { action: 'standalone' }).then((res) => {
      if (res.status === 'true') { toast(res.info, 'ok'); onReload(); } else toast(res.info || '切换失败', 'err');
    }).catch(() => {});
  };
  const join = () => {
    const m = trim(master);
    const tk = trim(token);
    if (!m || !tk) { toast('请填写主节点地址与接入令牌', 'err'); return; }
    setJoining(true);
    postJq('/api/mode.json', { action: 'join', master: m, token: tk }).then((res) => {
      if (res.status === 'true') { toast(res.info, 'ok'); onReload(); } else toast(res.info || '加入失败', 'err');
    }).catch(() => toast('请求失败', 'err')).finally(() => setJoining(false));
  };

  return (
    <Box>
      <Typography component="h3" sx={{ fontSize: 14.5, fontWeight: 600, mb: '10px' }}>主节点 + 自动组网</Typography>
      <Box sx={{ color: palette.text3, mb: 2, fontSize: 13, lineHeight: 1.9 }}>
        推荐架构：任选一台部署作为<b>主节点</b>（保存全量配置与用户），其他机器用下面的一条命令加入。<br />
        加入后自动完成<b>全互联 Pingmesh 组网</b>（新节点监测所有既有节点、所有探测节点监测新节点），
        并切换为 cloud 模式，每分钟从主节点自动同步配置 —— 之后所有改动只需在主节点页面操作。
      </Box>
      <Field label="接入令牌（即配置密码，主节点与新节点经此互信）" hint="修改后点击「保存配置」生效；新节点 join 时会自动采用该令牌（集群令牌统一，签名/加密密钥对齐）">
        <Box sx={{ display: 'flex', gap: '10px', '@media (max-width:900px)': { flexDirection: 'column' } }}>
          <Input isMono value={form.jointoken} onChange={(v) => setF('jointoken', v)} placeholder="加载中..." />
          <Btn sm sx={{ flex: 'none' }} onClick={regen}>随机生成</Btn>
        </Box>
      </Field>
      <Field label="API 签名与加密" hint="开启前提：所有节点已升级到本版本且接入令牌一致（重新 join 即自动统一）。开启后仅凭 IP 互信的请求将被拒绝">
        <SwBlock checked={form.apisign} onChange={(v) => setF('apisign', v)} label="API 签名与加密" />
        <Hint inline>节点间接口强制 HMAC-SHA256 验签（防伪造、±5分钟时间戳+nonce 防重放），配置同步走 AES-256-GCM 加密</Hint>
      </Field>
      <Field label="在新节点上执行（单二进制，自带页面资源，首次运行自动初始化）">
        <TextArea rows={3} value={cmd} readOnly inputRef={cmdRef} />
        <Box sx={{ mt: 1 }}>
          <Btn sm onClick={copy}>复制命令</Btn>
          <Hint inline>把 &lt;节点名&gt; 换成机房/地域名称；多数情况下 IP 可自动识别，跨 NAT 时用 -addr 显式指定</Hint>
        </Box>
      </Field>
      <Field label="当前节点模式">
        <Box sx={{ color: palette.text3, mb: 1, fontSize: 13 }}>
          {isCloud
            ? <><Badge tone="indigo">Agent 模式</Badge> 配置同步自 <Box component="span" sx={{ fontFamily: mono }}>{cfg.Mode.Endpoint || ''}</Box>，最后同步 {cfg.Mode.LastSuccTime || '-'}</>
            : <><Badge tone="green" dot>主节点 / 独立模式</Badge> 其他节点可使用上面的命令加入本节点</>}
        </Box>
        {isCloud ? (
          <Box>
            <Btn sm danger onClick={standalone}>切换为独立 / 主节点模式</Btn>
            <Hint inline>脱离主节点，停止配置同步，本节点配置自此独立维护</Hint>
          </Box>
        ) : (
          <Box>
            <Box sx={{ display: 'flex', gap: '10px', maxWidth: 680, '@media (max-width:900px)': { flexDirection: 'column' } }}>
              <Input isMono value={master} onChange={setMaster} placeholder="主节点地址, 如 http://10.0.0.1:8899" />
              <Input isMono value={token} onChange={setToken} placeholder="接入令牌" sx={{ maxWidth: 180 }} />
              <Btn sm primary sx={{ flex: 'none' }} disabled={joining} onClick={join}>{joining ? '加入中...' : '加入主节点'}</Btn>
            </Box>
            <Hint>以当前节点名与 IP 加入对方集群，加入后切换为 Agent 模式并每分钟同步配置</Hint>
          </Box>
        )}
      </Field>
    </Box>
  );
}
