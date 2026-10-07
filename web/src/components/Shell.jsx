import { useState } from 'react';
import {
  AppBar, Avatar, Box, Button, ButtonBase, Dialog, DialogActions, DialogContent, DialogTitle, Drawer, IconButton,
  List, ListItemButton, ListItemIcon, ListItemText, Menu, MenuItem, Stack, TextField, Toolbar, Tooltip, Typography, useMediaQuery,
} from '@mui/material';
import { keyframes } from '@mui/system';
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import CloseIcon from '@mui/icons-material/Close';
import MenuIcon from '@mui/icons-material/Menu';
import { NAV_ICONS } from './icons';
import { useToast } from './Feedback';
import { loadPref, postForm, savePref } from '../api';
import { mono, palette } from '../theme';

const DRAWER = 232;
const RAIL = 72;
const CONTENT_MAX = 1760;
const fadeIn = keyframes`from{opacity:0;transform:translateY(4px)}to{opacity:1;transform:none}`;

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

function Sidebar({ page, isAdmin, cfg, onNavigate, collapsed = false, onToggle }) {
  const brand = cfg.Brand || {};
  const name = brand.Name || 'ZENLENET';
  const slogan = brand.Slogan || 'PingMesh 网络质量监控';
  const hideMapping = cfg.Base && cfg.Base.Chinamap === 0;
  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', bgcolor: palette.sidebarBg, boxShadow: 'inset -1px 0 0 rgba(255,255,255,.03)' }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25, px: collapsed ? 0 : 2.5, pt: 2.5, pb: 2, color: '#fff', justifyContent: collapsed ? 'center' : 'flex-start' }}>
        <Box sx={{ width: 36, height: 36, borderRadius: '10px', overflow: 'hidden', flex: 'none', boxShadow: '0 4px 12px rgba(99,102,241,.4)', background: 'linear-gradient(135deg, #6366f1, #8b5cf6)' }}>
          <Box component="img" src={brand.Logo || '/assets/img/logo.png'} alt="logo" sx={{ width: '100%', height: '100%', display: 'block' }} />
        </Box>
        {!collapsed && (
          <Box sx={{ minWidth: 0 }}>
            <Typography noWrap sx={{ fontSize: 17, fontWeight: 700, letterSpacing: '.2px', lineHeight: 1.25 }}>{name}</Typography>
            <Typography sx={{ fontSize: 11, color: palette.sidebarText, lineHeight: 1.35 }}>{slogan}</Typography>
          </Box>
        )}
      </Box>
      <Box component="nav" sx={{ flex: 1, px: collapsed ? 1.25 : 1.5, py: 1, overflowY: 'auto', overflowX: 'hidden', scrollbarWidth: 'thin' }}>
        <List disablePadding>
          {NAV.filter((n) => (isAdmin || !n.admin) && !(hideMapping && n.id === 'mapping')).map((n) => (n.group ? (
            collapsed
              ? <Box key={n.group} sx={{ height: '1px', bgcolor: '#1e293b', mx: 1, my: 1.5 }} />
              : <Typography key={n.group} sx={{ fontSize: 11, lineHeight: 1.2, letterSpacing: '.12em', color: '#475569', px: 1.25, pt: 2, pb: 0.75 }}>{n.group}</Typography>
          ) : (
            <Tooltip key={n.id} title={collapsed ? n.title : ''} placement="right" arrow disableInteractive>
            <ListItemButton component="a" href={n.href} selected={n.id === page} onClick={onNavigate} aria-label={n.title}
              sx={{
                borderRadius: '8px', my: '2px', px: 1.25, py: '9px', gap: 1.25, color: palette.sidebarText, fontSize: 13.5,
                justifyContent: collapsed ? 'center' : 'flex-start',
                '&.Mui-focusVisible': { outline: `2px solid ${palette.primary2}`, outlineOffset: '1px' },
                transition: 'background .15s, color .15s',
                '&:hover': { bgcolor: 'rgba(255,255,255,.06)', color: '#e2e8f0' },
                '&.Mui-selected, &.Mui-selected:hover': { bgcolor: palette.primary, color: '#fff', boxShadow: '0 4px 12px rgba(79,70,229,.35)' },
              }}>
              <ListItemIcon sx={{ minWidth: 0, color: 'inherit', '& svg': { width: 17, height: 17 } }}>{NAV_ICONS[n.icon]}</ListItemIcon>
              {!collapsed && <ListItemText primary={n.title} primaryTypographyProps={{ fontSize: 13.5, lineHeight: '17px', noWrap: true }} sx={{ my: 0 }} />}
            </ListItemButton>
            </Tooltip>
          )))}
        </List>
      </Box>
      <Box sx={{ px: collapsed ? 0 : 2.5, py: 1.25, borderTop: '1px solid #1e293b', color: '#475569', fontSize: 11.5, display: 'flex', alignItems: 'center', gap: 1, justifyContent: collapsed ? 'center' : 'space-between' }}>
        {!collapsed && <Box component="span" sx={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{name} PingMesh v{cfg.Ver}</Box>}
        {onToggle && (
          <Tooltip title={collapsed ? '展开侧栏' : '收起侧栏'} placement="right">
            <IconButton size="small" onClick={onToggle} aria-label={collapsed ? '展开侧栏' : '收起侧栏'}
              sx={{ color: palette.sidebarText, '&:hover': { color: '#e2e8f0', bgcolor: 'rgba(255,255,255,.06)' } }}>
              {collapsed ? <ChevronRightIcon fontSize="small" /> : <ChevronLeftIcon fontSize="small" />}
            </IconButton>
          </Tooltip>
        )}
      </Box>
    </Box>
  );
}

