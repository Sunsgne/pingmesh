import { useState } from 'react';
import { Box, Table, TableBody, TableCell, TableHead, TableRow, Typography } from '@mui/material';
import { Badge } from '../../components/ui';
import { palette } from '../../theme';
import { CH_FIELDS, CH_TYPES, CH_TYPE_NAMES, selVal, trim, txtVal } from './logic';
import { postJq } from './post';
import { Btn, Field, Input, Modal, NSelect, Spacer, SwBlock, flexRow, plainSx, tableSx } from './ui';

const MAIL_PRESETS = [
  ['', '手动配置...'],
  ['smtp.qq.com:465|使用「授权码」而非登录密码（QQ邮箱设置 → 账户 → 开启SMTP服务获取）', 'QQ 邮箱'],
  ['smtp.163.com:465|使用「客户端授权密码」（163设置 → POP3/SMTP → 开启服务获取）', '网易 163 邮箱'],
  ['smtp.exmail.qq.com:465|使用成员账号密码；管理员需在后台开启 SMTP', '腾讯企业邮'],
  ['smtp.qiye.aliyun.com:465|使用成员账号密码', '阿里企业邮'],
  ['smtp.gmail.com:587|需开启两步验证并使用「应用专用密码」(App Password)', 'Gmail'],
  ['smtp.office365.com:587|Outlook / Microsoft 365 账号；部分租户需管理员启用 SMTP AUTH', 'Outlook / Microsoft 365'],
  ['smtp.zoho.com:465|使用 Zoho 账号或应用专用密码', 'Zoho Mail'],
  ['smtp.sendgrid.net:587|发件账号固定填 apikey，密码填 SendGrid API Key', 'SendGrid'],
  ['email-smtp.us-east-1.amazonaws.com:587|使用 SES SMTP 凭证；按实际区域修改服务器地址', 'Amazon SES'],
];
const REMIND = [
  ['0', '关闭（只在故障/恢复时各通知一次）'], ['30', '每 30 分钟提醒一次'], ['60', '每 1 小时提醒一次（默认）'],
  ['360', '每 6 小时提醒一次'], ['1440', '每 24 小时提醒一次'],
];
const TYPE_VALUES = CH_TYPES.map((t) => t[0]);

function ChannelModal({ dlg, setDlg, onSave, testChannel, testing }) {
  if (!dlg) return <Modal open={false} onClose={() => {}} title="" />;
  const close = () => setDlg({ ...dlg, open: false });
  const fields = CH_FIELDS[dlg.fieldsType] || [];
  const read = () => {
    const params = {};
    fields.forEach((f) => { params[f.k] = trim(dlg.params[f.k] || ''); });
    return { Type: dlg.type, Name: trim(dlg.name), Enabled: dlg.enabled, Params: params };
  };
  const save = () => onSave(read());
  return (
    <Modal open={dlg.open} onClose={close} title={dlg.index !== null ? '编辑告警通道' : '添加告警通道'}
      actions={<>
        <Btn onClick={close}>取消</Btn>
        <Btn disabled={testing.modal} onClick={() => testChannel(read(), 'modal')}>{testing.modal ? '发送中...' : '发送测试'}</Btn>
        <Btn primary onClick={save}>确定</Btn>
      </>}>
      <Field label="通道类型">
        <NSelect value={dlg.type} onChange={(v) => setDlg({ ...dlg, type: v, fieldsType: v, params: {} })} options={CH_TYPES} />
      </Field>
      <Field label="通道名称"><Input value={dlg.name} onChange={(v) => setDlg({ ...dlg, name: v })} placeholder="如: 运维群" /></Field>
      {fields.map((f) => (
        <Field key={dlg.fieldsType + f.k} label={f.l}>
          <Input isMono value={dlg.params[f.k] || ''} placeholder={f.ph || ''} onChange={(v) => setDlg({ ...dlg, params: { ...dlg.params, [f.k]: v } })} />
        </Field>
      ))}
      <Field label="启用"><SwBlock mt={0} checked={dlg.enabled} onChange={(v) => setDlg({ ...dlg, enabled: v })} label="启用" /></Field>
    </Modal>
  );
}

