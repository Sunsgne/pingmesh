import { Box, Typography } from '@mui/material';
import { palette } from '../../theme';
import { Field, Input, NSelect, SwBlock, codeSx, gridCols } from './ui';

export default function AuthTab({ form, setF }) {
  const redirect = window.location.origin + '/api/oauth/microsoft/callback';
  return (
    <Box>
      <Typography component="h3" sx={{ fontSize: 14.5, fontWeight: 600, mb: '10px' }}>Microsoft 365 / Office 邮箱登录</Typography>
      <Box sx={{ color: palette.text3, mb: 2, fontSize: 13, lineHeight: 1.9 }}>
        在 Azure 门户注册应用后, 将 Client ID / Secret 填入下方并启用。回调地址需注册为:{' '}
        <Box component="code" sx={codeSx}>{redirect}</Box>
      </Box>
      <Box sx={gridCols(2)}>
        <Field sx={{ gridColumn: '1 / -1' }} label="启用 Microsoft 登录">
          <SwBlock checked={form.oauthEnabled} onChange={(v) => setF('oauthEnabled', v)} label="启用 Microsoft 登录" />
        </Field>
        <Field label="Application (Client) ID">
          <Input isMono value={form.oauthClientId} onChange={(v) => setF('oauthClientId', v)} placeholder="Azure 应用 Client ID" />
        </Field>
        <Field label="Client Secret">
          <Input isMono type="password" value={form.oauthClientSecret} onChange={(v) => setF('oauthClientSecret', v)} placeholder="留空则保持原密钥不变" />
        </Field>
        <Field label="Directory (Tenant) ID" hint={<>仅允许组织账号填 <code>organizations</code>; 单租户填租户 GUID</>}>
          <Input isMono value={form.oauthTenantId} onChange={(v) => setF('oauthTenantId', v)} placeholder="organizations / common / 租户 GUID" />
        </Field>
        <Field label="允许登录的邮箱域" hint="逗号分隔, 留空表示不限制域名">
          <Input isMono value={form.oauthDomains} onChange={(v) => setF('oauthDomains', v)} placeholder="example.com, contoso.com" />
        </Field>
        <Field label="首次登录自动创建账号">
          <SwBlock checked={form.oauthAutoCreate} onChange={(v) => setF('oauthAutoCreate', v)} label="首次登录自动创建账号" />
        </Field>
        <Field label="自动创建账号的默认角色">
          <NSelect value={form.oauthRole} onChange={(v) => setF('oauthRole', v)} options={[['viewer', '只读 (viewer)'], ['admin', '管理员 (admin)']]} />
        </Field>
      </Box>
    </Box>
  );
}
