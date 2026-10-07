import { palette } from './theme';
import { chartAxisLabels, fmtLossPct, sanitizeSeries } from './api';

const tooltipBase = {
  trigger: 'axis',
  backgroundColor: 'rgba(15,23,42,.92)',
  borderWidth: 0,
  padding: [8, 12],
  textStyle: { color: '#e2e8f0', fontSize: 11 },
  confine: true,
};

function fmtVal(name, v) {
  if (v == null) return '-';
  return name === '丢包率' ? fmtLossPct(v) : Number(v).toFixed(2) + ' ms';
}

function tooltipFormatter(params) {
  const list = Array.isArray(params) ? params : [params];
  if (!list.length) return '';
  const lines = [list[0].axisValueLabel || list[0].name || ''];
  list.forEach((p) => lines.push(`${p.marker} ${p.seriesName}  ${fmtVal(p.seriesName, p.value)}`));
  return lines.join('<br/>');
}

function area(alpha) {
  return {
    color: new window.echarts.graphic.LinearGradient(0, 0, 0, 1, [
      { offset: 0, color: `rgba(99,102,241,${alpha})` },
      { offset: 1, color: 'rgba(99,102,241,.02)' },
    ]),
  };
}

export function miniOption(data) {
  return {
    animation: false,
    grid: { left: 8, right: 12, top: 14, bottom: 8, containLabel: true },
    tooltip: { ...tooltipBase, formatter: tooltipFormatter },
    xAxis: {
      data: chartAxisLabels(data.lastcheck),
      boundaryGap: false,
      axisLabel: { fontSize: 10, color: palette.text3, hideOverlap: true },
      axisLine: { show: false },
      axisTick: { show: false },
    },
    yAxis: [
      { type: 'value', min: 0, splitNumber: 3, axisLabel: { fontSize: 10, color: palette.text3 }, splitLine: { lineStyle: { color: '#f1f5f9' } } },
      { type: 'value', min: 0, max: 100, show: false },
    ],
    series: [
      { name: '延迟', type: 'line', showSymbol: false, connectNulls: true, sampling: 'lttb', itemStyle: { color: palette.primary2 }, lineStyle: { width: 2 }, areaStyle: area(0.28), data: sanitizeSeries(data.avgdelay) },
      { name: '丢包率', type: 'line', yAxisIndex: 1, showSymbol: false, connectNulls: true, sampling: 'lttb', itemStyle: { color: '#f43f5e' }, lineStyle: { width: 1.4, type: 'dashed' }, data: sanitizeSeries(data.losspk) },
    ],
  };
}

export function bigOption(data) {
  const line = (name, color, extra = {}) => ({
    name, type: 'line', showSymbol: false, smooth: true, connectNulls: true, sampling: 'lttb',
    itemStyle: { color }, lineStyle: { width: 1.6 }, ...extra,
  });
  return {
    animation: false,
    tooltip: { ...tooltipBase, padding: [10, 14], textStyle: { color: '#e2e8f0', fontSize: 12 }, formatter: tooltipFormatter, axisPointer: { type: 'cross', label: { backgroundColor: palette.primary } } },
    legend: { data: ['最大延迟', '平均延迟', '最小延迟', '丢包率', '抖动'], selected: { 最大延迟: false, 最小延迟: false }, icon: 'roundRect', itemWidth: 14, itemHeight: 8, textStyle: { color: palette.text2 } },
    grid: { left: 12, right: 16, top: 52, bottom: 56, containLabel: true },
    dataZoom: [{ type: 'slider', height: 20, bottom: 8, borderColor: 'transparent', backgroundColor: '#f1f5f9', fillerColor: 'rgba(99,102,241,.15)', handleStyle: { color: palette.primary2 } }],
    xAxis: { data: data.lastcheck || [], boundaryGap: false, axisLine: { lineStyle: { color: palette.border } }, axisLabel: { color: palette.text3 } },
    yAxis: [
      { type: 'value', name: '延迟(ms)', min: 0, nameTextStyle: { color: palette.text3 }, axisLabel: { color: palette.text3 }, splitLine: { lineStyle: { color: '#f1f5f9' } } },
      { type: 'value', name: '丢包率(%)', min: 0, max: 100, nameTextStyle: { color: palette.text3 }, axisLabel: { color: palette.text3, formatter: (v) => fmtLossPct(v) }, splitLine: { show: false } },
    ],
    series: [
      line('最大延迟', '#a5b4fc', { areaStyle: { opacity: 0.12 }, data: sanitizeSeries(data.maxdelay) }),
      line('最小延迟', '#c4b5fd', { areaStyle: { opacity: 0.12 }, data: sanitizeSeries(data.mindelay) }),
      line('平均延迟', palette.primary2, { lineStyle: { width: 2.2 }, areaStyle: area(0.3), data: sanitizeSeries(data.avgdelay) }),
      line('丢包率', '#f43f5e', { yAxisIndex: 1, smooth: false, lineStyle: { width: 1.8, type: 'dashed' }, data: sanitizeSeries(data.losspk) }),
      line('抖动', palette.yellow, { data: sanitizeSeries(data.jitter || []) }),
    ],
  };
}
