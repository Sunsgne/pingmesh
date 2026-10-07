import { useState } from 'react';
import {
  AppBar, Avatar, Box, Button, ButtonBase, Dialog, DialogActions, DialogContent, DialogTitle, Drawer, IconButton,
  List, ListItemButton, ListItemIcon, ListItemText, Menu, MenuItem, Stack, TextField, Toolbar, Typography, useMediaQuery,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import MenuIcon from '@mui/icons-material/Menu';
import { NAV_ICONS } from './icons';
import { useToast } from './Feedback';
import { postForm } from '../api';
import { mono, palette } from '../theme';

const DRAWER = 232;

const NAV = [
  { group: '监控' },
  { id: 'index', href: '/index.html', title: '概览', icon: 'dashboard' },
  { id: 'pingmesh', href: '/pingmesh.html', title: 'Pingmesh', icon: 'mesh' },
  { id: 'reverse', href: '/reverse.html', title: '反向 Ping', icon: 'reverse' },
  { id: 'topology', href: '/topology.html', title: '网络拓扑', icon: 'topo' },
  { id: 'mapping', href: '/mapping.html', title: '全球延迟', icon: 'map' },
  { group: '运维' },
  { id: 'tools', href: '/tools.html', title: '检测工具', icon: 'tools' },
  { id: 'alerts', href: '/alerts.html', title: '报警记录', icon: 'alert' },
  { group: '管理', admin: true },
  { id: 'cluster', href: '/cluster.html', title: '集群容灾', icon: 'cluster', admin: true },
  { id: 'config', href: '/config.html', title: '系统配置', icon: 'config', admin: true },
  { id: 'users', href: '/users.html', title: '用户管理', icon: 'users', admin: true },
];

function PasswordDialog({ open, onClose }) {
  const toast = useToast();
  const [f, setF] = useState({ old: '', pw: '', pw2: '' });
  const [busy, setBusy] = useState(false);
  const set = (k) => (e) => setF((x) => ({ ...x, [k]: e.target.value }));
  const save = () => {
    if (f.pw.length < 6) { toast('新密码至少6个字符', 'err'); return; }
    if (f.pw !== f.pw2) { toast('两次输入的新密码不一致', 'err'); return; }
    setBusy(true);
    postForm('/api/user/passwd.json', { oldpassword: f.old, password: f.pw })
      .then((res) => {
        if (res.status === 'true') {
          toast('密码修改成功', 'ok');
          setF({ old: '', pw: '', pw2: '' });
          onClose();
        } else toast(res.info || '修改失败', 'err');
      })
      .catch(() => toast('修改失败', 'err'))
      .finally(() => setBusy(false));
  };
  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm" PaperProps={{ sx: { maxWidth: 580 } }}>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', borderBottom: `1px solid ${palette.border}` }}>
        <Box component="span" sx={{ flex: 1 }}>修改密码</Box>
        <IconButton size="small" onClick={onClose}><CloseIcon fontSize="small" /></IconButton>
      </DialogTitle>
      <DialogContent>
        <Stack gap={2} sx={{ pt: 2.5 }}>
          <TextField label="原密码" type="password" autoComplete="current-password" value={f.old} onChange={set('old')} autoFocus />
          <TextField label="新密码" type="password" autoComplete="new-password" value={f.pw} onChange={set('pw')} helperText="至少 6 个字符" />
          <TextField label="确认新密码" type="password" autoComplete="new-password" value={f.pw2} onChange={set('pw2')}
            onKeyDown={(e) => e.key === 'Enter' && save()} />
        </Stack>
      </DialogContent>
      <DialogActions sx={{ px: 2.5, py: 1.75, borderTop: `1px solid ${palette.border}` }}>
        <Button variant="outlined" onClick={onClose}>取消</Button>
        <Button variant="contained" onClick={save} disabled={busy}>保存</Button>
      </DialogActions>
    </Dialog>
  );
}

