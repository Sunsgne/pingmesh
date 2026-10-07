import { createContext, useCallback, useContext, useRef, useState } from 'react';
import { Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, Slide, Typography } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { palette, shadowLg } from '../theme';

const Ctx = createContext(null);
const TOAST_BG = { ok: '#047857', err: '#b91c1c', info: palette.text };

// 全局 toast(右上角堆叠, 3.2s 消失) 与确认框, 对应旧版 SP.toast / SP.confirm
export function FeedbackProvider({ children }) {
  const [toasts, setToasts] = useState([]);
  const [confirm, setConfirm] = useState(null);
  const seq = useRef(0);

  const toast = useCallback((msg, type = 'info') => {
    const id = ++seq.current;
    setToasts((t) => [...t, { id, msg: String(msg), type }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 3200);
  }, []);

  const ask = useCallback((title, msg, opts = {}) => new Promise((resolve) => {
    setConfirm({ title, msg, okText: opts.okText || '确定', danger: opts.danger !== false, resolve });
  }), []);

  const close = (v) => {
    if (confirm) confirm.resolve(v);
    setConfirm(null);
  };

  return (
    <Ctx.Provider value={{ toast, confirm: ask }}>
      {children}
      <Box sx={{ position: 'fixed', top: 18, right: 18, zIndex: 2000, display: 'flex', flexDirection: 'column', gap: 1.25, pointerEvents: 'none' }}>
        {toasts.map((t) => (
          <Slide key={t.id} direction="left" in mountOnEnter>
            <Box role="status" sx={{
              bgcolor: TOAST_BG[t.type] || TOAST_BG.info, color: '#fff', px: 2.25, py: 1.4, borderRadius: 2.5,
              fontSize: 13.5, boxShadow: shadowLg, maxWidth: 380, pointerEvents: 'auto', wordBreak: 'break-word',
            }}>{t.msg}</Box>
          </Slide>
        ))}
      </Box>
      <Dialog open={!!confirm} onClose={() => close(false)} maxWidth="xs" fullWidth>
        {confirm && (
          <>
            <DialogTitle sx={{ display: 'flex', alignItems: 'center', fontSize: 15, fontWeight: 700, borderBottom: `1px solid ${palette.border}` }}>
              <Box component="span" sx={{ flex: 1 }}>{confirm.title}</Box>
              <IconButton size="small" onClick={() => close(false)}><CloseIcon fontSize="small" /></IconButton>
            </DialogTitle>
            <DialogContent sx={{ pt: '20px !important' }}>
              <Typography sx={{ fontSize: 14, whiteSpace: 'pre-wrap' }}>{confirm.msg}</Typography>
            </DialogContent>
            <DialogActions sx={{ px: 2.5, py: 1.75, borderTop: `1px solid ${palette.border}` }}>
              <Button variant="outlined" color="inherit" onClick={() => close(false)}>取消</Button>
              <Button variant="contained" disableElevation color={confirm.danger ? 'error' : 'primary'} onClick={() => close(true)}>{confirm.okText}</Button>
            </DialogActions>
          </>
        )}
      </Dialog>
    </Ctx.Provider>
  );
}

export function useToast() {
  return useContext(Ctx).toast;
}
export function useConfirm() {
  return useContext(Ctx).confirm;
}
