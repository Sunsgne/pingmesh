import { useLayoutEffect, useRef } from 'react';
import { Box, Card, CardContent, CircularProgress, Stack, Typography } from '@mui/material';
import { keyframes } from '@mui/system';
import { mono, palette, shadow, shadowLg } from '../theme';

/* 旧版 .badge: 圆角药丸 + 可选呼吸点 */
const TONE = {
  green: [palette.greenSoft, '#047857'],
  red: [palette.redSoft, '#b91c1c'],
  yellow: [palette.yellowSoft, '#b45309'],
  gray: ['#f1f5f9', palette.text2],
  indigo: [palette.primarySoft, palette.primary],
};
const pulseGreen = keyframes`0%,100%{box-shadow:0 0 0 0 rgba(16,185,129,.5)}50%{box-shadow:0 0 0 4px rgba(16,185,129,0)}`;
const pulseRed = keyframes`0%,100%{box-shadow:0 0 0 0 rgba(239,68,68,.55)}50%{box-shadow:0 0 0 4px rgba(239,68,68,0)}`;

export function Badge({ tone = 'gray', dot = false, mono: isMono = false, children, sx, ...rest }) {
  const [bg, fg] = TONE[tone] || TONE.gray;
  return (
    <Box component="span" sx={{
      display: 'inline-flex', alignItems: 'center', gap: '5px', px: '9px', py: '3px', fontSize: 12, fontWeight: 600,
      borderRadius: 999, bgcolor: bg, color: fg, lineHeight: 1.4, whiteSpace: 'nowrap', verticalAlign: 'middle', fontVariantNumeric: 'tabular-nums',
      ...(isMono ? { fontFamily: mono, fontSize: 12.5 } : {}),
      '@media (max-width:480px)': { fontSize: 11, px: '7px', py: '2px' }, ...sx,
    }} {...rest}>
      {dot && (
        <Box component="span" sx={{
          width: 6, height: 6, borderRadius: '50%', bgcolor: 'currentColor', flex: 'none',
          animation: tone === 'green' ? `${pulseGreen} 1.8s ease-in-out infinite` : tone === 'red' ? `${pulseRed} 1.6s ease-in-out infinite` : 'none',
        }} />
      )}
      {children}
    </Box>
  );
}

/* 旧版 .card + .card-head */
export function Panel({ title, sub, actions, children, bodySx, flat = false, sx }) {
  return (
    <Card sx={{ overflow: 'visible', ...sx }}>
      {(title || actions) && (
        <Box sx={{ px: 2.25, py: 1.75, borderBottom: `1px solid ${palette.border}`, display: 'flex', alignItems: 'center', gap: 1.25, flexWrap: 'wrap', rowGap: 1, '@media (max-width:900px)': { px: 1.75, py: 1.5 } }}>
          {title && <Typography component="h3" sx={{ fontSize: 14.5, fontWeight: 600 }}>{title}</Typography>}
          {sub && <Typography component="span" sx={{ color: palette.text3, fontSize: 12, minWidth: 0, '@media (max-width:900px)': { display: 'block', width: '100%', flexBasis: '100%' } }}>{sub}</Typography>}
          <Box sx={{ flex: 1 }} />
          {actions}
        </Box>
      )}
      <Box sx={{ p: flat ? 0 : 2.25, ...(flat ? {} : { '@media (max-width:900px)': { p: 1.75 } }), ...bodySx }}>{children}</Box>
    </Card>
  );
}

/* 旧版 .page-toolbar / .mesh-toolbar */
export function ToolbarBox({ children, sx }) {
  return (
    <Stack direction="row" alignItems="center" sx={{
      gap: '10px 12px', flexWrap: 'wrap', p: '12px 14px', mb: 1.75,
      background: 'linear-gradient(180deg, #f8fafc 0%, #fff 100%)', border: `1px solid ${palette.border}`, borderRadius: '12px',
      '@media (max-width:900px)': { p: '10px 12px', gap: '8px' }, ...sx,
    }}>{children}</Stack>
  );
}
export function ToolbarLabel({ children }) {
  return <Typography component="span" sx={{ fontSize: 11, fontWeight: 700, color: palette.text3, letterSpacing: '.06em', whiteSpace: 'nowrap', '@media (max-width:900px)': { display: 'none' } }}>{children}</Typography>;
}

