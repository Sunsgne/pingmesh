import { useState } from 'react';
import {
  AppBar, Avatar, Box, ButtonBase, Chip, Divider, Drawer, IconButton, List, ListItemButton,
  ListItemIcon, ListItemText, ListSubheader, Menu, MenuItem, Toolbar, Typography, useMediaQuery,
} from '@mui/material';
import DashboardOutlined from '@mui/icons-material/DashboardOutlined';
import GridOnOutlined from '@mui/icons-material/GridOnOutlined';
import ReplyOutlined from '@mui/icons-material/ReplyOutlined';
import HubOutlined from '@mui/icons-material/HubOutlined';
import MapOutlined from '@mui/icons-material/MapOutlined';
import BuildOutlined from '@mui/icons-material/BuildOutlined';
import NotificationsNoneOutlined from '@mui/icons-material/NotificationsNoneOutlined';
import LanOutlined from '@mui/icons-material/LanOutlined';
import SettingsOutlined from '@mui/icons-material/SettingsOutlined';
import GroupOutlined from '@mui/icons-material/GroupOutlined';
import MenuIcon from '@mui/icons-material/Menu';
import { palette } from './theme';

const DRAWER = 232;

// 未迁移的页面仍跳转到现有 jQuery 页面, 可逐页替换
const nav = [
  { group: '监控' },
  { id: 'index', href: '/next/', title: '概览', icon: <DashboardOutlined /> },
  { id: 'pingmesh', href: '/pingmesh.html', title: 'Pingmesh', icon: <GridOnOutlined /> },
  { id: 'reverse', href: '/reverse.html', title: '反向 Ping', icon: <ReplyOutlined /> },
  { id: 'topology', href: '/topology.html', title: '网络拓扑', icon: <HubOutlined /> },
  { id: 'mapping', href: '/mapping.html', title: '全球延迟', icon: <MapOutlined /> },
  { group: '运维' },
  { id: 'tools', href: '/tools.html', title: '检测工具', icon: <BuildOutlined /> },
  { id: 'alerts', href: '/alerts.html', title: '报警记录', icon: <NotificationsNoneOutlined /> },
  { group: '管理', admin: true },
  { id: 'cluster', href: '/cluster.html', title: '集群容灾', icon: <LanOutlined />, admin: true },
  { id: 'config', href: '/config.html', title: '系统配置', icon: <SettingsOutlined />, admin: true },
  { id: 'users', href: '/users.html', title: '用户管理', icon: <GroupOutlined />, admin: true },
];

function DrawerBody({ page, isAdmin, brand, ver }) {
  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', bgcolor: palette.sidebarBg, color: '#fff' }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25, px: 2.5, pt: 2.5, pb: 2 }}>
        <Box component="img" src={brand.Logo || '/assets/img/logo.png'} alt="logo"
          sx={{ width: 36, height: 36, borderRadius: '10px', boxShadow: '0 4px 12px rgba(99,102,241,.4)' }} />
        <Box sx={{ minWidth: 0 }}>
          <Typography sx={{ fontSize: 17, fontWeight: 700, lineHeight: 1.2 }}>{brand.Name || 'ZENLENET'}</Typography>
          <Typography noWrap sx={{ fontSize: 11, color: palette.sidebarText }}>{brand.Slogan || 'PingMesh 网络质量监控'}</Typography>
        </Box>
      </Box>
      <List dense sx={{ flex: 1, px: 1.5, overflowY: 'auto' }}>
        {nav.filter((n) => isAdmin || !n.admin).map((n) =>
          n.group ? (
            <ListSubheader key={n.group} disableSticky sx={{ bgcolor: 'transparent', color: '#475569', fontSize: 11, letterSpacing: '.12em', lineHeight: '32px', mt: 1 }}>
              {n.group}
            </ListSubheader>
          ) : (
            <ListItemButton key={n.id} component="a" href={n.href} selected={n.id === page}
              sx={{
                borderRadius: 2, my: 0.25, color: palette.sidebarText,
                '& .MuiListItemIcon-root': { color: 'inherit', minWidth: 30 },
                '&:hover': { bgcolor: 'rgba(255,255,255,.06)', color: '#e2e8f0' },
                '&.Mui-selected, &.Mui-selected:hover': { bgcolor: palette.primary, color: '#fff', boxShadow: '0 4px 12px rgba(79,70,229,.35)' },
              }}>
              <ListItemIcon sx={{ '& svg': { fontSize: 19 } }}>{n.icon}</ListItemIcon>
              <ListItemText primary={n.title} primaryTypographyProps={{ fontSize: 13.5 }} />
            </ListItemButton>
          ),
        )}
      </List>
      <Divider sx={{ borderColor: '#1e293b' }} />
      <Typography sx={{ px: 2.5, py: 1.75, fontSize: 11.5, color: '#475569' }}>{brand.Name || 'ZENLENET'} PingMesh v{ver}</Typography>
    </Box>
  );
}

