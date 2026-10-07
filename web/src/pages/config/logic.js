// 配置页的纯逻辑: 与旧版 config.html 内联脚本逐行对应。
// 保存的 JSON 按对象键的插入顺序序列化, 因此这里对工作副本的每一步修改(包括新增键的先后)都与旧版保持一致。
import { nodeName } from '../../api';

export const deepCopy = (o) => JSON.parse(JSON.stringify(o));

const IP_RE = /^(\d{1,3}\.){3}\d{1,3}$/;
const DOMAIN_RE = /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)+$/;
export const isIp = (a) => IP_RE.test(a);
export const validAddr = (a) => IP_RE.test(a) || DOMAIN_RE.test(a);

// jQuery $.trim
export const trim = (s) => (s == null ? '' : String(s).trim());

// 浏览器对 <input type=number> 的取值净化: 非合法浮点数字符串读回为 ''
const FLOAT_RE = /^-?(?:\d+(?:\.\d+)?|\.\d+)(?:[eE][-+]?\d+)?$/;
export function numVal(v) {
  if (v === undefined || v === null) return '';
  const s = String(v);
  return FLOAT_RE.test(s) ? s : '';
}
// <input type=text|password> 的取值净化: 去掉换行
export function txtVal(v) {
  if (v === undefined || v === null) return '';
  return String(v).replace(/[\r\n]/g, '');
}
// <select>.val(x): 无匹配项时浏览器回落到第一个选项
export function selVal(v, values) {
  const s = v === undefined || v === null ? '' : String(v);
  return values.indexOf(s) >= 0 ? s : values[0];
}

export const REMIND_VALUES = ['0', '30', '60', '360', '1440'];
export const ROLE_VALUES = ['viewer', 'admin'];

/* ----- fill(): 工作副本 → 表单值 (同时补齐旧版在 fill 中添加的键: OAuth / Channels / Chinamap) ----- */
export function fillForm(cfg) {
  const brand = cfg.Brand || null;
  const base = cfg.Base || {};
  const topo = cfg.Topology || {};
  const alert = cfg.Alert || {};
  const f = {
    brandName: txtVal((brand && brand.Name) || ''),
    brandSlogan: txtVal((brand && brand.Slogan) || ''),
    brandLogo: (brand && brand.Logo) || '',
    name: txtVal(cfg.Name),
    addr: txtVal(cfg.Addr),
    timeout: numVal(base.Timeout),
    archive: numVal(base.Archive),
    refresh: numVal(base.Refresh),
    toollimit: numVal(cfg.Toollimit),
    tline: numVal(topo.Tline),
    tsymbolsize: numVal(topo.Tsymbolsize),
    tsound: txtVal(topo.Tsound),
    authiplist: txtVal(cfg.Authiplist),
  };
  cfg.OAuth = cfg.OAuth || {};
  const oa = cfg.OAuth;
  Object.assign(f, {
    oauthEnabled: oa.Enabled === '1' || String(oa.Enabled).toLowerCase() === 'true',
    oauthClientId: txtVal(oa.ClientId || ''),
    oauthClientSecret: txtVal(oa.ClientSecret || ''),
    oauthTenantId: txtVal(oa.TenantId || 'organizations'),
    oauthDomains: txtVal(oa.AllowedDomains || ''),
    oauthAutoCreate: oa.AutoCreate !== '0' && String(oa.AutoCreate).toLowerCase() !== 'false',
    oauthRole: selVal(oa.DefaultRole || 'viewer', ROLE_VALUES),
    chinamap: base.Chinamap !== 0,
    pinginterval: numVal(base.Pinginterval || 2500),
    pingcount: numVal(base.Pingcount || 20),
    pingtimeout: numVal(base.Pingtimeout || 3000),
    pingsize: numVal(base.Pingsize || 56),
    emailhost: txtVal(alert.EmailHost),
    emailaccount: txtVal(alert.SendEmailAccount),
    emailpassword: txtVal(alert.SendEmailPassword),
    emaillist: txtVal(alert.RevcEmailList),
    jointoken: txtVal(cfg.Password || ''),
    apisign: base.Apisign === 1,
    remindmin: selVal(String(base.Remindmin !== undefined ? base.Remindmin : 60), REMIND_VALUES),
  });
  if (!cfg.Channels) cfg.Channels = [];
  if (!cfg.Chinamap) cfg.Chinamap = {};
  return f;
}

