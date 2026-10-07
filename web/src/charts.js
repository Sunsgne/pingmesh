import { palette } from './theme';
import { chartAxisLabels, fmtChartVal, fmtLossPct, sanitizeSeries } from './api';

const gradient = (alpha) => {
  try {
    return {
      color: new window.echarts.graphic.LinearGradient(0, 0, 0, 1, [
        { offset: 0, color: `rgba(99,102,241,${alpha})` },
        { offset: 1, color: 'rgba(99,102,241,.02)' },
      ]),
    };
  } catch (e) {
    return { color: 'rgba(99,102,241,.15)' };
  }
};

const SERIES_KEYS = ['maxdelay', 'mindelay', 'avgdelay', 'losspk', 'jitter'];
const isNum = (v) => v !== '-' && v !== '' && v != null && !Number.isNaN(parseFloat(v));
const toTime = (s) => new Date(String(s).replace(' ', 'T')).getTime();

/**
 * 去掉末尾尚未入库的空桶(当前采样周期 + 节点上报延迟), 避免曲线右侧留一截空白。
 * 空白超过 3 分钟视为真实中断, 保留以便看出节点断了。
 */
export function trimTrailingGap(d, maxGapSec = 180) {
  if (!d || !d.lastcheck || !d.lastcheck.length) return d;
  const n = d.lastcheck.length;
  let last = -1;
  for (let i = n - 1; i >= 0; i--) {
    if (isNum((d.avgdelay || [])[i]) || isNum((d.losspk || [])[i])) { last = i; break; }
  }
  if (last < 0 || last === n - 1) return d;
  const gap = (toTime(d.lastcheck[n - 1]) - toTime(d.lastcheck[last])) / 1000;
  if (!(gap > 0 && gap <= maxGapSec)) return d;
  const out = { ...d, lastcheck: d.lastcheck.slice(0, last + 1) };
  SERIES_KEYS.forEach((k) => { if (Array.isArray(d[k])) out[k] = d[k].slice(0, last + 1); });
  return out;
}

function miniTooltip(params) {
  params = Array.isArray(params) ? params : params ? [params] : [];
  if (!params.length) return '';
  const lines = [params[0].axisValueLabel || params[0].name || ''];
  params.forEach((p) => {
    const isLoss = p.seriesName === '丢包率' || p.seriesIndex === 1;
    lines.push(`${p.marker} ${p.seriesName || ''}  ${isLoss ? fmtLossPct(p.value) : fmtChartVal(p.value) + ' ms'}`);
  });
  return lines.join('\n');
}

// 卡片迷你图(延迟 + 丢包率), 对应旧版 miniChartOption + miniChartDataOption
// 迷你图横轴固定 5 个等距刻度(含首尾), 首尾标签贴边对齐, 不会互相挤压或被裁
function evenTicks(n, count = 5) {
  if (n <= count) return () => true;
  const keep = new Set();
  for (let k = 0; k < count; k++) keep.add(Math.round((k * (n - 1)) / (count - 1)));
  return (i) => keep.has(i);
}

export function miniChartOption(raw) {
  const d = trimTrailingGap(raw);
  const n = ((d && d.lastcheck) || []).length;
  return {
    animation: false,
    grid: { left: 6, right: 10, top: 14, bottom: 6, containLabel: true },
    tooltip: {
      trigger: 'axis', backgroundColor: 'rgba(15,23,42,.92)', borderWidth: 0, padding: [8, 12],
      textStyle: { color: '#e2e8f0', fontSize: 11 }, confine: true, formatter: miniTooltip,
    },
    xAxis: {
      data: chartAxisLabels((d && d.lastcheck) || []), boundaryGap: false,
      axisLabel: { fontSize: 10, color: palette.text3, margin: 6, interval: evenTicks(n), showMinLabel: true, showMaxLabel: true, alignMinLabel: 'left', alignMaxLabel: 'right' },
      axisLine: { show: false }, axisTick: { show: false },
    },
    yAxis: [
      { type: 'value', min: 0, scale: false, axisLabel: { fontSize: 10, color: palette.text3, hideOverlap: true }, splitLine: { lineStyle: { color: '#f1f5f9' } }, splitNumber: 3 },
      { type: 'value', min: 0, max: 100, show: false },
    ],
    series: [
      {
        name: '延迟', type: 'line', animation: false, showSymbol: false, smooth: false, connectNulls: true, clip: true,
        sampling: 'lttb', itemStyle: { color: palette.primary2 }, lineStyle: { width: 2 }, areaStyle: gradient(0.28),
        data: sanitizeSeries(d && d.avgdelay),
      },
      {
        name: '丢包率', type: 'line', yAxisIndex: 1, animation: false, showSymbol: false, connectNulls: true, clip: true,
        sampling: 'lttb', itemStyle: { color: '#f43f5e' }, lineStyle: { width: 1.4, type: 'dashed' },
        data: sanitizeSeries(d && d.losspk),
      },
    ],
  };
}