export default function Shell({ page, title, user, config, children }) {
  const mobile = useMediaQuery('(max-width:860px)');
  const [open, setOpen] = useState(false);
  const [menu, setMenu] = useState(null);
  const isAdmin = user && user.role === 'admin';
  const brand = config.Brand || {};

  const logout = () => fetch('/api/logout.json').finally(() => { window.location.href = '/login.html'; });

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      <Drawer variant={mobile ? 'temporary' : 'permanent'} open={mobile ? open : true} onClose={() => setOpen(false)}
        sx={{ width: DRAWER, flexShrink: 0, '& .MuiDrawer-paper': { width: DRAWER, border: 'none', borderRadius: 0 } }}>
        <DrawerBody page={page} isAdmin={isAdmin} brand={brand} ver={config.Ver} />
      </Drawer>
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <AppBar position="sticky" elevation={0} color="inherit"
          sx={{ bgcolor: '#fff', borderBottom: `1px solid ${palette.border}`, borderRadius: 0,
            '&::after': { content: '""', position: 'absolute', left: 0, right: 0, bottom: -1, height: 2, background: `linear-gradient(90deg, ${palette.primary}, #8b5cf6, #ec4899)` } }}>
          <Toolbar sx={{ minHeight: '60px !important', gap: 1.75, px: { xs: 1.5, md: 3 } }}>
            {mobile && <IconButton onClick={() => setOpen(true)}><MenuIcon /></IconButton>}
            <Typography sx={{ fontSize: 16, fontWeight: 600 }}>{title}</Typography>
            <Box sx={{ flex: 1 }} />
            {!mobile && (
              <Chip size="small" label={<><b>● {config.Name}</b> <span style={{ fontFamily: 'ui-monospace, Menlo, monospace', fontWeight: 400 }}>{config.Addr}</span></>}
                sx={{ bgcolor: palette.primarySoft, color: palette.primary, height: 28, px: 0.5 }} />
            )}
            <ButtonBase onClick={(e) => setMenu(e.currentTarget)}
              sx={{ gap: 1, px: 1.25, py: 0.75, borderRadius: 2, border: `1px solid ${palette.border}`, '&:hover': { bgcolor: palette.bg } }}>
              <Avatar sx={{ width: 26, height: 26, fontSize: 12, fontWeight: 700, background: 'linear-gradient(135deg, #6366f1, #ec4899)' }}>
                {(user.username || '?').charAt(0).toUpperCase()}
              </Avatar>
              {!mobile && (
                <Box sx={{ textAlign: 'left' }}>
                  <Typography sx={{ fontSize: 13, fontWeight: 600, lineHeight: 1.2 }}>{user.username}</Typography>
                  <Typography sx={{ fontSize: 11, color: palette.text3 }}>{isAdmin ? '管理员' : '只读用户'}</Typography>
                </Box>
              )}
            </ButtonBase>
            <Menu anchorEl={menu} open={!!menu} onClose={() => setMenu(null)} anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }} transformOrigin={{ vertical: 'top', horizontal: 'right' }}>
              <MenuItem component="a" href="/index.html">旧版界面</MenuItem>
              <MenuItem onClick={logout} sx={{ color: palette.red }}>退出登录</MenuItem>
            </Menu>
          </Toolbar>
        </AppBar>
        <Box component="main" sx={{ p: { xs: 1.75, md: 3 }, flex: 1 }}>{children}</Box>
        <Box sx={{ px: 3, pb: 2.25, color: palette.text3, fontSize: 12, display: 'flex', justifyContent: 'space-between' }}>
          <span>&copy; 2026 {brand.Name || 'ZENLENET'} PingMesh · Apache-2.0</span>
          <span>当前节点: {config.Name} ({config.Addr})</span>
        </Box>
      </Box>
    </Box>
  );
}
