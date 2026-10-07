import { useRef } from 'react';
import { Box } from '@mui/material';
import { palette } from '../../theme';
import { Btn, Field, Hint, Input, SwBlock, gridCols } from './ui';

const DEFAULT_LOGO = 'assets/img/logo.png';
const full = { gridColumn: '1 / -1' };

export default function BasicTab({ form, setF, toast }) {
  const fileRef = useRef(null);
  const logo = form.brandLogo;

  const onFile = (e) => {
    const input = e.target;
    const f = input.files && input.files[0];
    if (!f) return;
    if (!/^image\//.test(f.type)) { toast('请选择图片文件', 'err'); input.value = ''; return; }
    if (f.size > 256 * 1024) { toast('图片过大，请压缩到 256KB 以内', 'err'); input.value = ''; return; }
    const reader = new FileReader();
    reader.onload = (ev) => {
      setF('brandLogo', ev.target.result);
      toast('Logo 已就绪，记得点击「保存配置」', 'ok');
    };
    reader.readAsDataURL(f);
    input.value = '';
  };

  const num = (k, label, attrs, hint) => (
    <Field label={label} hint={hint}>
      <Input type="number" value={form[k]} onChange={(v) => setF(k, v)} inputProps={attrs} />
    </Field>
  );

  return (
    <Box sx={gridCols(2)}>
      <Field sx={{ ...full, mb: 0 }} label="品牌定制 — 登录页与左上角显示的名称 / 标语 / Logo" labelStyle={{ fontSize: 14 }}
        hint="留空则使用默认值。Logo 建议上传正方形 PNG（≤256KB），将随配置在集群内同步" />
      <Field label="品牌名称"><Input value={form.brandName} onChange={(v) => setF('brandName', v)} placeholder="ZENLENET" /></Field>
      <Field label="标语 (Slogan)"><Input value={form.brandSlogan} onChange={(v) => setF('brandSlogan', v)} placeholder="PingMesh · 网络质量监控平台" /></Field>
      <Field sx={full} label="Logo 图片">
        <Box sx={{ display: 'flex', alignItems: 'center', gap: '14px' }}>
          <Box sx={{ width: 46, height: 46, border: `1px solid ${palette.border}`, borderRadius: '10px', overflow: 'hidden', background: palette.primarySoft, flex: 'none', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <img src={logo || DEFAULT_LOGO} alt="logo" style={{ maxWidth: '100%', maxHeight: '100%', display: 'block' }} />
          </Box>
          <input ref={fileRef} type="file" accept="image/*" style={{ display: 'none' }} onChange={onFile} data-testid="brand-logo-file" />
          <Btn sm onClick={() => fileRef.current && fileRef.current.click()}>选择图片</Btn>
          {logo && <Btn sm danger onClick={() => setF('brandLogo', '')}>恢复默认</Btn>}
          <Hint sx={{ mt: 0 }}>{logo ? '已自定义 Logo' : '当前使用默认 Logo'}</Hint>
        </Box>
      </Field>
      <Box sx={{ ...full, borderTop: `1px solid ${palette.border}`, m: '4px 0 0', pt: 1 }} />
      <Field label="本机节点名称"><Input value={form.name} onChange={(v) => setF('name', v)} /></Field>
      <Field label="本机节点 IP" hint="其他节点通过该 IP 访问本节点"><Input isMono value={form.addr} onChange={(v) => setF('addr', v)} /></Field>
      {num('timeout', '请求超时 (秒)', { min: 1 })}
      {num('archive', '数据存档天数', { min: 1 })}
      {num('refresh', '页面刷新频率 (分钟)', { min: 1 })}
      {num('toollimit', '检测工具限频 (秒)', { min: 0 }, '同一来源两次检测的最小间隔，0 表示不限制')}
      {num('tline', '拓扑连线粗细', { min: 1 })}
      {num('tsymbolsize', '拓扑节点大小', { min: 10 })}
      <Field label="报警提示音地址"><Input value={form.tsound} onChange={(v) => setF('tsound', v)} /></Field>
      <Field label="访问 IP 白名单" hint="逗号分隔。白名单中的 IP 无需登录即可访问（兼容旧版）">
        <Input isMono value={form.authiplist} onChange={(v) => setF('authiplist', v)} placeholder="为空则只允许登录用户访问" />
      </Field>
      <Field sx={{ ...full, mb: 0 }} label="探测参数 — 全局默认（毫秒级，可在「节点管理」按链路单独覆盖）" labelStyle={{ fontSize: 14 }}
        hint="包间隔不变；系统每 10 秒汇总入库一次。Pingcount 为每分钟总包数（当前 67ms 间隔约每 10 秒 136 包）。每分钟总时长 = 间隔 × 包数 ≤ 60 秒" />
      {num('pinginterval', '探测间隔 (毫秒)', { min: 10, max: 60000 }, '相邻两个探测包之间的间隔')}
      {num('pingcount', '每轮包数', { min: 1, max: 1000 }, '每分钟向每个目标发送的总探测包数（每 10 秒汇总一次）')}
      {num('pingtimeout', '单包超时 (毫秒)', { min: 50, max: 10000 }, '超过该时间未收到应答按丢包计')}
      {num('pingsize', '探测包大小 (字节)', { min: 24, max: 1472 }, 'ICMP payload 大小，可用 512 / 1400 验证大包表现')}
      <Field label="全球延迟功能">
        <SwBlock checked={form.chinamap} onChange={(v) => setF('chinamap', v)} label="全球延迟功能" />
        <Hint inline>关闭后停止探测并隐藏菜单入口</Hint>
      </Field>
    </Box>
  );
}
