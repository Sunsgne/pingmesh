import { createTheme } from '@mui/material/styles';

// 沿用 modern.css 的原有配色 / 圆角 / 阴影, 仅换成 MUI 组件实现
export const palette = {
  bg: '#f1f5f9',
  card: '#ffffff',
  border: '#e2e8f0',
  text: '#0f172a',
  text2: '#475569',
  text3: '#94a3b8',
  primary: '#4f46e5',
  primary2: '#6366f1',
  primarySoft: '#eef2ff',
  green: '#10b981',
  greenSoft: '#ecfdf5',
  yellow: '#f59e0b',
  yellowSoft: '#fffbeb',
  red: '#ef4444',
  redSoft: '#fef2f2',
  sidebarBg: '#0f172a',
  sidebarText: '#94a3b8',
};

export const shadow = '0 1px 2px rgba(15, 23, 42, .06), 0 1px 3px rgba(15, 23, 42, .08)';
export const shadowLg = '0 10px 30px rgba(15, 23, 42, .18)';
export const font = '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "微软雅黑", Arial, sans-serif';
export const mono = 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace';

export const theme = createTheme({
  palette: {
    primary: { main: palette.primary, light: palette.primary2, dark: '#4338ca' },
    success: { main: palette.green, contrastText: '#fff' },
    warning: { main: palette.yellow },
    error: { main: palette.red },
    background: { default: palette.bg, paper: palette.card },
    text: { primary: palette.text, secondary: palette.text2, disabled: palette.text3 },
    divider: palette.border,
  },
  shape: { borderRadius: 8 },
  typography: {
    fontFamily: font,
    fontSize: 13,
    button: { textTransform: 'none', fontWeight: 600 },
  },
  shadows: ['none', shadow, shadow, shadow, shadow, ...Array(20).fill(shadowLg)],
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        body: { WebkitFontSmoothing: 'antialiased', fontSize: 14 },
        a: { color: palette.primary, textDecoration: 'none' },
      },
    },
    MuiPaper: { styleOverrides: { rounded: { borderRadius: 12 } } },
    MuiCard: {
      defaultProps: { elevation: 0 },
      styleOverrides: { root: { border: `1px solid ${palette.border}`, boxShadow: shadow } },
    },
    MuiButton: {
      defaultProps: { disableElevation: true },
      styleOverrides: {
        root: { borderRadius: 8, fontSize: 13.5, padding: '6px 16px' },
        sizeSmall: { fontSize: 12.5, padding: '3px 10px', borderRadius: 7 },
        outlined: { borderColor: palette.border, color: palette.text2, background: '#fff', '&:hover': { background: palette.bg, borderColor: palette.border, color: palette.text } },
        containedPrimary: { background: `linear-gradient(135deg, ${palette.primary}, #6d28d9)`, '&:hover': { background: 'linear-gradient(135deg, #4338ca, #5b21b6)' } },
      },
    },
    MuiChip: { styleOverrides: { root: { fontWeight: 600 } } },
    MuiToggleButton: {
      styleOverrides: {
        root: {
          border: 'none', borderRadius: '7px !important', padding: '5px 14px', lineHeight: 1.5,
          fontWeight: 600, fontSize: 12.5, color: palette.text2, textTransform: 'none', whiteSpace: 'nowrap',
          '@media (max-width:600px)': { padding: '5px 10px', fontSize: 12 },
          '&.Mui-selected': { background: '#fff', color: palette.text, boxShadow: shadow },
          '&.Mui-selected:hover': { background: '#fff' },
        },
      },
    },
    MuiToggleButtonGroup: {
      styleOverrides: { root: { background: palette.border, borderRadius: 9, padding: 3, gap: 2, flexWrap: 'wrap' } },
    },
    MuiDialog: {
      styleOverrides: {
        container: { alignItems: 'flex-start' },
        paper: {
          borderRadius: 14, marginTop: '6vh',
          '@media (max-width:600px)': { margin: '3vh 10px 10px', width: 'calc(100% - 20px)', maxWidth: 'calc(100% - 20px) !important', borderRadius: 12 },
        },
      },
    },
    MuiBackdrop: {
      styleOverrides: {
        root: { '&:not(.MuiBackdrop-invisible)': { backgroundColor: 'rgba(15, 23, 42, .45)', backdropFilter: 'blur(8px)' } },
      },
    },
    MuiDialogTitle: { styleOverrides: { root: { fontSize: 15, fontWeight: 700, '@media (max-width:600px)': { padding: '12px 14px' } } } },
    MuiDialogContent: { styleOverrides: { root: { '@media (max-width:600px)': { padding: '14px' } } } },
    MuiDialogActions: { styleOverrides: { root: { '@media (max-width:600px)': { padding: '10px 14px' } } } },
    MuiOutlinedInput: {
      styleOverrides: {
        root: { background: '#fff', fontSize: 13.5, '& fieldset': { borderColor: palette.border } },
        input: { padding: '9px 12px' },
        inputSizeSmall: { padding: '6px 10px' },
      },
    },
    MuiInputLabel: { styleOverrides: { root: { fontSize: 13.5 } } },
    MuiTableCell: {
      styleOverrides: {
        root: { borderBottom: '1px solid #f1f5f9', padding: '11px 16px', fontSize: 13.5 },
        head: { color: palette.text3, fontSize: 12, fontWeight: 600, letterSpacing: '.06em', borderBottom: `1px solid ${palette.border}`, whiteSpace: 'nowrap' },
      },
    },
    MuiTableRow: { styleOverrides: { root: { '&.MuiTableRow-hover:hover': { background: '#f8fafc' } } } },
    MuiTab: { styleOverrides: { root: { textTransform: 'none', fontWeight: 600, fontSize: 13.5, minHeight: 46 } } },
    MuiTooltip: { styleOverrides: { tooltip: { background: 'rgba(15,23,42,.92)', fontSize: 12 } } },
  },
});
