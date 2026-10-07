import { useEffect, useRef, useState } from 'react';
import { Box, Button, GlobalStyles, Stack, TextField, Typography } from '@mui/material';
import { keyframes } from '@mui/system';
import { getJSON, postForm } from '../api';
import { palette } from '../theme';

const shake = keyframes`0%,100%{transform:translateX(0)}25%{transform:translateX(-7px)}75%{transform:translateX(7px)}`;
const DEFAULT_SLOGAN = '全网互 PING · 网络质量监控平台';

function MicrosoftLogo() {
  return (
    <svg width="18" height="18" viewBox="0 0 23 23" aria-hidden="true">
      <rect x="1" y="1" width="10" height="10" fill="#f25022" /><rect x="12" y="1" width="10" height="10" fill="#7fba00" />
      <rect x="1" y="12" width="10" height="10" fill="#00a4ef" /><rect x="12" y="12" width="10" height="10" fill="#ffb900" />
    </svg>
  );
}

function Field({ label, ...props }) {
  return (
    <Box sx={{ mb: 2 }}>
      <Typography component="label" sx={{ display: 'block', fontSize: 13, fontWeight: 600, color: palette.text2, mb: 0.75 }}>{label}</Typography>
      <TextField fullWidth {...props} />
    </Box>
  );
}

export default function Login() {
  const [meta, setMeta] = useState(null);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);
  const [shaking, setShaking] = useState(false);
  const shakeTimer = useRef(null);

  const showErr = (msg) => {
    setErr(msg);
    setShaking(true);
    clearTimeout(shakeTimer.current);
    shakeTimer.current = setTimeout(() => setShaking(false), 450);
  };

  useEffect(() => {
    const oauthErr = new URLSearchParams(window.location.search).get('oauth_error');
    if (oauthErr) showErr(decodeURIComponent(oauthErr.replace(/\+/g, ' ')));
    getJSON('/api/loginmeta.json').then((m) => {
      setMeta(m || {});
      if (m && m.brand) document.title = `登录 - ${m.brand.Name || 'ZENLENET'} PingMesh`;
    }).catch(() => {});
    return () => clearTimeout(shakeTimer.current);
  }, []);

  const submit = (e) => {
    e.preventDefault();
    if (!username || !password) { showErr('请输入用户名和密码'); return; }
    setBusy(true);
    postForm('/api/login.json', { username, password })
      .then((res) => {
        if (res.status === 'true') window.location.href = '/index.html';
        else showErr(res.info || '登录失败');
      })
      .catch(() => showErr('服务不可用，请稍后重试'))
      .finally(() => setBusy(false));
  };

  const brand = (meta && meta.brand) || {};
  return (
    <>
      <GlobalStyles styles={{ body: { background: 'radial-gradient(1000px 600px at 20% 0%, #312e81 0%, #0f172a 50%, #020617 100%)', minHeight: '100vh' } }} />
      <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Box sx={{
          width: 380, maxWidth: 'calc(100vw - 32px)', bgcolor: 'rgba(255,255,255,.98)', borderRadius: '18px',
          boxShadow: '0 25px 60px rgba(0,0,0,.5)', p: '38px 34px', animation: shaking ? `${shake} .4s` : 'none',
        }}>
          <Stack direction="row" alignItems="center" gap={1.5} sx={{ mb: 1 }}>
            <Box sx={{ width: 42, height: 42, borderRadius: '12px', overflow: 'hidden', flex: 'none', background: 'linear-gradient(135deg, #6366f1, #8b5cf6)' }}>
              <Box component="img" src={brand.Logo || '/assets/img/logo.png'} alt="logo" sx={{ width: '100%', height: '100%', display: 'block' }} />
            </Box>
            <Typography component="h1" sx={{ fontSize: 20, fontWeight: 800 }}>
              {brand.Name || 'ZENLENET'} <Box component="span" sx={{ fontWeight: 400, color: '#64748b' }}>PingMesh</Box>
            </Typography>
          </Stack>
          <Typography sx={{ color: palette.text3, fontSize: 13, mt: 0.5, mb: 3.25 }}>{brand.Slogan || DEFAULT_SLOGAN}</Typography>

          {err && <Box sx={{ bgcolor: palette.redSoft, color: '#b91c1c', fontSize: 13, borderRadius: '8px', px: 1.5, py: 1.1, mb: 1.75 }}>{err}</Box>}

          <form autoComplete="on" onSubmit={submit} noValidate>
            <Field label="用户名" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" autoFocus placeholder="请输入用户名" inputProps={{ id: 'username' }} />
            <Field label="密码" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" placeholder="请输入密码" inputProps={{ id: 'password' }} />
            <Button type="submit" variant="contained" fullWidth disabled={busy} id="login-btn" sx={{ py: '11px', fontSize: 14.5, mt: 0.75 }}>
              {busy ? '登录中...' : '登 录'}
            </Button>
          </form>

          {meta && meta.microsoft_oauth && (
            <Box sx={{ mt: 2.25 }}>
              <Stack direction="row" alignItems="center" gap={1.5} sx={{ my: 1.75, color: palette.text3, fontSize: 12 }}>
                <Box sx={{ flex: 1, height: '1px', bgcolor: palette.border }} />或<Box sx={{ flex: 1, height: '1px', bgcolor: palette.border }} />
              </Stack>
              <Button component="a" href="/api/oauth/microsoft/login" fullWidth variant="outlined" startIcon={<MicrosoftLogo />}
                sx={{ py: '11px', fontSize: 14, color: '#1f2937', borderColor: '#d1d5db', bgcolor: '#fff', '&:hover': { bgcolor: '#f9fafb', borderColor: '#d1d5db', color: '#1f2937' } }}>
                使用 Microsoft 365 登录
              </Button>
            </Box>
          )}

          {meta && meta.defaultcreds && (
            <Typography sx={{ mt: 2.25, fontSize: 12, color: palette.text3, textAlign: 'center' }}>
              首次部署默认账号 <b>admin / admin123</b>，登录后请尽快修改密码
            </Typography>
          )}
        </Box>
      </Box>
    </>
  );
}
