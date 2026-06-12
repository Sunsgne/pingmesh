/* SmartPing Modern UI - shared shell & helpers */
var SP = (function () {

    var icons = {
        dashboard: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="9" rx="1"/><rect x="14" y="3" width="7" height="5" rx="1"/><rect x="14" y="12" width="7" height="9" rx="1"/><rect x="3" y="16" width="7" height="5" rx="1"/></svg>',
        mesh: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M3 9h18M3 15h18M9 3v18M15 3v18"/></svg>',
        reverse: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 14 4 9 9 4"/><path d="M20 20v-7a4 4 0 0 0-4-4H4"/></svg>',
        topo: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="5" cy="6" r="3"/><circle cx="19" cy="6" r="3"/><circle cx="12" cy="18" r="3"/><path d="M7.5 8 10 15M16.5 8 14 15M8 6h8"/></svg>',
        map: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="1 6 8 3 16 6 23 3 23 18 16 21 8 18 1 21"/><line x1="8" y1="3" x2="8" y2="18"/><line x1="16" y1="6" x2="16" y2="21"/></svg>',
        tools: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>',
        alert: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>',
        config: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>',
        users: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/></svg>'
    };

    var navItems = [
        { group: '监控' },
        { id: 'index', href: 'index.html', title: '概览', icon: 'dashboard' },
        { id: 'pingmesh', href: 'pingmesh.html', title: 'Pingmesh', icon: 'mesh' },
        { id: 'reverse', href: 'reverse.html', title: '反向 Ping', icon: 'reverse' },
        { id: 'topology', href: 'topology.html', title: '网络拓扑', icon: 'topo' },
        { id: 'mapping', href: 'mapping.html', title: '全球延迟', icon: 'map' },
        { group: '运维' },
        { id: 'tools', href: 'tools.html', title: '检测工具', icon: 'tools' },
        { id: 'alerts', href: 'alerts.html', title: '报警记录', icon: 'alert' },
        { group: '管理', admin: true },
        { id: 'config', href: 'config.html', title: '系统配置', icon: 'config', admin: true },
        { id: 'users', href: 'users.html', title: '用户管理', icon: 'users', admin: true }
    ];

    var S = { user: null, config: null, page: '', title: '' };

    function esc(s) {
        return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
            return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
        });
    }

    /* ---------- toast ---------- */
    function toast(msg, type) {
        if ($('#sp-toasts').length === 0) $('body').append('<div id="sp-toasts"></div>');
        var cls = type === 'ok' ? 'ok' : (type === 'err' ? 'err' : '');
        var el = $('<div class="sp-toast ' + cls + '">' + esc(msg) + '</div>');
        $('#sp-toasts').append(el);
        setTimeout(function () { el.fadeOut(250, function () { el.remove(); }); }, 3200);
    }

    /* ---------- modal ---------- */
    function openModal(id) { $('#' + id).addClass('open'); }
    function closeModal(id) { $('#' + id).removeClass('open'); }
    // 点击遮罩不关闭弹窗(防误触), 仅通过 × / 取消 按钮主动关闭
    $(document).on('click', '.m-close', function () { $(this).closest('.sp-modal-mask').removeClass('open'); });

    function confirmBox(title, msg, cb) {
        $('#sp-confirm').remove();
        var html =
            '<div class="sp-modal-mask open" id="sp-confirm"><div class="sp-modal" style="max-width:400px">' +
            '<div class="m-head"><h3>' + esc(title) + '</h3><button class="m-close">&times;</button></div>' +
            '<div class="m-body">' + esc(msg) + '</div>' +
            '<div class="m-foot"><button class="btn" id="sp-confirm-no">取消</button>' +
            '<button class="btn danger" id="sp-confirm-yes">确定</button></div></div></div>';
        $('body').append(html);
        $('#sp-confirm-no').click(function () { $('#sp-confirm').remove(); });
        $('#sp-confirm-yes').click(function () { $('#sp-confirm').remove(); cb(); });
    }

    /* ---------- helpers ---------- */
    function proxy(addr, port, path) {
        return '/api/proxy.json?g=' + 'http://' + addr + ':' + port + path;
    }
    function fmtMs(v) {
        v = parseFloat(v);
        if (isNaN(v)) return '-';
        return v >= 100 ? v.toFixed(0) : v.toFixed(1);
    }
    function delayLevel(delay, loss) {
        // flashcat pingmesh 风格的健康度分级
        if (loss >= 50) return 4;
        if (loss >= 20) return 3;
        if (delay >= 300) return 4;
        if (delay >= 150) return 3;
        if (delay >= 80 || loss >= 5) return 2;
        if (delay >= 30) return 1;
        return 0;
    }

    /* ---------- shared big ping chart modal ---------- */
    function ensureChartModal() {
        if ($('#sp-chart-modal').length) return;
        var html =
            '<div class="sp-modal-mask" id="sp-chart-modal"><div class="sp-modal lg">' +
            '<div class="m-head"><h3 id="sp-chart-title">历史曲线</h3><button class="m-close">&times;</button></div>' +
            '<div class="m-body">' +
            '<div class="flex mb-4">' +
            '<label class="muted" style="font-size:12.5px">开始</label>' +
            '<input type="datetime-local" class="input" id="sp-chart-start" style="width:200px">' +
            '<label class="muted" style="font-size:12.5px">结束</label>' +
            '<input type="datetime-local" class="input" id="sp-chart-end" style="width:200px">' +
            '<button class="btn primary sm" id="sp-chart-query">查询</button>' +
            '<span class="spacer"></span>' +
            '<span class="muted" id="sp-chart-loading"></span>' +
            '</div>' +
            '<div id="sp-chart-box" style="width:100%;height:420px"></div>' +
            '</div></div></div>';
        $('body').append(html);
        $('#sp-chart-query').click(function () {
            var st = $('#sp-chart-start').val().replace('T', ' ');
            var et = $('#sp-chart-end').val().replace('T', ' ');
            loadChart(S._chartUrl, st, et);
        });
    }

    var bigChart = null;
    function loadChart(apiurl, start, end) {
        var url = apiurl;
        if (start && end) {
            var sep = apiurl.indexOf('proxy.json') >= 0 ? '%26' : '&';
            url += sep + 'starttime=' + encodeURIComponent(start) + sep + 'endtime=' + encodeURIComponent(end);
        }
        $('#sp-chart-loading').html('<span class="spinner"></span>');
        $.getJSON(url).done(function (data) {
            $('#sp-chart-loading').html('');
            bigChart.setOption({
                xAxis: { data: data.lastcheck },
                series: [
                    { name: '最大延迟', data: data.maxdelay },
                    { name: '最小延迟', data: data.mindelay },
                    { name: '平均延迟', data: data.avgdelay },
                    { name: '丢包率', data: data.losspk },
                    { name: '抖动', data: data.jitter || [] }
                ]
            });
        }).fail(function () {
            $('#sp-chart-loading').html('');
            toast('获取数据失败', 'err');
        });
    }

    function openPingChart(title, apiurl) {
        ensureChartModal();
        S._chartUrl = apiurl;
        $('#sp-chart-title').text(title);
        openModal('sp-chart-modal');
        if (!bigChart) {
            bigChart = echarts.init(document.getElementById('sp-chart-box'));
            bigChart.setOption({
                tooltip: {
                    trigger: 'axis',
                    backgroundColor: 'rgba(15,23,42,.92)', borderWidth: 0, padding: [10, 14],
                    textStyle: { color: '#e2e8f0', fontSize: 12 },
                    axisPointer: { type: 'cross', label: { backgroundColor: '#4f46e5' } }
                },
                legend: {
                    data: ['最大延迟', '平均延迟', '最小延迟', '丢包率', '抖动'],
                    selected: { '最大延迟': false, '最小延迟': false },
                    icon: 'roundRect', itemWidth: 14, itemHeight: 8,
                    textStyle: { color: '#475569', fontSize: 12 }
                },
                grid: { left: 52, right: 58, top: 48, bottom: 62 },
                dataZoom: [{ height: 22, bottom: 14, borderColor: 'transparent', backgroundColor: '#f1f5f9', fillerColor: 'rgba(99,102,241,.15)', handleStyle: { color: '#6366f1' } }],
                xAxis: {
                    data: [], boundaryGap: false,
                    axisLine: { lineStyle: { color: '#e2e8f0' } },
                    axisLabel: { color: '#94a3b8', fontSize: 11 }, axisTick: { show: false }
                },
                yAxis: [
                    { type: 'value', name: '延迟(ms)', position: 'left', nameTextStyle: { color: '#94a3b8' },
                      axisLabel: { color: '#94a3b8', fontSize: 11 }, splitLine: { lineStyle: { color: '#f1f5f9' } } },
                    { type: 'value', name: '丢包(%)', min: 0, max: 100, position: 'right', nameTextStyle: { color: '#94a3b8' },
                      axisLabel: { formatter: '{value}%', color: '#94a3b8', fontSize: 11 }, splitLine: { show: false } }
                ],
                series: [
                    { name: '最大延迟', type: 'line', animation: false, showSymbol: false, smooth: true, connectNulls: true,
                      itemStyle: { color: '#a5b4fc' }, areaStyle: { opacity: .12 }, lineStyle: { width: 1.2 }, data: [] },
                    { name: '最小延迟', type: 'line', animation: false, showSymbol: false, smooth: true, connectNulls: true,
                      itemStyle: { color: '#c4b5fd' }, areaStyle: { opacity: .12 }, lineStyle: { width: 1.2 }, data: [] },
                    { name: '平均延迟', type: 'line', animation: false, showSymbol: false, smooth: true, connectNulls: true,
                      itemStyle: { color: '#6366f1' }, lineStyle: { width: 2.2 },
                      areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                          { offset: 0, color: 'rgba(99,102,241,.30)' }, { offset: 1, color: 'rgba(99,102,241,.02)' }]) },
                      data: [] },
                    { name: '丢包率', type: 'line', yAxisIndex: 1, animation: false, showSymbol: false, connectNulls: true,
                      itemStyle: { color: '#f43f5e' }, lineStyle: { width: 1.8, type: 'dashed' }, data: [] },
                    { name: '抖动', type: 'line', animation: false, showSymbol: false, smooth: true, connectNulls: true,
                      itemStyle: { color: '#f59e0b' }, lineStyle: { width: 1.6 }, data: [] }
                ]
            });
        } else {
            bigChart.resize();
        }
        var now = new Date();
        var ago = new Date(now.getTime() - 2 * 3600 * 1000);
        function dlocal(d) {
            function p(n) { return n < 10 ? '0' + n : n; }
            return d.getFullYear() + '-' + p(d.getMonth() + 1) + '-' + p(d.getDate()) + 'T' + p(d.getHours()) + ':' + p(d.getMinutes());
        }
        $('#sp-chart-start').val(dlocal(ago));
        $('#sp-chart-end').val(dlocal(now));
        loadChart(apiurl);
    }

    /* ---------- password modal ---------- */
    function ensurePasswdModal() {
        if ($('#sp-passwd-modal').length) return;
        var html =
            '<div class="sp-modal-mask" id="sp-passwd-modal"><div class="sp-modal">' +
            '<div class="m-head"><h3>修改密码</h3><button class="m-close">&times;</button></div>' +
            '<div class="m-body">' +
            '<div class="field"><label>原密码</label><input type="password" class="input" id="sp-old-pw" autocomplete="current-password"></div>' +
            '<div class="field"><label>新密码</label><input type="password" class="input" id="sp-new-pw" autocomplete="new-password"><div class="hint">至少 6 个字符</div></div>' +
            '<div class="field"><label>确认新密码</label><input type="password" class="input" id="sp-new-pw2" autocomplete="new-password"></div>' +
            '</div>' +
            '<div class="m-foot"><button class="btn m-cancel">取消</button><button class="btn primary" id="sp-passwd-save">保存</button></div>' +
            '</div></div>';
        $('body').append(html);
        $('#sp-passwd-modal .m-cancel').click(function () { closeModal('sp-passwd-modal'); });
        $('#sp-passwd-save').click(function () {
            var oldpw = $('#sp-old-pw').val(), pw = $('#sp-new-pw').val(), pw2 = $('#sp-new-pw2').val();
            if (pw.length < 6) { toast('新密码至少6个字符', 'err'); return; }
            if (pw !== pw2) { toast('两次输入的新密码不一致', 'err'); return; }
            $.post('/api/user/passwd.json', { oldpassword: oldpw, password: pw }, function (res) {
                if (res.status === 'true') {
                    toast('密码修改成功', 'ok');
                    closeModal('sp-passwd-modal');
                    $('#sp-old-pw,#sp-new-pw,#sp-new-pw2').val('');
                } else {
                    toast(res.info || '修改失败', 'err');
                }
            }, 'json');
        });
    }

    /* ---------- shell ---------- */
    function renderShell() {
        var isAdmin = S.user && S.user.role === 'admin';
        var nav = '';
        $.each(navItems, function (_, it) {
            if (it.admin && !isAdmin) return;
            if (it.group) { nav += '<div class="nav-group">' + it.group + '</div>'; return; }
            nav += '<a href="' + it.href + '" class="' + (it.id === S.page ? 'active' : '') + '">' +
                icons[it.icon] + '<span>' + it.title + '</span></a>';
        });
        var roleTxt = isAdmin ? '管理员' : '只读用户';
        var initial = (S.user.username || '?').substring(0, 1).toUpperCase();
        var shell =
            '<div class="sp-layout">' +
            '<aside class="sp-sidebar" id="sp-sidebar">' +
            '<div class="sp-logo"><div class="logo-mark"><img src="assets/img/logo.png" alt="ZENLENET"></div><div class="logo-text">ZENLENET<small>PingMesh 网络质量监控</small></div></div>' +
            '<nav class="sp-nav">' + nav + '</nav>' +
            '<div class="sp-sidebar-foot">ZENLENET PingMesh <span id="sp-ver"></span></div>' +
            '</aside>' +
            '<div class="sp-main">' +
            '<header class="sp-topbar">' +
            '<button class="sp-menu-btn" id="sp-menu-btn">&#9776;</button>' +
            '<div class="page-title">' + esc(S.title) + '</div>' +
            '<div class="spacer"></div>' +
            '<span class="sync-time" id="sp-sync-time"></span>' +
            '<span class="node-chip" id="sp-node-chip" title="当前节点"></span>' +
            '<div class="sp-user" id="sp-user">' +
            '<div class="user-btn" id="sp-user-btn">' +
            '<div class="avatar">' + esc(initial) + '</div>' +
            '<div><div class="uname">' + esc(S.user.username) + '</div><div class="urole">' + roleTxt + '</div></div>' +
            '</div>' +
            '<div class="menu" id="sp-user-menu">' +
            '<a id="sp-menu-passwd">修改密码</a>' +
            '<a id="sp-menu-logout" class="danger">退出登录</a>' +
            '</div>' +
            '</div>' +
            '</header>' +
            '<main class="sp-content" id="sp-content"></main>' +
            '<div class="sp-footer"><span>&copy; 2026 ZENLENET PingMesh · Apache-2.0</span><span id="sp-foot-node"></span></div>' +
            '</div>' +
            '</div>';
        var page = $('#app').detach();
        $('body').prepend(shell);
        $('#sp-content').append(page.contents());
        page.remove();

        $('#sp-user-btn').click(function (e) { e.stopPropagation(); $('#sp-user-menu').toggleClass('open'); });
        $(document).click(function () { $('#sp-user-menu').removeClass('open'); });
        $('#sp-menu-btn').click(function (e) { e.stopPropagation(); $('#sp-sidebar').toggleClass('open'); });
        $('#sp-menu-logout').click(function () {
            $.getJSON('/api/logout.json').always(function () { window.location.href = '/login.html'; });
        });
        ensurePasswdModal();
        $('#sp-menu-passwd').click(function () { openModal('sp-passwd-modal'); });
    }

    function applyConfig(cfg) {
        S.config = cfg;
        $('#sp-ver').text('v' + cfg.Ver);
        $('#sp-node-chip').html('&#9679;&nbsp;' + esc(cfg.Name) + ' <span class="mono" style="font-weight:400">' + esc(cfg.Addr) + '</span>');
        $('#sp-foot-node').text('当前节点: ' + cfg.Name + ' (' + cfg.Addr + ')');
        if (cfg.Mode && cfg.Mode['Type'] === 'cloud') {
            $('#sp-sync-time').text('云端配置 · 最后同步 ' + (cfg.Mode['LastSuccTime'] || '-'));
        }
        // 全国延迟功能关闭时隐藏导航入口
        if (cfg.Base && cfg.Base['Chinamap'] === 0) {
            $('.sp-nav a[href="mapping.html"]').hide();
        }
    }

    /* ---------- init ---------- */
    function init(page, title) {
        S.page = page; S.title = title;
        var dfd = $.Deferred();
        $.ajax({ url: '/api/whoami.json', dataType: 'json' })
            .done(function (res) {
                S.user = res.user;
                renderShell();
                $.getJSON('/api/config.json', function (cfg) {
                    applyConfig(cfg);
                    dfd.resolve({ user: S.user, config: cfg });
                }).fail(function () {
                    toast('获取节点配置失败', 'err');
                    dfd.reject();
                });
            })
            .fail(function () { window.location.href = '/login.html'; });
        return dfd.promise();
    }

    /* ---------- ASN 查询(前端去重缓存, 服务端24h缓存, RIPE NCC 数据) ---------- */
    var asnCache = {};   // addr -> {asn, holder, prefix, ip} | null(查不到)
    var asnPending = {}; // addr -> [callbacks]

    function isPrivateHost(addr) {
        if (!addr) return true;
        if (/^(10\.|127\.|192\.168\.|169\.254\.)/.test(addr)) return true;
        var m = addr.match(/^172\.(\d+)\./);
        if (m && +m[1] >= 16 && +m[1] <= 31) return true;
        return false;
    }

    // asnGet(addr, cb): cb(info|null)
    function asnGet(addr, cb) {
        addr = (addr || '').replace(/^https?:\/\//, '').split('/')[0].split(':')[0];
        if (!addr || isPrivateHost(addr)) { cb(null); return; }
        if (asnCache.hasOwnProperty(addr)) { cb(asnCache[addr]); return; }
        if (asnPending[addr]) { asnPending[addr].push(cb); return; }
        asnPending[addr] = [cb];
        $.getJSON('/api/asn.json?ip=' + encodeURIComponent(addr))
            .done(function (res) {
                asnCache[addr] = (res.status === 'true') ? res : null;
            })
            .fail(function () { asnCache[addr] = null; })
            .always(function () {
                var cbs = asnPending[addr] || [];
                delete asnPending[addr];
                for (var i = 0; i < cbs.length; i++) cbs[i](asnCache[addr]);
            });
    }

    // 短文本: AS15169 · GOOGLE
    function asnShort(info) {
        if (!info || !info.asn) return '';
        var holder = String(info.holder || '');
        holder = holder.split(' - ')[0].split(',')[0];
        if (holder.length > 18) holder = holder.substring(0, 17) + '…';
        return 'AS' + info.asn + (holder ? ' · ' + holder : '');
    }

    // 同步读缓存(供 tooltip 等即时场景)
    function asnCached(addr) {
        addr = (addr || '').split(':')[0];
        return asnCache.hasOwnProperty(addr) ? asnCache[addr] : undefined;
    }

    /* 提取互Ping的源节点(探测节点且有监测目标) */
    function sourceNodes(cfg) {
        var list = [];
        $.each(cfg.Network, function (addr, n) {
            if (n.Pingmesh && ((n.Ping && n.Ping.length > 0) || (n.Topology && n.Topology.length > 0))) {
                list.push(n);
            }
        });
        list.sort(function (a, b) { return a.Addr === cfg.Addr ? -1 : (b.Addr === cfg.Addr ? 1 : (a.Name < b.Name ? -1 : 1)); });
        return list;
    }

    function nodeName(cfg, addr) {
        return (cfg.Network[addr] && cfg.Network[addr].Name) ? cfg.Network[addr].Name : addr;
    }

    /* ---------- MTR 渲染器(报警记录/检测工具共用) ---------- */
    function renderMtr(hops, targetIp) {
        hops = hops || [];
        if (hops.length === 0) {
            return '<div class="empty-state"><div class="big">&#128679;</div>无 MTR 数据</div>';
        }
        // 折叠尾部连续无响应跳
        var lastResp = -1;
        for (var i = 0; i < hops.length; i++) {
            if (hops[i].Host && hops[i].Host !== '???') lastResp = i;
        }
        var trailing = hops.length - 1 - lastResp;
        var shown = trailing > 1 ? hops.slice(0, lastResp + 1) : hops;
        var maxAvg = 0.001;
        $.each(shown, function (_, h) {
            var avg = h.Avg / 1e6;
            if (h.Host !== '???' && avg > maxAvg) maxAvg = avg;
        });
        var html = '<table class="mtr-table"><thead><tr>' +
            '<th style="width:46px">#</th><th>节点</th><th style="width:90px">丢包</th>' +
            '<th>平均延迟</th><th style="width:150px">最近 / 最优 / 最差</th><th style="width:80px">抖动</th>' +
            '</tr></thead><tbody>';
        $.each(shown, function (i, h) {
            var timeout = (h.Host === '???');
            var loss = h.Send > 0 ? (h.Loss / h.Send * 100) : 0;
            var avg = h.Avg / 1e6;
            var reached = !timeout && targetIp && h.Host === targetIp;
            var rowCls = timeout ? 'mtr-timeout' : (reached ? 'mtr-reach' : '');
            var hostCell;
            if (timeout) {
                hostCell = '<span class="muted">* * * 无响应(不回 TTL 超时报文)</span>';
            } else {
                hostCell = '<span class="mono">' + esc(h.Host) + '</span> <span class="mtr-asn" data-h="' + esc(h.Host) + '"></span>' +
                    (reached ? ' <span class="badge green" style="font-size:10px"><span class="dot"></span>到达目标</span>' : '');
            }
            var lossCell;
            if (timeout) lossCell = '<span class="muted">—</span>';
            else if (loss >= 50) lossCell = '<span class="badge red">' + loss.toFixed(0) + '%</span>';
            else if (loss >= 10) lossCell = '<span class="badge yellow">' + loss.toFixed(0) + '%</span>';
            else lossCell = '<span class="badge green">' + loss.toFixed(0) + '%</span>';
            var barCell = '<span class="muted">—</span>';
            if (!timeout) {
                var pct = Math.max(2, Math.round(avg / maxAvg * 100));
                var barCls = avg >= 150 ? ' hot' : (avg >= 50 ? ' warm' : '');
                barCell = '<div class="mtr-bar-wrap"><div class="mtr-bar-track"><div class="mtr-bar' + barCls + '" style="width:' + pct + '%"></div></div>' +
                    '<span class="mtr-bar-val">' + avg.toFixed(2) + ' ms</span></div>';
            }
            var lbw = timeout ? '<span class="muted">—</span>' :
                '<span class="mtr-sub">' + (h.Last / 1e6).toFixed(1) + ' / ' + (h.Best / 1e6).toFixed(1) + ' / ' + (h.Wrst / 1e6).toFixed(1) + '</span>';
            var sd = timeout ? '<span class="muted">—</span>' : '<span class="mtr-sub">' + h.StDev.toFixed(2) + ' ms</span>';
            html += '<tr class="' + rowCls + '"><td><span class="mtr-hopno">' + (i + 1) + '</span></td>' +
                '<td>' + hostCell + '</td><td>' + lossCell + '</td><td>' + barCell + '</td><td>' + lbw + '</td><td>' + sd + '</td></tr>';
        });
        if (trailing > 1) {
            html += '<tr class="mtr-timeout"><td><span class="mtr-hopno">&#8943;</span></td>' +
                '<td colspan="5"><span class="muted">后续 ' + trailing + ' 跳均无响应(目标或沿途设备不回 TTL 超时报文, 属常见现象)</span></td></tr>';
        }
        html += '</tbody></table>';
        return html;
    }

    // 渲染后异步补 ASN 标注
    function fillMtrAsn(container) {
        $(container).find('.mtr-asn').each(function () {
            var el = $(this);
            asnGet(el.attr('data-h'), function (info) {
                if (info) el.html('<span class="badge indigo" style="font-size:10px">' + esc(asnShort(info)) + '</span>');
            });
        });
    }

    return {
        state: S,
        renderMtr: renderMtr,
        fillMtrAsn: fillMtrAsn,
        init: init,
        esc: esc,
        toast: toast,
        openModal: openModal,
        closeModal: closeModal,
        confirm: confirmBox,
        proxy: proxy,
        fmtMs: fmtMs,
        delayLevel: delayLevel,
        openPingChart: openPingChart,
        sourceNodes: sourceNodes,
        nodeName: nodeName,
        asn: asnGet,
        asnShort: asnShort,
        asnCached: asnCached,
        isPrivateHost: isPrivateHost
    };
})();
