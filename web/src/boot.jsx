import { StrictMode, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Box, CssBaseline, ThemeProvider } from '@mui/material';
import { theme } from './theme';
import { getJSON } from './api';
import Shell from './components/Shell';
import { FeedbackProvider, useToast } from './components/Feedback';
import { Spinner } from './components/ui';

function Page({ id, title, Component }) {
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