/* ----- collectFromForms(): 表单值 → 工作副本 (原地修改, 抛错时已做的修改保留, 与旧版一致) ----- */
export function collectFromForms(cfg, f) {
  const newName = trim(f.name);
  const newAddr = trim(f.addr);
  if (newAddr !== cfg.Addr) {
    if (!validAddr(newAddr)) throw new Error('本机节点 IP/域名 格式非法: ' + newAddr);
    if (cfg.Network[newAddr]) throw new Error('本机新地址与现有节点「' + nodeName(cfg, newAddr) + '」冲突');
    const self = cfg.Network[cfg.Addr];
    if (self) {
      delete cfg.Network[cfg.Addr];
      const oldAddr = cfg.Addr;
      self.Addr = newAddr;
      cfg.Network[newAddr] = self;
      Object.keys(cfg.Network).forEach((k) => {
        const m = cfg.Network[k];
        m.Ping = (m.Ping || []).map((x) => (x === oldAddr ? newAddr : x));
        (m.Topology || []).forEach((t) => { if (t.Addr === oldAddr) t.Addr = newAddr; });
      });
    }
  }
  cfg.Addr = newAddr;
  if (cfg.Network[cfg.Addr] && newName !== '') {
    cfg.Network[cfg.Addr].Name = newName;
    Object.keys(cfg.Network).forEach((k) => {
      (cfg.Network[k].Topology || []).forEach((t) => { if (t.Addr === cfg.Addr) t.Name = newName; });
    });
  }
  cfg.Name = newName;
  const B = cfg.Base;
  B.Timeout = parseInt(f.timeout, 10) || 5;
  B.Archive = parseInt(f.archive, 10) || 10;
  B.Refresh = parseInt(f.refresh, 10) || 1;
  B.Chinamap = f.chinamap ? 1 : 0;
  B.Pinginterval = parseInt(f.pinginterval, 10) || 2500;
  B.Pingcount = parseInt(f.pingcount, 10) || 20;
  B.Pingtimeout = parseInt(f.pingtimeout, 10) || 3000;
  B.Pingsize = parseInt(f.pingsize, 10) || 56;
  B.Apisign = f.apisign ? 1 : 0;
  B.Remindmin = parseInt(f.remindmin, 10) || 0;
  if (B.Pinginterval * B.Pingcount > 55000) {
    const tot = Math.round((B.Pinginterval * B.Pingcount) / 1000);
    throw new Error('探测参数组合无效: 每分钟探测一轮, 一轮要发完全部包。当前 间隔' + B.Pinginterval + 'ms × ' + B.Pingcount + '包 = ' + tot + '秒, 超过55秒上限。改成例如 1000ms×30包 或 500ms×60包 即可');
  }
  cfg.Toollimit = parseInt(f.toollimit, 10) || 0;
  cfg.Topology.Tline = String(f.tline || '1');
  cfg.Topology.Tsymbolsize = String(f.tsymbolsize || '70');
  cfg.Topology.Tsound = f.tsound;
  cfg.Authiplist = trim(f.authiplist);
  cfg.OAuth = cfg.OAuth || {};
  const oa = cfg.OAuth;
  oa.Enabled = f.oauthEnabled ? '1' : '0';
  oa.ClientId = trim(f.oauthClientId);
  const sec = f.oauthClientSecret;
  if (sec) oa.ClientSecret = sec;
  else if (oa.ClientSecret) oa.ClientSecret = 'samepasswordasbefore';
  oa.TenantId = trim(f.oauthTenantId) || 'organizations';
  oa.AllowedDomains = trim(f.oauthDomains);
  oa.AutoCreate = f.oauthAutoCreate ? '1' : '0';
  oa.DefaultRole = f.oauthRole || 'viewer';
  cfg.Alert.EmailHost = trim(f.emailhost);
  cfg.Alert.SendEmailAccount = trim(f.emailaccount);
  cfg.Alert.SendEmailPassword = f.emailpassword;
  cfg.Alert.RevcEmailList = trim(f.emaillist);
  cfg.Password = trim(f.jointoken);
  cfg.Brand = cfg.Brand || {};
  cfg.Brand.Name = trim(f.brandName);
  cfg.Brand.Slogan = trim(f.brandSlogan);
  cfg.Brand.Logo = f.brandLogo || '';
  return cfg;
}

