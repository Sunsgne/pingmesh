import { useCallback, useMemo, useState } from 'react';
import { Box, Button, Stack, TextField, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { useToast } from './Feedback';
import { ToolbarBox, ToolbarLabel } from './ui';
import { fromInputTime, loadPref, rangeFromPreset, savePref, timeWindowLabel, toInputTime } from '../api';
import { palette } from '../theme';

export const DEFAULT_PRESETS = [
  { v: 15, label: '15分钟' }, { v: 60, label: '1小时' }, { v: 120, label: '2小时' },
  { v: 360, label: '6小时' }, { v: 1440, label: '24小时' },
];

/**
 * 时间窗口状态, 对应旧版 SP.initPageTimeFilter: localStorage 记忆 {mins, custom},
 * 自定义区间需点「应用」才生效; 预设区间在 tick 变化时滚动到当前时间。
 */
export function useTimeWindow(storageKey, defaultMins = 120) {
  const initial = useMemo(() => {
    const s = loadPref(storageKey, null);
    if (s && s.custom && s.custom.start && s.custom.end) return { mins: defaultMins, custom: s.custom };
    if (s && s.mins) return { mins: s.mins, custom: null };
    return { mins: defaultMins, custom: null };
  }, [storageKey, defaultMins]);
  const [mins, setMinsState] = useState(initial.mins);
  const [custom, setCustomState] = useState(initial.custom);
  const [tick, setTick] = useState(0);

  const setMins = useCallback((m) => {
    setMinsState(m); setCustomState(null);
    savePref(storageKey, { mins: m, custom: null });
  }, [storageKey]);
  const setCustom = useCallback((c) => {
    setCustomState(c);
    savePref(storageKey, { mins, custom: c });
  }, [storageKey, mins]);
  const refresh = useCallback(() => setTick((t) => t + 1), []);

  const range = useMemo(() => custom || rangeFromPreset(mins), [custom, mins, tick]); // eslint-disable-line react-hooks/exhaustive-deps
  const label = timeWindowLabel(custom ? null : mins, custom);
  return { mins, custom, range, label, tick, setMins, setCustom, refresh };
}

/** 时间工具栏 UI(分段预设 + 自定义起止 + 区间提示 + 时区) */
export function TimeToolbar({ tw, presets = DEFAULT_PRESETS, label = '时间', children, tz = true, inline = false, sx }) {
  const toast = useToast();
  const [editing, setEditing] = useState(!!tw.custom);
  const [draft, setDraft] = useState(() => tw.custom || rangeFromPreset(360));

  const onSeg = (_, v) => {
    if (v == null) return;
    if (v === 'custom') {
      setEditing(true);
      if (!draft.start) setDraft(rangeFromPreset(360));
      return;
    }
    setEditing(false);
    tw.setMins(v);
  };
  const apply = () => {
    if (!draft.start || !draft.end) { toast('请选择起止时间', 'err'); return; }
    if (draft.start >= draft.end) { toast('结束时间需晚于开始时间', 'err'); return; }
    tw.setCustom({ start: draft.start, end: draft.end });
  };

  const body = (
    <>
      <ToolbarLabel>{label}</ToolbarLabel>
      <ToggleButtonGroup exclusive size="small" value={editing ? 'custom' : tw.mins} onChange={onSeg}>
        {presets.map((p) => <ToggleButton key={p.v} value={p.v}>{p.label}</ToggleButton>)}
        <ToggleButton value="custom">自定义</ToggleButton>
      </ToggleButtonGroup>
      {editing && (
        <Stack direction="row" alignItems="center" gap={0.75} sx={{ flexWrap: 'nowrap' }}>
          <TextField size="small" type="datetime-local" value={toInputTime(draft.start)} sx={{ width: 190 }}
            onChange={(e) => setDraft((d) => ({ ...d, start: fromInputTime(e.target.value) }))} />
          <Typography sx={{ color: palette.text3 }}>~</Typography>
          <TextField size="small" type="datetime-local" value={toInputTime(draft.end)} sx={{ width: 190 }}
            onChange={(e) => setDraft((d) => ({ ...d, end: fromInputTime(e.target.value) }))} />
          <Button size="small" variant="contained" onClick={apply}>应用</Button>
        </Stack>
      )}
      {children}
      <Box sx={{ flex: '1 1 12px' }} />
      {!editing && <Typography sx={{ fontSize: 11.5, color: palette.text3, whiteSpace: 'nowrap' }}>{tw.range.start} ~ {tw.range.end}</Typography>}
      {tz && <Typography sx={{ fontSize: 11.5, color: palette.text3, whiteSpace: 'nowrap' }}>北京时间 (UTC+8)</Typography>}
    </>
  );
  if (inline) return <Stack direction="row" alignItems="center" sx={{ gap: '8px 12px', flexWrap: 'wrap', ...sx }}>{body}</Stack>;
  return <ToolbarBox sx={sx}>{body}</ToolbarBox>;
}
