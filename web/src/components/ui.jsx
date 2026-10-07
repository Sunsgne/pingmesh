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
      borderRadius: 999, bgcolor: bg, color: fg, lineHeight: 1.4, whiteSpace: 'nowrap', verticalAlign: 'middle',
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
          {sub && <Typography component="span" sx={{ color: palette.text3, fontSize: 12, minWidth: 0, '@media (max-width:900px)': { display: 'block', width: '100%', flexBasis: '100%', order: 1 } }}>{sub}</Typography>}
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
      <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 1.75, p: '18px !important', '@media (max-width:900px)': { p: '14px !important' } }}>
        <Box sx={{ width: 44, height: 44, borderRadius: '12px', bgcolor: bg, color: fg, display: 'grid', placeItems: 'center', flex: 'none', '& svg': { width: 22, height: 22 } }}>{icon}</Box>
        <Box sx={{ minWidth: 0 }}>
          <Typography sx={{ fontSize: 22, fontWeight: 700, lineHeight: 1.1, '@media (max-width:900px)': { fontSize: 20 } }}>{value}</Typography>
          <Typography noWrap sx={{ fontSize: 12.5, color: palette.text3, mt: 0.25 }}>{label}</Typography>
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
