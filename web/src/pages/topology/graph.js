import { asnCached, asnShort } from '../../api';

const ESC = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
// ECharts tooltip/label 以 HTML 渲染, 服务端数据一律转义(= SP.esc)
export const esc = (s) => String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ESC[c]);

export const groupOf = (n) => n.group || '未分组';

/** 由所有节点的 Topology 规则构建节点/连线(对缺失字段做防御, 监测规则指向已删除节点时补占位节点) */
export function buildModel(cfg) {
  const nodes = {};
  const links = {};
  const net = (cfg && cfg.Network) || {};
  Object.keys(net).forEach((addr) => {
    const n = net[addr];
    if (!n) return;
    nodes[addr] = { name: n.Name || addr, addr, group: n.Group || '', source: (n.Topology || []).length > 0 };
  });
  Object.keys(net).forEach((addr) => {
    const n = net[addr];
    if (!n) return;
    (n.Topology || []).forEach((topo) => {
      if (!topo || !topo.Addr) return;
      const to = topo.Addr;
      if (!nodes[to]) nodes[to] = { name: topo.Name || to, addr: to, group: '', source: false, missing: true };
      links[addr + '|' + to] = { from: addr, to };
    });
  });

  const groupSet = {};
  Object.values(nodes).forEach((n) => { groupSet[groupOf(n)] = 1; });
  const groupNames = Object.keys(groupSet).sort((a, b) => {
    if (a === '未分组') return 1;
    if (b === '未分组') return -1;
    return a < b ? -1 : 1;
  });
  const nodeOpts = Object.keys(nodes).map((addr) => ({ addr, name: nodes[addr].name }));
  nodeOpts.sort((a, b) => {
    if (a.name === b.name) return a.addr < b.addr ? -1 : 1;
    return a.name < b.name ? -1 : 1;
  });
  return { nodes, links, groupNames, nodeOpts, count: Object.keys(nodes).length };
}

/**
 * 把各探测节点的 topology.json 结果套到模型上。
 * status[addr] = { st: 'pending' | 'ok' | 'fail', data }
 */
export function applyStatus(model, status) {
  const linkSt = {};
  const nodeBad = {};
  let hasAlert = false;
  Object.keys(model.links).forEach((k) => { linkSt[k] = { bad: false, pending: true }; });
  Object.keys(model.nodes).forEach((addr) => {
    const s = status[addr];
    if (!s || !model.nodes[addr].source) return;
    if (s.st === 'fail') {
      nodeBad[addr] = true;
      hasAlert = true;
    } else if (s.st === 'ok') {
      Object.keys(s.data || {}).forEach((toAddr) => {
        const l = linkSt[addr + '|' + toAddr];
        if (!l) return;
        l.pending = false;
        if (s.data[toAddr] === 'true') l.bad = false;
        else { l.bad = true; hasAlert = true; }
      });
    }
  });
  return { linkSt, nodeBad, hasAlert };
}

