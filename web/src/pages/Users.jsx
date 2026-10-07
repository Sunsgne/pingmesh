import { useEffect, useState } from 'react';
import {
  Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, MenuItem, Table, TableBody, TableCell,
  TableHead, TableRow, TextField, Typography,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { Badge, EmptyState, Hint, Panel, ResponsiveTable } from '../components/ui';
import { useConfirm, useToast } from '../components/Feedback';
import { getJSON, postForm } from '../api';
import { palette } from '../theme';

const RoleBadge = ({ role }) => (role === 'admin' ? <Badge tone="indigo">管理员</Badge> : <Badge tone="gray">只读用户</Badge>);

const smBtn = { minWidth: 0, py: '5px', lineHeight: 'normal' };
const fieldLabel = { display: 'block', fontSize: 13, fontWeight: 600, color: palette.text2, mb: 0.75 };

function resInfo(e) {
  try { return JSON.parse(e.body).info; } catch (x) { return ''; }
}

function UserDialog({ open, editing, onClose, onSaved }) {
  const toast = useToast();
  const [name, setName] = useState('');
  const [pass, setPass] = useState('');
  const [role, setRole] = useState('viewer');

  useEffect(() => {
    if (!open) return;
    setName(editing ? editing.username : '');
    setPass('');
    setRole(editing ? editing.role : 'viewer');
  }, [open, editing]);

  const save = () => {
    const username = name.trim();
    if (!username) { toast('请输入用户名', 'err'); return; }
    if (!editing && pass.length < 6) { toast('密码至少6个字符', 'err'); return; }
    postForm('/api/user/save.json', { username, password: pass, role })
      .then((res) => {
        if (res.status === 'true') {
          toast('保存成功', 'ok');
          onClose();
          onSaved();
        } else {
          toast(res.info || '保存失败', 'err');
        }
      })
      .catch((e) => toast(resInfo(e) || '保存失败', 'err'));
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth={false} PaperProps={{ sx: { maxWidth: 580 } }}>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', borderBottom: `1px solid ${palette.border}`, py: 2, px: 2.5 }}>
        <Box component="span" sx={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {editing ? `编辑用户 - ${editing.username}` : '新建用户'}
        </Box>
        <IconButton size="small" onClick={onClose} aria-label="关闭"><CloseIcon fontSize="small" /></IconButton>
      </DialogTitle>
      <Box component="form" noValidate onSubmit={(e) => { e.preventDefault(); save(); }}>
        <DialogContent sx={{ p: '20px !important', '@media (max-width:900px)': { p: '14px !important' } }}>
          <Box sx={{ mb: 2 }}>
            <Typography component="label" htmlFor="u-name" sx={fieldLabel}>用户名</Typography>
            <TextField fullWidth id="u-name" value={name} placeholder="2 个字符以上" disabled={!!editing}
              sx={{ '& .Mui-disabled': { WebkitTextFillColor: palette.text2 } }}
              autoFocus={!editing} onChange={(e) => setName(e.target.value)} />
          </Box>
          <Box sx={{ mb: 2 }}>
            <Typography component="label" htmlFor="u-pass" sx={fieldLabel}>密码</Typography>
            <TextField fullWidth id="u-pass" type="password" value={pass} placeholder="6 个字符以上" autoFocus={!!editing}
              inputProps={{ autoComplete: 'new-password' }} onChange={(e) => setPass(e.target.value)} />
            {editing && <Hint sx={{ mt: '5px' }}>留空表示不修改密码</Hint>}
          </Box>
          <Box sx={{ mb: 2 }}>
            <Typography component="label" htmlFor="u-role" sx={fieldLabel}>角色</Typography>
            <TextField select fullWidth value={role} onChange={(e) => setRole(e.target.value)} inputProps={{ id: 'u-role' }}>
              <MenuItem value="viewer" sx={{ fontSize: 13.5 }}>只读用户</MenuItem>
              <MenuItem value="admin" sx={{ fontSize: 13.5 }}>管理员</MenuItem>
            </TextField>
          </Box>
        </DialogContent>
        <DialogActions sx={{ px: 2.5, py: 1.75, borderTop: `1px solid ${palette.border}`, gap: 1.25, '& > :not(style) ~ :not(style)': { ml: 0 } }}>
          <Button variant="outlined" color="inherit" onClick={onClose}>取消</Button>
          <Button variant="contained" type="submit" id="u-save">保存</Button>
        </DialogActions>
      </Box>
    </Dialog>
  );
}

export default function Users({ user, isAdmin }) {
  const toast = useToast();
  const confirm = useConfirm();
  const [users, setUsers] = useState([]);
  const [dialog, setDialog] = useState({ open: false, editing: null });

  const load = () => {
    getJSON('/api/user/list.json').then((res) => setUsers(res.users || [])).catch(() => {});
  };
  useEffect(() => { if (isAdmin) load(); }, [isAdmin]); // eslint-disable-line react-hooks/exhaustive-deps

  if (!isAdmin) return <EmptyState icon="🔒">没有权限访问该页面</EmptyState>;

  const remove = async (name) => {
    if (!(await confirm('删除用户', '确定删除用户「' + name + '」吗？该操作不可恢复。'))) return;
    postForm('/api/user/delete.json', { username: name })
      .then((res) => {
        if (res.status === 'true') { toast('已删除', 'ok'); load(); } else toast(res.info || '删除失败', 'err');
      })
      .catch((e) => toast(resInfo(e) || '删除失败', 'err'));
  };

  return (
    <>
      <Panel title="用户管理" sub="管理员可以创建用户、重置密码、调整角色" flat
        actions={<Button size="small" variant="contained" sx={smBtn} onClick={() => setDialog({ open: true, editing: null })}>+ 新建用户</Button>}>
        <ResponsiveTable sx={{ '@media (max-width:600px)': { px: '10px' } }}>
          <Table sx={{ '& tbody tr:last-child td': { borderBottom: 'none' } }}>
            <TableHead>
              <TableRow>
                <TableCell>用户名</TableCell>
                <TableCell>角色</TableCell>
                <TableCell>创建时间</TableCell>
                <TableCell>最后登录</TableCell>
                <TableCell sx={{ width: 200 }}>操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {users.map((u) => {
                const self = u.username === user.username;
                return (
                  <TableRow key={u.username} hover>
                    <TableCell>
                      <b>{u.username}</b>
                      {self && <Box component="span" sx={{ color: palette.text3 }}> (当前)</Box>}
                    </TableCell>
                    <TableCell><RoleBadge role={u.role} /></TableCell>
                    <TableCell sx={{ color: palette.text3 }}>{u.created_at || '-'}</TableCell>
                    <TableCell sx={{ color: palette.text3 }}>{u.last_login || '从未登录'}</TableCell>
                    <TableCell>
                      <Button size="small" variant="outlined" sx={smBtn} onClick={() => setDialog({ open: true, editing: { username: u.username, role: u.role } })}>编辑</Button>
                      {!self && (
                        <Button size="small" variant="outlined" onClick={() => remove(u.username)}
                          sx={{ ...smBtn, ml: '4px', color: palette.red, borderColor: '#fecaca', '&:hover': { bgcolor: palette.redSoft, color: palette.red, borderColor: '#fecaca' } }}>
                          删除
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </ResponsiveTable>
      </Panel>

      <Panel title="角色说明" sx={{ mt: 2 }} bodySx={{ fontSize: 13, color: palette.text2, lineHeight: 2 }}>
        <Badge tone="indigo">管理员</Badge> 可访问全部功能，包括系统配置与用户管理<br />
        <Badge tone="gray">只读用户</Badge> 可查看监控数据、使用检测工具，不能修改配置
      </Panel>

      <UserDialog open={dialog.open} editing={dialog.editing} onClose={() => setDialog((d) => ({ ...d, open: false }))} onSaved={load} />
    </>
  );
}