function Sidebar({ page, isAdmin, cfg, onNavigate }) {
  const brand = cfg.Brand || {};
  const name = brand.Name || 'ZENLENET';
  const slogan = brand.Slogan || 'PingMesh 网络质量监控';
  const hideMapping = cfg.Base && cfg.Base.Chinamap === 0;
  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', bgcolor: palette.sidebarBg, boxShadow: 'inset -1px 0 0 rgba(255,255,255,.03)' }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25, px: 2.5, pt: 2.5, pb: 2, color: '#fff' }}>
        <Box sx={{ width: 36, height: 36, borderRadius: '10px', overflow: 'hidden', flex: 'none', boxShadow: '0 4px 12px rgba(99,102,241,.4)', background: 'linear-gradient(135deg, #6366f1, #8b5cf6)' }}>
          <Box component="img" src={brand.Logo || '/assets/img/logo.png'} alt="logo" sx={{ width: '100%', height: '100%', display: 'block' }} />
        </Box>
        <Box sx={{ minWidth: 0 }}>
          <Typography sx={{ fontSize: 17, fontWeight: 700, letterSpacing: '.2px', lineHeight: 1.25 }}>{name}</Typography>
          <Typography sx={{ fontSize: 11, color: palette.sidebarText, lineHeight: 1.35 }}>{slogan}</Typography>
        </Box>
      </Box>
      <Box component="nav" sx={{ flex: 1, px: 1.5, py: 1, overflowY: 'auto' }}>
        <List disablePadding>
          {NAV.filter((n) => (isAdmin || !n.admin) && !(hideMapping && n.id === 'mapping')).map((n) => (n.group ? (
            <Typography key={n.group} sx={{ fontSize: 11, lineHeight: 1.2, letterSpacing: '.12em', color: '#475569', px: 1.25, pt: 2, pb: 0.75 }}>{n.group}</Typography>
          ) : (
            <ListItemButton key={n.id} component="a" href={n.href} selected={n.id === page} onClick={onNavigate}
              sx={{
                borderRadius: '8px', my: '2px', px: 1.25, py: '9px', gap: 1.25, color: palette.sidebarText, fontSize: 13.5,
                transition: 'background .15s, color .15s',
                '&:hover': { bgcolor: 'rgba(255,255,255,.06)', color: '#e2e8f0' },
                '&.Mui-selected, &.Mui-selected:hover': { bgcolor: palette.primary, color: '#fff', boxShadow: '0 4px 12px rgba(79,70,229,.35)' },
              }}>
              <ListItemIcon sx={{ minWidth: 0, color: 'inherit', '& svg': { width: 17, height: 17 } }}>{NAV_ICONS[n.icon]}</ListItemIcon>
              <ListItemText primary={n.title} primaryTypographyProps={{ fontSize: 13.5, lineHeight: '17px' }} sx={{ my: 0 }} />
            </ListItemButton>
          )))}
        </List>
      </Box>
      <Box sx={{ px: 2.5, py: 1.75, borderTop: '1px solid #1e293b', color: '#475569', fontSize: 11.5 }}>
        {name} PingMesh v{cfg.Ver}
      </Box>
    </Box>
  );
}