export default function Shell({ page, title, user, config, children }) {
  const compact = useMediaQuery('(max-width:900px)');
  const [navOpen, setNavOpen] = useState(false);
  const [menu, setMenu] = useState(null);
  const [pwOpen, setPwOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(() => loadPref('sp-nav-collapsed', false) === true);
  const isAdmin = user.role === 'admin';
  const rail = !compact && collapsed;
  const navW = rail ? RAIL : DRAWER;
  const toggleRail = () => setCollapsed((c) => { savePref('sp-nav-collapsed', !c); return !c; });
  const syncTime = config.Mode && config.Mode.Type === 'cloud' ? `云端配置 · 最后同步 ${config.Mode.LastSuccTime || '-'}` : '';

  const logout = () => {
    fetch('/api/logout.json', { credentials: 'same-origin' }).finally(() => { window.location.href = '/login.html'; });
  };

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      <Drawer variant={compact ? 'temporary' : 'permanent'} open={compact ? navOpen : true} onClose={() => setNavOpen(false)}
        sx={{
          width: compact ? 0 : navW, flexShrink: 0, transition: 'width .2s ease',
          '& .MuiDrawer-paper': { width: compact ? DRAWER : navW, border: 'none', borderRadius: 0, bgcolor: palette.sidebarBg, overflowX: 'hidden', transition: 'width .2s ease' },
        }}>
        <Sidebar page={page} isAdmin={isAdmin} cfg={config} onNavigate={() => setNavOpen(false)} collapsed={rail} onToggle={compact ? null : toggleRail} />
      </Drawer>
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <AppBar position="sticky" elevation={0} color="inherit" sx={{
          bgcolor: 'rgba(255,255,255,.82)', backdropFilter: 'saturate(180%) blur(12px)', WebkitBackdropFilter: 'saturate(180%) blur(12px)',
          borderRadius: 0, boxShadow: `0 1px 0 0 ${palette.border}`,
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
        <Box component="main" sx={{ p: 'clamp(14px, 1.8vw, 28px)', flex: 1, minWidth: 0 }}>
          <Box sx={{ maxWidth: CONTENT_MAX, mx: 'auto', animation: `${fadeIn} .22s ease-out`, '@media (prefers-reduced-motion: reduce)': { animation: 'none' } }}>{children}</Box>
        </Box>
        <Box sx={{ px: 'clamp(14px, 1.8vw, 28px)', pb: 2.25, color: palette.text3, fontSize: 12, display: 'flex', justifyContent: 'space-between', gap: 1, flexWrap: 'wrap', maxWidth: CONTENT_MAX + 56, width: '100%', mx: 'auto' }}>
          <span>&copy; 2026 ZENLENET PingMesh · Apache-2.0</span>
          <span>当前节点: {config.Name} ({config.Addr})</span>
        </Box>
      </Box>
      <PasswordDialog open={pwOpen} onClose={() => setPwOpen(false)} />
    </Box>
  );
}
