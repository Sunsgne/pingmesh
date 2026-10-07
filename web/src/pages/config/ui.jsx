import {
  Box, Button, Checkbox, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, Switch, TextField, Typography,
} from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { mono, palette } from '../../theme';

/* 旧版 .field / label / .hint */
export const labelSx = { display: 'block', fontSize: 13, fontWeight: 600, color: palette.text2, mb: '6px' };
export const hintSx = { fontSize: 12, color: palette.text3, mt: '5px', lineHeight: 1.5 };
export const mutedSx = { color: palette.text3 };
// 旧版 .hint 仅在 .field 内有样式; 其他位置(如表格下方的 .hint.mt-2)按普通正文显示
export const plainSx = { fontSize: 14, color: palette.text, mt: 1, lineHeight: 1.5 };
export const codeSx = { background: palette.bg, padding: '2px 6px', borderRadius: '4px', fontFamily: mono, fontSize: '0.9em', wordBreak: 'break-all' };

export function Field({ label, hint, children, sx, labelStyle }) {
  return (
    <Box sx={{ mb: 2, minWidth: 0, ...sx }}>
      {label != null && <Typography component="label" sx={{ ...labelSx, ...labelStyle }}>{label}</Typography>}
      {children}
      {hint != null && <Typography component="div" sx={hintSx}>{hint}</Typography>}
    </Box>
  );
}
export function Hint({ children, sx, inline }) {
  return <Typography component={inline ? 'span' : 'div'} sx={{ ...hintSx, ...(inline ? { display: 'inline', ml: 1, mt: 0 } : {}), ...sx }}>{children}</Typography>;
}

/* 旧版 .input: 受控文本/数字输入, onChange 直接给出字符串值 */
export function Input({ value, onChange, type = 'text', placeholder, isMono, dense, width, sx, inputProps, title, ...rest }) {
  return (
    <TextField
      value={value}
      type={type}
      placeholder={placeholder}
      onChange={(e) => onChange && onChange(e.target.value)}
      size={dense ? 'small' : 'medium'}
      fullWidth={!width}
      title={title}
      inputProps={{ spellCheck: false, ...inputProps }}
      sx={{
        ...(width ? { width } : {}),
        '& .MuiOutlinedInput-root': { borderRadius: '8px', ...(isMono ? { fontFamily: mono, fontSize: 12.5 } : {}) },
        '& .MuiOutlinedInput-input': dense ? { padding: '4px 8px', ...(isMono ? {} : { fontSize: 13.5 }) } : {},
        '& .MuiOutlinedInput-root.Mui-focused fieldset': { borderColor: palette.primary2, borderWidth: 1, boxShadow: '0 0 0 3px rgba(99,102,241,.15)' },
        ...sx,
      }}
      {...rest}
    />
  );
}

export function TextArea({ value, onChange, rows = 3, placeholder, readOnly, sx, inputRef, id }) {
  return (
    <TextField
      multiline rows={rows} fullWidth value={value} placeholder={placeholder} id={id}
      onChange={(e) => onChange && onChange(e.target.value)}
      inputRef={inputRef}
      inputProps={{ readOnly, spellCheck: false }}
      sx={{
        '& .MuiOutlinedInput-root': { p: '9px 12px', borderRadius: '8px', fontFamily: mono, fontSize: 12.5, lineHeight: 1.6, alignItems: 'flex-start' },
        '& textarea': { resize: 'vertical', padding: 0 },
        '& .MuiOutlinedInput-root.Mui-focused fieldset': { borderColor: palette.primary2, borderWidth: 1, boxShadow: '0 0 0 3px rgba(99,102,241,.15)' },
        ...sx,
      }}
    />
  );
}