function buildNodeGraph(model, st, state, data, lns, baseSize, baseLine) {
  const { nodes, links } = model;
  const nodeMatches = (n) => {
    if (state.group && groupOf(n) !== state.group) return false;
    if (state.qAddr) return n.addr === state.qAddr;
    return true;
  };
  const isBad = (addr) => !!st.nodeBad[addr];
  const linkBad = (k) => st.linkSt[k].bad;
  const match = {};
  Object.keys(nodes).forEach((addr) => { if (nodeMatches(nodes[addr])) match[addr] = true; });
  let vis = {};
  Object.keys(match).forEach((a) => { vis[a] = 1; });
  if (state.group || state.qAddr) {
    // 筛选时把命中节点的链路对端也带上(半透明上下文), 否则只剩孤立的点看不出监测关系
    Object.values(links).forEach((l) => {
      if (match[l.from] && !match[l.to] && !vis[l.to]) vis[l.to] = 2;
      if (match[l.to] && !match[l.from] && !vis[l.from]) vis[l.from] = 2;
    });
  }
  if (state.badOnly) {
    // 只保留异常链路两端的节点与本身异常(不可达)的节点
    const keep = {};
    Object.keys(links).forEach((k) => {
      const l = links[k];
      if (linkBad(k) && vis[l.from] && vis[l.to]) { keep[l.from] = vis[l.from]; keep[l.to] = vis[l.to]; }
    });
    Object.keys(nodes).forEach((addr) => { if (isBad(addr) && vis[addr]) keep[addr] = vis[addr]; });
    vis = keep;
  }
  const cnt = Object.keys(vis).length;

  // 节点数量自适应: 节点越多, 图标越小; 超过 60 个隐藏常显名称(悬浮/异常时仍显示)
  const size = Math.max(14, Math.round(baseSize * Math.min(1, Math.sqrt(36 / Math.max(cnt, 1)))));
  const showLabel = cnt <= 60;

  const idx = {};
  const colors = ['#6366f1', '#10b981', '#f43f5e'];
  Object.keys(nodes).forEach((addr) => {
    if (!vis[addr]) return;
    const n = nodes[addr];
    const bad = isBad(addr);
    const ctx = vis[addr] === 2; // 上下文节点(链路对端), 半透明展示
    const cat = bad ? 2 : (n.source ? 0 : 1);
    idx[addr] = data.length;
    data.push({
      name: n.name + ' (' + addr + ')', _label: n.name, _tipName: n.name, category: cat, draggable: true, _addr: addr,
      symbolSize: n.source ? size : Math.max(12, Math.round(size * 0.72)),
      label: { show: showLabel || bad },
      _sub: '<span class="mono">' + esc(addr) + '</span><br>' +
        (bad ? '节点不可达' : (n.source ? '探测节点' : '被测目标')) +
        (n.group ? ' · ' + esc(n.group) : '') +
        (n.missing ? '<br><span style="color:#fbbf24">未注册节点(仅存在于监测规则中)</span>' : ''),
      itemStyle: {
        color: colors[cat],
        opacity: ctx ? 0.45 : 1,
        borderColor: '#fff', borderWidth: cnt > 60 ? 1.5 : 3,
        shadowBlur: cnt > 60 ? 6 : 14, shadowColor: colors[cat] + '66',
      },
    });
  });
  // 连线数量自适应透明度, 大规模 mesh 不至于糊成一团
  const visLinks = [];
  Object.keys(links).forEach((k) => {
    const l = links[k];
    if (idx[l.from] === undefined || idx[l.to] === undefined) return;
    if (state.badOnly && !linkBad(k)) return;
    visLinks.push(k);
  });
  const op = visLinks.length > 300 ? 0.3 : (visLinks.length > 120 ? 0.5 : 0.85);
  visLinks.forEach((k) => {
    const l = links[k];
    const { bad, pending } = st.linkSt[k];
    lns.push({
      source: idx[l.from], target: idx[l.to],
      _bad: bad, _diag: true,
      _from: l.from, _to: l.to,
      _fromName: nodes[l.from].name, _toName: nodes[l.to].name,
      lineStyle: {
        color: bad ? '#f43f5e' : (pending ? '#cbd5e1' : '#34d399'),
        width: bad ? baseLine * 1.4 : baseLine,
        opacity: bad ? 0.95 : op,
        type: bad ? 'dashed' : 'solid',
        curveness: 0.25, shadowBlur: bad ? 6 : 0, shadowColor: 'rgba(244,63,94,.4)',
      },
    });
  });
  return showLabel ? '' : '节点较多(' + cnt + ' 个)已隐藏名称，悬浮可查看；建议用「按分组聚合」或分组筛选缩小范围';
}

