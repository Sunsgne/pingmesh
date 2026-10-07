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

const shadow = '0 1px 2px rgba(15, 23, 42, .06), 0 1px 3px rgba(15, 23, 42, .08)';
const shadowLg = '0 10px 30px rgba(15, 23, 42, .18)';
const font = '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "微软雅黑", Arial, sans-serif';

export const theme = createTheme({
  palette: {
    primary: { main: palette.primary, light: palette.primary2, dark: '#4338ca' },
    success: { main: palette.green },
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
    h6: { fontSize: 16, fontWeight: 600 },
  },
  shadows: ['none', shadow, shadow, shadow, shadow, ...Array(20).fill(shadowLg)],
  components: {
    MuiCssBaseline: { styleOverrides: { body: { WebkitFontSmoothing: 'antialiased' } } },
    MuiPaper: { styleOverrides: { rounded: { borderRadius: 12 } } },
    MuiCard: {
      defaultProps: { elevation: 0 },
      styleOverrides: { root: { border: `1px solid ${palette.border}`, boxShadow: shadow } },
    },
    MuiButton: { styleOverrides: { root: { borderRadius: 8 } } },
    MuiChip: { styleOverrides: { root: { fontWeight: 600 } } },
    MuiToggleButton: {
      styleOverrides: {
        root: {
          border: 'none', borderRadius: '7px !important', padding: '5px 14px',
          fontWeight: 600, fontSize: 12.5, color: palette.text2, textTransform: 'none',
          '&.Mui-selected': { background: '#fff', color: palette.text, boxShadow: shadow },
          '&.Mui-selected:hover': { background: '#fff' },
        },
      },
    },
    MuiToggleButtonGroup: {
      styleOverrides: { root: { background: palette.border, borderRadius: 9, padding: 3, gap: 2 } },
    },
    MuiDialog: { styleOverrides: { paper: { borderRadius: 14 } } },
    MuiOutlinedInput: { styleOverrides: { root: { background: '#fff' } } },
  },
});

export { shadow, shadowLg };
