import { useEffect, useRef } from 'react';

// 轻量 ECharts 封装: echarts 由 /assets/js/echarts.min.js 全局提供
export default function EChart({ option, height = 150, onClick }) {
  const el = useRef(null);
  const chart = useRef(null);

  useEffect(() => {
    chart.current = window.echarts.init(el.current);
    const onResize = () => chart.current && chart.current.resize();
    window.addEventListener('resize', onResize);
    return () => {
      window.removeEventListener('resize', onResize);
      chart.current.dispose();
      chart.current = null;
    };
  }, []);

  useEffect(() => {
    if (chart.current && option) chart.current.setOption(option, true);
  }, [option]);

  return <div ref={el} onClick={onClick} style={{ width: '100%', height }} />;
}