function buildGroupGraph(model, st, state, data, lns, baseSize, baseLine) {
  const { nodes, links, groupNames } = model;
  const gs = {};
  Object.keys(nodes).forEach((addr) => {
    const n = nodes[addr];
    const g = groupOf(n);
    const e = gs[g] = gs[g] || { count: 0, badNodes: 0, probes: 0 };
    e.count++;
    if (st.nodeBad[addr]) e.badNodes++;
    if (n.source) e.probes++;
  });
  const ge = {}; // "a|b" -> {total, bad}  跨组链路(按无序组对聚合)
  Object.keys(links).forEach((k) => {
    const l = links[k];
    const gf = groupOf(nodes[l.from]);
    const gt = groupOf(nodes[l.to]);
    const inner = gf === gt;
    let key;
    if (inner) key = gf + '|' + gf;
    else key = gf < gt ? gf + '|' + gt : gt + '|' + gf;
    const e = ge[key] = ge[key] || { total: 0, bad: 0, inner };
    e.total++;
    if (st.linkSt[k].bad) e.bad++;
  });

  const idx = {};
  groupNames.forEach((g) => {
    const e = gs[g];
    if (!e) return;
    const innerKey = g + '|' + g;
    const innerBad = ge[innerKey] ? ge[innerKey].bad : 0;
    const bad = e.badNodes > 0 || innerBad > 0;
    idx[g] = data.length;
    data.push({
      name: g, _label: g + ' (' + e.count + ')', _tipName: g, category: bad ? 2 : 3,
      draggable: true, _group: g,
      symbolSize: Math.min(76, 30 + Math.round(Math.sqrt(e.count) * 7)),
      label: { show: true },
      _sub: '成员 ' + e.count + ' 个(探测节点 ' + e.probes + ')' +
        (ge[innerKey] ? '<br>组内链路 ' + ge[innerKey].total + ' 条' + (innerBad ? '，<b style="color:#fb7185">异常 ' + innerBad + ' 条</b>' : '') : '') +
        (e.badNodes ? '<br><b style="color:#fb7185">不可达节点 ' + e.badNodes + ' 个</b>' : '') +
        '<br><span style="color:#a5b4fc">点击下钻查看本组节点</span>',
      itemStyle: {
        color: bad ? '#f43f5e' : '#0ea5e9',
        borderColor: '#fff', borderWidth: 3,
        shadowBlur: 14, shadowColor: (bad ? '#f43f5e' : '#0ea5e9') + '66',
      },
    });
  });
  Object.keys(ge).forEach((key) => {
    const e = ge[key];
    if (e.inner) return;
    const parts = key.split('|');
    if (idx[parts[0]] === undefined || idx[parts[1]] === undefined) return;
    if (state.badOnly && e.bad === 0) return;
    lns.push({
      source: idx[parts[0]], target: idx[parts[1]],
      symbol: ['none', 'none'], // 聚合链路是双向汇总, 不画箭头
      _agg: true, _gtotal: e.total, _gbad: e.bad,
      _fromName: parts[0], _toName: parts[1],
      lineStyle: {
        color: e.bad > 0 ? '#f43f5e' : '#34d399',
        width: Math.min(6, baseLine + Math.log(e.total + 1)),
        opacity: 0.8,
        type: e.bad > 0 ? 'dashed' : 'solid',
        curveness: 0.2, shadowBlur: e.bad > 0 ? 6 : 0, shadowColor: 'rgba(244,63,94,.4)',
      },
    });
  });
  if (state.badOnly) {
    // 只看异常: 去掉既无异常也未挂在异常链路上的组
    const used = {};
    lns.forEach((l) => { used[l.source] = 1; used[l.target] = 1; });
    const data2 = [];
    const remap = {};
    data.forEach((d, i) => {
      if (d.category === 2 || used[i]) { remap[i] = data2.length; data2.push(d); }
    });
    lns.forEach((l) => { l.source = remap[l.source]; l.target = remap[l.target]; });
    data.length = 0;
    data2.forEach((d) => data.push(d));
  }
  return '';
}

