import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react';

// ECharts 封装: echarts 由 /assets/js/echarts.min.js 全局提供(含 world 地图注册)。
// option 变化时整体替换(notMerge); 容器尺寸变化自动 resize(弹窗打开、侧栏折叠等)。
const EChart = forwardRef(function EChart({ option, height = 150, style, onEvents, initOpts, notMerge = true }, ref) {
  const el = useRef(null);
  const chart = useRef(null);
  const events = useRef(onEvents);
  events.current = onEvents;

  useImperativeHandle(ref, () => ({ get instance() { return chart.current; } }), []);

  useEffect(() => {
    const c = window.echarts.init(el.current, null, initOpts);
    chart.current = c;
    const names = Object.keys(events.current || {});
    names.forEach((n) => c.on(n, (p) => events.current && events.current[n] && events.current[n](p, c)));
    const ro = new ResizeObserver(() => { if (!c.isDisposed()) c.resize(); });
    ro.observe(el.current);
    return () => {
      ro.disconnect();
      c.dispose();
      chart.current = null;
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (chart.current && option) chart.current.setOption(option, notMerge);
  }, [option, notMerge]);

  return <div ref={el} style={{ width: '100%', height, ...style }} />;
});

export default EChart;