const line = (name, color, extra) => ({
  name, type: 'line', animation: false, showSymbol: false, smooth: true, connectNulls: true, clip: true,
  emphasis: { disabled: true }, sampling: 'lttb', itemStyle: { color }, ...extra,
});

// 历史曲线大图, 对应旧版 openPingChart
export function bigChartOption(raw) {
  const d = trimTrailingGap(raw);
  return {
    animation: false,
    tooltip: {
      trigger: 'axis', renderMode: 'richText', backgroundColor: 'rgba(15,23,42,.92)', borderWidth: 0, padding: [10, 14],
      textStyle: { color: '#e2e8f0', fontSize: 12 }, confine: true, transitionDuration: 0, showDelay: 0, hideDelay: 0,
      axisPointer: {
        type: 'cross', animation: false, snap: true,
        label: {
          show: true, backgroundColor: palette.primary, precision: 2,
          formatter: (p) => (p.axisDimension === 'y' ? fmtChartVal(p.value) : p.value),
        },
        crossStyle: { color: palette.text3, type: 'dashed' },
      },
      formatter: (params) => {
        if (!params || !params.length) return '';
        const lines = [params[0].axisValueLabel || params[0].name || ''];
        params.forEach((p) => lines.push(`${p.marker} ${p.seriesName}  ${p.seriesName === '丢包率' ? fmtLossPct(p.value) : fmtChartVal(p.value) + ' ms'}`));
        return lines.join('\n');
      },
    },
    legend: {
      data: ['最大延迟', '平均延迟', '最小延迟', '丢包率', '抖动'],
      selected: { 最大延迟: false, 最小延迟: false },
      icon: 'roundRect', itemWidth: 14, itemHeight: 8, textStyle: { color: palette.text2, fontSize: 12 }, type: 'scroll',
    },
    grid: { left: 12, right: 16, top: 52, bottom: 56, containLabel: true },
    dataZoom: [{
      type: 'slider', height: 20, bottom: 8, borderColor: 'transparent', backgroundColor: '#f1f5f9',
      fillerColor: 'rgba(99,102,241,.15)', handleStyle: { color: palette.primary2 }, realtime: false, throttle: 100, filterMode: 'none',
    }],
    xAxis: {
      data: (d && d.lastcheck) || [], boundaryGap: false, axisLine: { lineStyle: { color: palette.border } },
      axisLabel: { color: palette.text3, fontSize: 11, hideOverlap: true, margin: 8, showMinLabel: true, showMaxLabel: true, alignMinLabel: 'left', alignMaxLabel: 'right' },
      axisTick: { show: false },
    },
    yAxis: [
      {
        type: 'value', name: '延迟(ms)', min: 0, scale: false, position: 'left', nameTextStyle: { color: palette.text3 },
        axisLabel: { color: palette.text3, fontSize: 11, hideOverlap: true, formatter: fmtChartVal }, splitLine: { lineStyle: { color: '#f1f5f9' } },
      },
      {
        type: 'value', name: '丢包率(%)', min: 0, max: 100, position: 'right', nameTextStyle: { color: palette.text3 },
        axisLabel: { formatter: (v) => fmtLossPct(v), color: palette.text3, fontSize: 11, hideOverlap: true }, splitLine: { show: false },
      },
    ],
    series: [
      line('最大延迟', '#a5b4fc', { areaStyle: { opacity: 0.12 }, lineStyle: { width: 1.2 }, data: sanitizeSeries(d && d.maxdelay) }),
      line('最小延迟', '#c4b5fd', { areaStyle: { opacity: 0.12 }, lineStyle: { width: 1.2 }, data: sanitizeSeries(d && d.mindelay) }),
      line('平均延迟', palette.primary2, { lineStyle: { width: 2.2 }, areaStyle: gradient(0.3), data: sanitizeSeries(d && d.avgdelay) }),
      line('丢包率', '#f43f5e', { yAxisIndex: 1, smooth: false, lineStyle: { width: 1.8, type: 'dashed' }, data: sanitizeSeries(d && d.losspk) }),
      line('抖动', palette.yellow, { lineStyle: { width: 1.6 }, data: sanitizeSeries((d && d.jitter) || []) }),
    ],
  };
}

export function aggregationHint(step) {
  step = parseInt(step, 10) || 0;
  if (step <= 10) return '';
  const label = step >= 3600 ? step / 3600 + ' 小时' : step >= 60 ? step / 60 + ' 分钟' : step + ' 秒';
  return `长跨度已按 ${label} 聚合显示`;
}