export default function Shell({ page, title, user, config, children }) {
  const compact = useMediaQuery('(max-width:900px)');
  const [navOpen, setNavOpen] = useState(false);
  const [menu, setMenu] = useState(null);
  const [pwOpen, setPwOpen] = useState(false);
  const isAdmin = user.role === 'admin';
  const syncTime = config.Mode && config.Mode.Type === 'cloud' ? `云端配置 · 最后同步 ${config.Mode.LastSuccTime || '-'}` : '';

  const logout = () => {
    fetch('/api/logout.json', { credentials: 'same-origin' }).finally(() => { window.location.href = '/login.html'; });
  };

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      <Drawer variant={compact ? 'temporary' : 'permanent'} open={compact ? navOpen : true} onClose={() => setNavOpen(false)}
        sx={{ width: compact ? 0 : DRAWER, flexShrink: 0, '& .MuiDrawer-paper': { width: DRAWER, border: 'none', borderRadius: 0, bgcolor: palette.sidebarBg } }}>
        <Sidebar page={page} isAdmin={isAdmin} cfg={config} onNavigate={() => setNavOpen(false)} />
      </Drawer>
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <AppBar position="sticky" elevation={0} color="inherit" sx={{
          bgcolor: '#fff', borderRadius: 0, boxShadow: `0 1px 0 0 ${palette.border}`,
          '&::after': { content: '""', position: 'absolute', left: 0, right: 0, bottom: -1, height: 2, opacity: 0.9, background: `linear-gradient(90deg, ${palette.primary}, #8b5cf6, #ec4899)` },
        }}>
          <Toolbar disableGutters sx={{ minHeight: `${compact ? 56 : 60}px !important`, gap: compact ? 1 : 1.75, px: compact ? 1.5 : 3 }}>
            {compact && (
              <IconButton size="small" onClick={() => setNavOpen(true)} aria-label="菜单"
                sx={{ border: `1px solid ${palette.border}`, borderRadius: '8px', bgcolor: '#fff', p: '5px 7px', '& svg': { fontSize: 18 } }}>
                <MenuIcon />
              </IconButton>
            )}
            <Typography component="div" sx={{ fontSize: compact ? 15 : 16, fontWeight: 600, whiteSpace: 'nowrap' }}>{title}</Typography>
            <Box sx={{ flex: 1 }} />
            {!compact && syncTime && <Typography sx={{ color: palette.text3, fontSize: 12 }}>{syncTime}</Typography>}
            <Box title="当前节点" sx={{
              display: { xs: 'none', sm: 'inline-flex' }, alignItems: 'center', gap: 0.75, bgcolor: palette.primarySoft, color: palette.primary,
              borderRadius: 999, px: 1.5, py: '5px', fontSize: 12.5, fontWeight: 600, whiteSpace: 'nowrap', maxWidth: compact ? 160 : 'none', overflow: 'hidden',
            }}>
              ●&nbsp;{config.Name} <Box component="span" sx={{ fontFamily: mono, fontWeight: 400 }}>{config.Addr}</Box>
            </Box>
            <ButtonBase onClick={(e) => setMenu(e.currentTarget)} sx={{
              gap: 1, px: compact ? 0.5 : 1.25, py: 0.75, borderRadius: '8px', border: compact ? 'none' : `1px solid ${palette.border}`, bgcolor: compact ? 'transparent' : '#fff',
              '&:hover': { bgcolor: palette.bg },
            }}>
              <Avatar sx={{ width: 26, height: 26, fontSize: 12, fontWeight: 700, background: 'linear-gradient(135deg, #6366f1, #ec4899)' }}>
                {(user.username || '?').substring(0, 1).toUpperCase()}
              </Avatar>
              {!compact && (
                <Box sx={{ textAlign: 'left' }}>
                  <Typography sx={{ fontSize: 13, fontWeight: 600, lineHeight: 1.3 }}>{user.username}</Typography>
                  <Typography sx={{ fontSize: 11, color: palette.text3, lineHeight: 1.3 }}>{isAdmin ? '管理员' : '只读用户'}</Typography>
                </Box>
              )}
            </ButtonBase>
            <Menu anchorEl={menu} open={!!menu} onClose={() => setMenu(null)}
              anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }} transformOrigin={{ vertical: 'top', horizontal: 'right' }}
              slotProps={{ paper: { sx: { mt: 0.75, minWidth: 170, borderRadius: '10px', border: `1px solid ${palette.border}` } } }}>
              <MenuItem sx={{ fontSize: 13, color: palette.text2 }} onClick={() => { setMenu(null); setPwOpen(true); }}>修改密码</MenuItem>
              <MenuItem sx={{ fontSize: 13, color: palette.red }} onClick={logout}>退出登录</MenuItem>
            </Menu>
          </Toolbar>
        </AppBar>
        <Box component="main" sx={{ p: compact ? 1.75 : 3, flex: 1, minWidth: 0 }}>{children}</Box>
        <Box sx={{ px: compact ? 1.75 : 3, pb: 2.25, color: palette.text3, fontSize: 12, display: 'flex', justifyContent: 'space-between', gap: 1, flexWrap: 'wrap' }}>
          <span>&copy; 2026 ZENLENET PingMesh · Apache-2.0</span>
          <span>当前节点: {config.Name} ({config.Addr})</span>
        </Box>
      </Box>
      <PasswordDialog open={pwOpen} onClose={() => setPwOpen(false)} />
    </Box>
  );
}