/** 返回 { data, links, hint, error } */
export function buildGraph(model, st, state, topoCfg) {
  // 「基础设置 - 拓扑节点大小 / 连线粗细」生效
  let baseSize = parseInt(topoCfg.Tsymbolsize || 56, 10);
  if (!(baseSize > 0)) baseSize = 56;
  let baseLine = parseFloat(topoCfg.Tline || 1.8);
  if (!(baseLine > 0)) baseLine = 1.8;
  const data = [];
  const lns = [];
  try {
    const hint = state.view === 'group' && model.groupNames.length >= 2
      ? buildGroupGraph(model, st, state, data, lns, baseSize, baseLine)
      : buildNodeGraph(model, st, state, data, lns, baseSize, baseLine);
    return { data, links: lns, hint, error: '' };
  } catch (e) {
    return { data: null, links: null, hint: '', error: '拓扑渲染失败: ' + e.message };
  }
}

function tooltipFormatter(p) {
  if (p.dataType === 'edge') {
    const d = p.data || {};
    if (d._agg) {
      return esc(d._fromName) + ' &harr; ' + esc(d._toName) + '<br>' +
        '链路 ' + d._gtotal + ' 条' +
        (d._gbad > 0 ? '，<b style="color:#fb7185">异常 ' + d._gbad + ' 条</b>' : '，<span style="color:#34d399">全部正常</span>');
    }
    return esc(d._fromName) + ' &rarr; ' + esc(d._toName) + '<br>' +
      (d._bad ? '<b style="color:#fb7185">所选时间内触发过告警</b><br><span style="color:#94a3b8">点击查看触发时刻</span>' : '<span style="color:#34d399">监测正常</span>');
  }
  let html = '<b>' + esc(p.data._tipName || p.name) + '</b><br>' + (p.data._sub || '');
  const info = asnCached(p.data._addr || '');
  if (info) html += '<br><span style="color:#a5b4fc">' + esc(asnShort(info)) + '</span>';
  return html;
}

export function baseOption() {
  return {
    tooltip: {
      show: true, backgroundColor: 'rgba(15,23,42,.92)', borderWidth: 0,
      padding: [9, 13], textStyle: { color: '#e2e8f0', fontSize: 12 },
      formatter: tooltipFormatter,
    },
    legend: {
      show: true, top: 0, right: 10, itemWidth: 14, itemHeight: 10,
      icon: 'circle', textStyle: { color: '#64748b', fontSize: 12 },
      data: ['探测节点', '被测目标', '异常', '分组'],
    },
    animationDuration: 300, animationDurationUpdate: 300,
    series: [{
      // 环形布局: 节点均匀分布在圆环上, 互PING连线交织在中间, mesh 结构一目了然
      type: 'graph', layout: 'circular', roam: true, draggable: true,
      circular: { rotateLabel: false },
      scaleLimit: { min: 0.4, max: 5 },
      categories: [
        { name: '探测节点', itemStyle: { color: '#6366f1' } },
        { name: '被测目标', itemStyle: { color: '#10b981' } },
        { name: '异常', itemStyle: { color: '#f43f5e' } },
        { name: '分组', itemStyle: { color: '#0ea5e9' } },
      ],
      emphasis: { focus: 'adjacency', lineStyle: { width: 4 }, itemStyle: { shadowBlur: 18 }, label: { show: true } },
      label: {
        show: true, position: 'bottom', distance: 6,
        fontSize: 12, fontWeight: 600, color: '#334155',
        formatter: (p) => {
          const name = p.data._label || p.name;
          return name.length > 14 ? name.substring(0, 13) + '…' : name;
        },
      },
      edgeSymbol: ['none', 'arrow'], edgeSymbolSize: [0, 9],
      data: [], links: [],
      lineStyle: { opacity: 0.85, curveness: 0.25 },
    }],
  };
}
