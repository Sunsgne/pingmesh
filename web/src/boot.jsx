import { StrictMode, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Box, CssBaseline, ThemeProvider } from '@mui/material';
import { theme } from './theme';
import { getJSON } from './api';
import Shell from './components/Shell';
import { FeedbackProvider, useToast } from './components/Feedback';
import { Spinner } from './components/ui';

// 页面长时间开着时自动跟上新版本: 回到前台或每 5 分钟比对一次当前页面引用的入口脚本,
// 变了就刷新(页面在后台时直接刷新; 前台时等用户下次切回来, 避免打断正在进行的操作)
function useReleaseWatch() {
  useEffect(() => {
    const mine = Array.from(document.scripts).map((s) => s.getAttribute('src') || '').find((s) => s.startsWith('/assets/app/'));
    if (!mine) return undefined;
    let stale = false;
    const check = () => fetch(window.location.pathname, { cache: 'no-store', credentials: 'same-origin' })
      .then((r) => (r.ok && !r.redirected ? r.text() : ''))
      .then((html) => { if (html && html.indexOf('src="' + mine + '"') < 0 && html.indexOf('/assets/app/') >= 0) stale = true; })
      .catch(() => {});
    const onVis = () => {
      if (document.visibilityState === 'hidden') { check(); return; }
      if (stale) { window.location.reload(); return; }
      check().then(() => { if (stale) window.location.reload(); });
    };
    const timer = setInterval(check, 5 * 60 * 1000);
    document.addEventListener('visibilitychange', onVis);
    return () => { clearInterval(timer); document.removeEventListener('visibilitychange', onVis); };
  }, []);
}

function Page({ id, title, Component }) {
  useReleaseWatch();
  const toast = useToast();
  const [ctx, setCtx] = useState(null);

  useEffect(() => {
    let user;
    getJSON('/api/whoami.json')
      .then((res) => {
        if (!res || !res.user) throw new Error('no user');
        user = res.user;
        return getJSON('/api/config.json').then(
          (config) => setCtx({ user, config }),
          () => { toast('获取节点配置失败', 'err'); },
        );
      })
      .catch(() => { window.location.href = '/login.html'; });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  if (!ctx) {
    return <Box sx={{ height: '100vh', display: 'grid', placeItems: 'center' }}><Spinner size={28} /></Box>;
  }
  return (
    <Shell page={id} title={title} user={ctx.user} config={ctx.config}>
      <Component user={ctx.user} config={ctx.config} isAdmin={ctx.user.role === 'admin'} />
    </Shell>
  );
}

export function mount(node) {
  createRoot(document.getElementById('root')).render(
    <StrictMode>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <FeedbackProvider>{node}</FeedbackProvider>
      </ThemeProvider>
    </StrictMode>,
  );
}

/** 每个页面入口: bootPage('index', '概览', Overview) */
export function bootPage(id, title, Component) {
  mount(<Page id={id} title={title} Component={Component} />);
}