const TONE_ICON = {
  indigo: [palette.primarySoft, palette.primary], green: [palette.greenSoft, palette.green],
  yellow: [palette.yellowSoft, palette.yellow], red: [palette.redSoft, palette.red],
};
export function StatCard({ icon, tone = 'indigo', value, label }) {
  const [bg, fg] = TONE_ICON[tone];
  return (
    <Card sx={{ transition: 'box-shadow .18s, transform .18s', '&:hover': { boxShadow: shadowLg, transform: 'translateY(-2px)' } }}>
      <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 1.75, p: '18px !important', '@media (max-width:900px)': { p: '14px !important' }, '@media (max-width:600px)': { p: '12px !important', gap: 1.25 } }}>
        <Box sx={{ width: 44, height: 44, borderRadius: '12px', bgcolor: bg, color: fg, display: 'grid', placeItems: 'center', flex: 'none', '& svg': { width: 22, height: 22 }, '@media (max-width:600px)': { width: 34, height: 34, borderRadius: '10px', '& svg': { width: 18, height: 18 } } }}>{icon}</Box>
        <Box sx={{ minWidth: 0 }}>
          <Typography sx={{ fontSize: 22, fontWeight: 700, lineHeight: 1.1, fontVariantNumeric: 'tabular-nums', letterSpacing: '-.01em', '@media (max-width:900px)': { fontSize: 20 }, '@media (max-width:600px)': { fontSize: 17, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' } }}>{value}</Typography>
          <Typography noWrap sx={{ fontSize: 12.5, color: palette.text3, mt: 0.25, '@media (max-width:600px)': { fontSize: 11.5 } }}>{label}</Typography>
        </Box>
      </CardContent>
    </Card>
  );
}

export function EmptyState({ icon, children, sx }) {
  return (
    <Box sx={{ textAlign: 'center', py: 6, px: 2.5, color: palette.text3, fontSize: 13.5, ...sx }}>
      {icon && <Box sx={{ fontSize: 38, mb: 1.25 }}>{icon}</Box>}
      {children}
    </Box>
  );
}

export function Spinner({ size = 16 }) {
  return <CircularProgress size={size} thickness={5} sx={{ color: palette.primary2, verticalAlign: 'middle' }} />;
}

export function Mono({ children, sx }) {
  return <Box component="span" sx={{ fontFamily: mono, fontSize: 12.5, ...sx }}>{children}</Box>;
}

export function Hint({ children, sx }) {
  return <Typography sx={{ fontSize: 12, color: palette.text3, mt: 0.6, ...sx }}>{children}</Typography>;
}

export const cardHover = { transition: 'box-shadow .18s, transform .18s', '&:hover': { boxShadow: shadowLg, transform: 'translateY(-2px)' } };
export { shadow };

const card = { border: `1px solid ${palette.border}`, borderRadius: '12px', my: '10px', bgcolor: '#fff', boxShadow: shadow, '&:hover': { bgcolor: '#fff' } };
function stackedSx(labelWidth, dense) {
  const label = { content: 'attr(data-label)', color: palette.text3, fontWeight: 600 };
  return {
    overflowX: 'visible', mx: '0 !important',
    '& table, & tbody, & tr': { display: 'block', width: '100%' },
    '& thead': { display: 'block' },
    '& thead tr': { display: 'flex', alignItems: 'center' },
    '& thead th': { display: 'none' },
    '& thead th:has(input)': { display: 'flex', alignItems: 'center', border: 'none', p: '4px 6px', width: 'auto !important' },
    '& tbody tr': dense
      ? { ...card, display: 'grid', gridTemplateColumns: '1fr 1fr', columnGap: '8px', position: 'relative', py: '8px', px: '4px' }
      : { ...card, py: '6px' },
    '& tbody td': {
      display: 'flex', alignItems: 'flex-start', border: 'none !important', textAlign: 'left !important',
      minWidth: '0 !important', maxWidth: 'none !important', width: 'auto !important',
      whiteSpace: 'normal !important', wordBreak: 'break-word', fontSize: 13,
      ...(dense
        ? { flexDirection: 'column', gap: '2px', py: '4px !important', px: '10px !important' }
        : { gap: '10px', py: '5px !important', px: '14px !important' }),
    },
    '& tbody td > *': { minWidth: 0 },
    '& tbody td button, & tbody td a.MuiButton-root': { minHeight: 32 },
    '& tbody td[data-label]:not([data-label=""])::before': dense
      ? { ...label, fontSize: 11, lineHeight: '16px' }
      : { ...label, flex: `0 0 ${labelWidth}px`, fontSize: 12, lineHeight: '20px' },
    '& tbody td[colspan]': dense ? { gridColumn: '1 / -1', justifyContent: 'center' } : { justifyContent: 'center' },
    ...(dense ? {
      // 无列名的单元格(复选框)放到卡片右上角; 第一个有列名的单元格作为卡片标题
      '& tbody td[data-label=""]:not([colspan])': { position: 'absolute', top: '4px', right: '4px', p: '0 !important' },
      '& tbody td.rt-title': { gridColumn: '1 / -1', pr: '44px !important', flexDirection: 'row', flexWrap: 'wrap', alignItems: 'baseline', gap: '8px', pb: '6px !important' },
      '& tbody td.rt-title > *': { display: 'inline' },
      '& tbody td.rt-title::before': { display: 'none' },
    } : {}),
  };
}

/**
 * 手机端(≤600px)把表格每行变成一张卡片, 每个单元格前显示列名。
 * 列名取自 thead, 由 MutationObserver 自动同步到 td[data-label], 页面无需逐格标注。
 * 含复选框的表头单元格(全选)在卡片模式下保留显示。
 */
export function ResponsiveTable({ children, sx, labelWidth = 76, dense = false }) {
  const ref = useRef(null);
  useLayoutEffect(() => {
    const root = ref.current;
    if (!root) return undefined;
    const label = () => {
      root.querySelectorAll('table').forEach((t) => {
        const heads = Array.from(t.querySelectorAll('thead th')).map((th) => th.textContent.trim());
        t.querySelectorAll('tbody tr').forEach((tr) => {
          let col = 0;
          let titled = false;
          Array.from(tr.children).forEach((td) => {
            const span = +td.getAttribute('colspan') || 1;
            const l = span > 1 ? '' : heads[col] || '';
            if (td.getAttribute('data-label') !== l) td.setAttribute('data-label', l);
            const isTitle = dense && !titled && l !== '';
            if (isTitle) titled = true;
            td.classList.toggle('rt-title', isTitle);
            col += span;
          });
        });
      });
    };
    label();
    const mo = new MutationObserver(label);
    mo.observe(root, { childList: true, subtree: true, characterData: true });
    return () => mo.disconnect();
  }, [dense]);
  const PHONE = '@media (max-width:600px)';
  const { [PHONE]: phoneSx, ...restSx } = sx || {};
  return (
    <Box ref={ref} sx={{
      overflowX: 'auto', WebkitOverflowScrolling: 'touch',
      ...restSx,
      [PHONE]: { ...stackedSx(labelWidth, dense), ...phoneSx },
    }}>{children}</Box>
  );
}