/* ----- 节点 ----- */
export const groupOf = (n) => (n && n.Group ? n.Group : '未分组');

export function groupNames(cfg) {
  const groups = {};
  Object.keys(cfg.Network).forEach((k) => { const m = cfg.Network[k]; if (m.Group) groups[m.Group] = 1; });
  return Object.keys(groups);
}

// 删除一批节点并清理其他节点对它们的监测引用(本机节点永不删除)
export function removeNodes(cfg, addrs) {
  const del = {};
  addrs.forEach((a) => {
    if (a === cfg.Addr) return;
    del[a] = true;
    delete cfg.Network[a];
  });
  Object.keys(cfg.Network).forEach((k) => {
    const n = cfg.Network[k];
    n.Ping = (n.Ping || []).filter((t) => !del[t]);
    n.Topology = (n.Topology || []).filter((t) => !del[t.Addr]);
  });
}

// row: 监测目标表的一行(界面值均为字符串); 无 row 时使用默认阈值
export function topoEntry(addr, name, row) {
  return {
    Addr: addr, Name: name,
    Thdavgdelay: String(row ? (row.delay || 200) : 200),
    Thdloss: String(row ? (row.loss || 30) : 30),
    Thdjitter: trim(row ? (row.jitter || '') : ''),
    Thdchecksec: String(row ? (row.sec || 900) : 900),
    Thdoccnum: String(row ? (row.num || 3) : 3),
    Srcip: trim(row ? (row.src || '') : ''),
    Pinterval: trim(row ? row.pint || '' : ''),
    Pcount: trim(row ? row.pcnt || '' : ''),
    Ptimeout: trim(row ? row.ptimeout || '' : ''),
    Psize: trim(row ? row.psize || '' : ''),
  };
}

// 监测目标表: 按分组归类(未分组排最后), 组内探测节点在前、其余按名称
export function buildTargetRows(cfg, n) {
  const topoMap = {};
  (n.Topology || []).forEach((t) => { topoMap[t.Addr] = t; });
  const byGroup = {};
  Object.keys(cfg.Network).forEach((addr) => {
    const other = cfg.Network[addr];
    if (addr === n.Addr && n.Addr !== '') return;
    const g = groupOf(other);
    (byGroup[g] = byGroup[g] || []).push([addr, other]);
  });
  const groups = Object.keys(byGroup);
  groups.sort((a, b) => {
    if (a === '未分组') return 1;
    if (b === '未分组') return -1;
    return a < b ? -1 : 1;
  });
  const out = [];
  groups.forEach((g) => {
    const rows = byGroup[g];
    rows.sort((a, b) => {
      if (a[1].Pingmesh !== b[1].Pingmesh) return a[1].Pingmesh ? -1 : 1;
      return a[1].Name < b[1].Name ? -1 : 1;
    });
    out.push({ group: g, rows: rows.map(([addr, other]) => {
      const t = topoMap[addr] || {};
      return {
        addr, g, name: other.Name, probe: !!other.Pingmesh,
        s: (other.Name + ' ' + addr + ' ' + g).toLowerCase(),
        on: (n.Ping || []).indexOf(addr) >= 0 || !!topoMap[addr],
        mutual: other.Pingmesh ? (n.Addr !== '' && (other.Ping || []).indexOf(n.Addr) >= 0) : null,
        delay: numVal(String(t.Thdavgdelay || 200)),
        loss: numVal(String(t.Thdloss || 30)),
        jitter: numVal(String(t.Thdjitter || '')),
        sec: numVal(String(t.Thdchecksec || 900)),
        num: numVal(String(t.Thdoccnum || 3)),
        src: txtVal(t.Srcip || ''),
        pint: numVal(t.Pinterval || ''),
        pcnt: numVal(t.Pcount || ''),
        ptimeout: numVal(t.Ptimeout || ''),
        psize: numVal(t.Psize || ''),
        open: false,
      };
    }) });
  });
  return out;
}