export default function AlertTab({ cfg, form, setF, toast, confirm, onChanged }) {
  const [preset, setPreset] = useState('');
  const [mailTesting, setMailTesting] = useState(false);
  const [testing, setTesting] = useState({});
  const [dlg, setDlg] = useState(null);
  const channels = cfg.Channels || [];

  const onPreset = (v) => {
    setPreset(v);
    if (!v) return;
    setF('emailhost', v.split('|')[0]);
  };
  const presetHint = preset ? preset.split('|')[1] || '' : '';

  const mailTest = () => {
    setMailTesting(true);
    postJq('/api/sendmailtest.json', {
      EmailHost: form.emailhost, SendEmailAccount: form.emailaccount, SendEmailPassword: form.emailpassword, RevcEmailList: form.emaillist,
    }).then((res) => {
      if (res.status === 'true') toast('测试邮件已发送', 'ok');
      else toast(res.info || '发送失败', 'err');
    }).catch(() => {}).finally(() => setMailTesting(false));
  };

  const testChannel = (ch, key) => {
    setTesting((m) => ({ ...m, [key]: true }));
    postJq('/api/alerttest.json', { channel: JSON.stringify(ch) }).then((res) => {
      if (res.status === 'true') toast('测试消息已发送，请到对应群/会话确认', 'ok');
      else toast('发送失败: ' + (res.info || ''), 'err');
    }).catch(() => toast('测试请求失败', 'err')).finally(() => setTesting((m) => ({ ...m, [key]: false })));
  };

  const openCh = (i) => {
    const idx = typeof i === 'number' ? i : null;
    const ch = idx !== null ? channels[idx] : { Type: 'dingtalk', Name: '', Enabled: true, Params: {} };
    const params = {};
    (CH_FIELDS[ch.Type] || []).forEach((f) => { params[f.k] = txtVal((ch.Params || {})[f.k] || ''); });
    setDlg({ open: true, index: idx, type: selVal(ch.Type, TYPE_VALUES), fieldsType: ch.Type, name: txtVal(ch.Name), enabled: !!ch.Enabled, params });
  };

  const saveCh = (ch) => {
    if (!ch.Name) { toast('请输入通道名称', 'err'); return; }
    if (ch.Type === 'telegram') {
      if (!ch.Params.Token || !ch.Params.ChatId) { toast('Telegram 需要 Token 与 ChatId', 'err'); return; }
    } else if (!ch.Params.Url) {
      toast('请填写 Url', 'err'); return;
    }
    if (dlg.index !== null) cfg.Channels[dlg.index] = ch;
    else cfg.Channels.push(ch);
    setDlg({ ...dlg, open: false });
    onChanged();
    toast('已更新工作副本，记得点击「保存配置」', 'ok');
  };

  const delCh = async (i) => {
    const c = cfg.Channels[i];
    if (!(await confirm('删除通道', '确定删除通道「' + (c.Name || CH_TYPE_NAMES[c.Type]) + '」吗？'))) return;
    cfg.Channels.splice(i, 1);
    onChanged();
    toast('已从工作副本删除，记得点击「保存配置」', 'ok');
  };

  return (
    <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: '380px 1fr', alignItems: 'start', '@media (max-width:900px)': { gridTemplateColumns: '1fr' } }}>
      <Box sx={{ minWidth: 0 }}>
        <Typography component="h3" sx={{ fontSize: 14.5, fontWeight: 600, mb: '14px' }}>邮件通知 (SMTP)</Typography>
        <Field label="邮件服务商（一键填充服务器配置）" hint={presetHint ? '💡 ' + presetHint : ''}>
          <NSelect value={preset} onChange={onPreset} options={MAIL_PRESETS} />
        </Field>
        <Field label="SMTP 服务器" hint="465 端口自动使用 SSL/TLS 加密，587/25 端口自动协商 STARTTLS">
          <Input value={form.emailhost} onChange={(v) => setF('emailhost', v)} placeholder="smtp.example.com:465" />
        </Field>
        <Field label="发件邮箱"><Input value={form.emailaccount} onChange={(v) => setF('emailaccount', v)} placeholder="alert@example.com" /></Field>
        <Field label="发件邮箱密码">
          <Input type="password" value={form.emailpassword} onChange={(v) => setF('emailpassword', v)} inputProps={{ autoComplete: 'new-password' }} />
        </Field>
        <Field label="收件人列表" hint="多个收件人用英文分号分隔">
          <Input value={form.emaillist} onChange={(v) => setF('emaillist', v)} placeholder="a@x.com;b@y.com" />
        </Field>
        <Btn sm disabled={mailTesting} onClick={mailTest}>{mailTesting ? '发送中...' : '发送测试邮件'}</Btn>
        <Field sx={{ mt: 2 }} label="持续故障重复提醒"
          hint={<>故障持续未恢复时定期再提醒；<b>确认后暂停提醒 24 小时</b>，到期仍未恢复则继续提醒（恢复后自动重置）。屏蔽到期仍异常会自动补一条通知</>}>
          <NSelect value={form.remindmin} onChange={(v) => setF('remindmin', v)} options={REMIND} />
        </Field>
      </Box>
      <Box sx={{ minWidth: 0 }}>
        <Box sx={{ ...flexRow, mb: 2 }}>
          <Typography component="h3" sx={{ fontSize: 14.5, fontWeight: 600 }}>告警通道（触发与恢复均通知）</Typography>
          <Spacer />
          <Btn sm primary onClick={() => openCh(null)}>+ 添加通道</Btn>
        </Box>
        <Box sx={{ overflowX: 'auto' }}>
          <Table sx={tableSx}>
            <TableHead><TableRow><TableCell>类型</TableCell><TableCell>名称</TableCell><TableCell>状态</TableCell><TableCell sx={{ width: 200 }}>操作</TableCell></TableRow></TableHead>
            <TableBody>
              {channels.length === 0 && <TableRow><TableCell colSpan={4} sx={{ color: palette.text3 }}>尚未配置告警通道</TableCell></TableRow>}
              {channels.map((ch, i) => (
                // eslint-disable-next-line react/no-array-index-key
                <TableRow key={i}>
                  <TableCell><Badge tone="indigo">{CH_TYPE_NAMES[ch.Type] || ch.Type}</Badge></TableCell>
                  <TableCell><b>{ch.Name || '-'}</b></TableCell>
                  <TableCell>{ch.Enabled ? <Badge tone="green" dot>启用</Badge> : <Badge tone="gray">停用</Badge>}</TableCell>
                  <TableCell sx={{ whiteSpace: 'nowrap' }}>
                    <Btn sm disabled={testing[i]} onClick={() => testChannel(cfg.Channels[i], i)}>{testing[i] ? '发送中...' : '测试'}</Btn>{' '}
                    <Btn sm onClick={() => openCh(i)}>编辑</Btn>{' '}
                    <Btn sm danger onClick={() => delCh(i)}>删除</Btn>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Box>
        <Box sx={plainSx}>支持钉钉 / 企业微信 / 飞书 / Telegram / Slack / Discord / 通用 Webhook，可同时启用多个</Box>
      </Box>
      <ChannelModal dlg={dlg} setDlg={setDlg} onSave={saveCh} testChannel={testChannel} testing={testing} />
    </Box>
  );
}