/* 原生 <select>(值语义与旧版一致) */
export function NSelect({ value, onChange, options, sx, id }) {
  return (
    <TextField
      select fullWidth value={value} id={id}
      onChange={(e) => onChange(e.target.value)}
      SelectProps={{ native: true }}
      sx={{ '& .MuiOutlinedInput-root': { borderRadius: '8px' }, '& select': { padding: '9px 12px', fontSize: 13.5 }, ...sx }}
    >
      {options.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
    </TextField>
  );
}

/* 旧版 .switch (38×22) */
const switchSx = {
  width: 38, height: 22, p: 0, verticalAlign: 'middle',
  '& .MuiSwitch-switchBase': { p: '2px', '&.Mui-checked': { transform: 'translateX(16px)', color: '#fff', '& + .MuiSwitch-track': { bgcolor: palette.primary, opacity: 1 } } },
  '& .MuiSwitch-thumb': { width: 18, height: 18, boxShadow: '0 1px 3px rgba(0,0,0,.25)', bgcolor: '#fff' },
  '& .MuiSwitch-track': { borderRadius: 999, bgcolor: '#cbd5e1', opacity: 1 },
};
// 旧版 label.switch 位于 .field 内时被 `.field label{display:block;margin-bottom:6px}` 变成块级
export function SwBlock({ mt = '6px', ...props }) {
  return <Box sx={{ mt, mb: '6px', width: 38, height: 22 }}><Sw {...props} /></Box>;
}
export function Sw({ checked, onChange, sx, label }) {
  return <Switch checked={!!checked} onChange={(e) => onChange(e.target.checked)} disableRipple sx={{ ...switchSx, ...sx }} inputProps={{ 'aria-label': label }} />;
}

export function Chk({ checked, onChange, disabled, title, sx }) {
  return (
    <Checkbox size="small" checked={!!checked} disabled={disabled} title={title} disableRipple
      onChange={(e) => onChange && onChange(e.target.checked)}
      sx={{ p: 0, '& .MuiSvgIcon-root': { fontSize: 17 }, ...sx }} />
  );
}

/* 旧版 .btn / .btn.sm / .btn.primary / .btn.ghost-danger */
export function Btn({ children, primary, danger, sm, sx, ...rest }) {
  const base = sm ? { fontSize: 12.5, padding: '4px 10px', borderRadius: '7px', minWidth: 0 } : { padding: '7px 16px', minWidth: 0 };
  if (primary) return <Button variant="contained" disableElevation sx={{ ...base, whiteSpace: 'nowrap', ...sx }} {...rest}>{children}</Button>;
  const dangerSx = danger ? { color: palette.red, borderColor: '#fecaca', '&:hover': { background: palette.redSoft, color: palette.red, borderColor: '#fecaca' } } : {};
  return (
    <Button variant="outlined" color="inherit" sx={{
      ...base, whiteSpace: 'nowrap', borderColor: palette.border, color: palette.text2, background: '#fff',
      '&:hover': { background: palette.bg, color: palette.text, borderColor: palette.border },
      '&.Mui-disabled': { opacity: 0.55, color: danger ? palette.red : palette.text2, borderColor: danger ? '#fecaca' : palette.border },
      ...dangerSx, ...sx,
    }} {...rest}>{children}</Button>
  );
}

/* 旧版 .sp-modal (580 / lg 1240 / xl 1420): × / 取消 / Esc / 点击遮罩关闭 */
export function Modal({ open, onClose, title, width = 580, children, actions }) {
  return (
    <Dialog open={open} onClose={onClose} scroll="body" fullWidth maxWidth={false} PaperProps={{ sx: { maxWidth: width, m: '6vh 16px', width: 'calc(100% - 32px)', '@media (max-width:900px)': { m: '3vh 10px', width: 'calc(100% - 20px)', borderRadius: '12px' } } }}>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', borderBottom: `1px solid ${palette.border}`, py: 2, px: 2.5, fontSize: 15, fontWeight: 700 }}>
        <Box component="span" sx={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{title}</Box>
        <IconButton size="small" onClick={onClose} aria-label="关闭"><CloseIcon fontSize="small" /></IconButton>
      </DialogTitle>
      <DialogContent sx={{ p: '20px !important', '@media (max-width:900px)': { p: '14px !important' } }}>{children}</DialogContent>
      <DialogActions sx={{ px: 2.5, py: 1.75, borderTop: `1px solid ${palette.border}`, gap: 1.25, '& > :not(style) ~ :not(style)': { ml: 0 } }}>
        {actions}
      </DialogActions>
    </Dialog>
  );
}

/* 旧版 .sp-table 的 sx(用于 MUI Table) */
export const tableSx = {
  width: '100%',
  '& th': { textTransform: 'uppercase', textAlign: 'left', p: '11px 16px', color: palette.text3, fontSize: 12, fontWeight: 600, letterSpacing: '.06em', borderBottom: `1px solid ${palette.border}`, whiteSpace: 'nowrap' },
  '& td': { p: '11px 16px', borderBottom: '1px solid #f1f5f9', fontSize: 13.5, verticalAlign: 'middle' },
  '& tbody tr:hover': { background: '#f8fafc' },
  '& tbody tr:last-child td': { borderBottom: 'none' },
};

export const softBox = { fontSize: 12.5, background: palette.primarySoft, borderRadius: '8px', p: '10px 14px', lineHeight: 1.8, color: palette.text3 };

export const flexRow = { display: 'flex', alignItems: 'center', gap: '10px' };
export const Spacer = () => <Box sx={{ flex: 1 }} />;

export const gridCols = (n) => ({
  display: 'grid', gap: 2, gridTemplateColumns: `repeat(${n}, 1fr)`,
  ...(n === 3 ? { '@media (max-width:1100px)': { gridTemplateColumns: 'repeat(2, 1fr)' } } : {}),
  '@media (max-width:720px)': { gridTemplateColumns: '1fr' },
});