export function buildWatcherRows(cfg, n) {
  const out = [];
  Object.keys(cfg.Network).forEach((addr) => {
    const other = cfg.Network[addr];
    if (!other.Pingmesh || (addr === n.Addr && n.Addr !== '')) return;
    out.push({ addr, name: other.Name, group: groupOf(other), on: n.Addr !== '' && (other.Ping || []).indexOf(n.Addr) >= 0 });
  });
  return out;
}

export function probeSummary(t) {
  const has = t.pint || t.pcnt || t.ptimeout || t.psize;
  if (!has) return null;
  return (t.pint || '默') + 'ms×' + (t.pcnt || '默') + ' / ' + (t.ptimeout || '默') + 'ms / ' + (t.psize || '默') + 'B';
}

// 节点弹窗「确定」: 返回错误文本或 null; 成功时已写入工作副本
export function saveNode(cfg, editingAddr, d, groups, watchers) {
  const name = trim(d.name);
  const addr = trim(d.addr);
  const group = trim(d.group);
  const probe = d.type === 'probe';
  if (!name) return '请输入节点名称';
  if (!validAddr(addr)) return '请输入合法的 IPv4 地址或域名';
  if (addr !== editingAddr && cfg.Network[addr]) return '该 IP 已存在: ' + nodeName(cfg, addr);

  if (editingAddr && addr !== editingAddr) {
    delete cfg.Network[editingAddr];
    Object.keys(cfg.Network).forEach((k) => {
      const m = cfg.Network[k];
      m.Ping = (m.Ping || []).map((x) => (x === editingAddr ? addr : x));
      (m.Topology || []).forEach((t) => { if (t.Addr === editingAddr) t.Addr = addr; });
    });
  }

  const rows = [];
  groups.forEach((gr) => gr.rows.forEach((r) => rows.push(r)));
  const ping = [];
  const topo = [];
  if (probe) {
    for (let i = 0; i < rows.length; i++) {
      const r = rows[i];
      if (!r.on) continue;
      const entry = topoEntry(r.addr, nodeName(cfg, r.addr), r);
      const sec = parseInt(entry.Thdchecksec, 10);
      const num = parseInt(entry.Thdoccnum, 10);
      if (num > 0 && Math.floor(sec / 60) < num) {
        return nodeName(cfg, r.addr) + ' 的告警规则无效: 检测窗口是"判定告警时回看多久的数据"(数据每分钟1个点), ' +
          sec + ' 秒装不下 ' + num + ' 个异常分钟。建议: 窗口 900 / 次数 3(15分钟内3个异常分钟告警); 若想调高探测频率, 请改「探测参数」里的探测间隔';
      }
      ping.push(r.addr);
      topo.push(entry);
    }
  }
  cfg.Network[addr] = { Name: name, Addr: addr, Group: group, Pingmesh: probe, Ping: ping, Topology: topo };

  const sync = (otherAddr, want) => {
    const other = cfg.Network[otherAddr];
    if (!other || !other.Pingmesh) return;
    other.Ping = other.Ping || [];
    other.Topology = other.Topology || [];
    const has = other.Ping.indexOf(addr) >= 0;
    if (want && !has) {
      other.Ping.push(addr);
      other.Topology.push(topoEntry(addr, name, null));
    } else if (!want && has) {
      other.Ping = other.Ping.filter((x) => x !== addr);
      other.Topology = other.Topology.filter((t) => t.Addr !== addr);
    }
  };
  if (probe) rows.forEach((r) => { if (r.mutual !== null) sync(r.addr, r.mutual); });
  else watchers.forEach((w) => sync(w.addr, w.on));

  Object.keys(cfg.Network).forEach((k) => {
    (cfg.Network[k].Topology || []).forEach((t) => { if (t.Addr === addr) t.Name = name; });
  });
  return null;
}

/* ----- 全球延迟 ----- */
export function parseIps(text) {
  const out = [];
  const seen = {};
  text.split(/[\s,;，；]+/).forEach((raw) => {
    const ip = trim(raw);
    if (ip === '' || seen[ip]) return;
    if (!/^(\d{1,3}\.){3}\d{1,3}$/.test(ip)) throw new Error('非法 IP: ' + ip);
    seen[ip] = true;
    out.push(ip);
  });
  return out;
}

export function removeGmEntry(cfg, r, l) {
  if (!cfg.Chinamap[r]) return;
  delete cfg.Chinamap[r][l];
  if (Object.keys(cfg.Chinamap[r]).length === 0) delete cfg.Chinamap[r];
}

export function gmEntries(cfg) {
  const out = [];
  const map = cfg.Chinamap || {};
  Object.keys(map).sort().forEach((region) => {
    Object.keys(map[region]).sort().forEach((line) => {
      out.push({ region, line, ips: map[region][line] || [] });
    });
  });
  return out;
}

/* ----- 告警通道 ----- */
export const CH_FIELDS = {
  dingtalk: [{ k: 'Url', l: '机器人 Webhook 地址', ph: 'https://oapi.dingtalk.com/robot/send?access_token=...' },
    { k: 'Secret', l: '加签密钥（安全设置选「加签」时填写，可选）', ph: 'SEC...' }],
  wecom: [{ k: 'Url', l: '机器人 Webhook 地址', ph: 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...' }],
  feishu: [{ k: 'Url', l: '机器人 Webhook 地址', ph: 'https://open.feishu.cn/open-apis/bot/v2/hook/...' },
    { k: 'Secret', l: '签名校验密钥（可选）', ph: '' }],
  telegram: [{ k: 'Token', l: 'Bot Token', ph: '123456:ABC-DEF...' },
    { k: 'ChatId', l: 'Chat ID', ph: '-1001234567890' }],
  slack: [{ k: 'Url', l: 'Incoming Webhook 地址', ph: 'https://hooks.slack.com/services/...' }],
  discord: [{ k: 'Url', l: 'Webhook 地址', ph: 'https://discord.com/api/webhooks/...' }],
  webhook: [{ k: 'Url', l: '回调地址（POST JSON：event/title/content/源/目标/时间）', ph: 'https://your-system/alert' }],
};
export const CH_TYPES = [
  ['dingtalk', '钉钉机器人'], ['wecom', '企业微信机器人'], ['feishu', '飞书机器人'], ['telegram', 'Telegram Bot'],
  ['slack', 'Slack Incoming Webhook'], ['discord', 'Discord Webhook'], ['webhook', '通用 Webhook (JSON)'],
];
export const CH_TYPE_NAMES = { dingtalk: '钉钉', wecom: '企业微信', feishu: '飞书', telegram: 'Telegram', slack: 'Slack', discord: 'Discord', webhook: 'Webhook' };

export function randomToken() {
  let s = '';
  const chars = 'abcdefghijkmnpqrstuvwxyz23456789ABCDEFGHJKLMNPQRSTUVWXYZ';
  for (let i = 0; i < 20; i++) s += chars.charAt(Math.floor(Math.random() * chars.length));
  return s;
}
