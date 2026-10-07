import { StrictMode, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Box, CircularProgress, CssBaseline, ThemeProvider } from '@mui/material';
import { theme } from './theme';
import { getJSON } from './api';
import Shell from './Shell';
import Overview from './Overview';

function App() {
  const [ctx, setCtx] = useState(null);

  useEffect(() => {
    Promise.all([getJSON('/api/whoami.json'), getJSON('/api/config.json')])
      .then(([who, cfg]) => setCtx({ user: who.user, config: cfg }))
      .catch(() => { window.location.href = '/login.html'; });
  }, []);

  if (!ctx) {
    return <Box sx={{ height: '100vh', display: 'grid', placeItems: 'center' }}><CircularProgress /></Box>;
  }
  return (
    <Shell page="index" title="概览" user={ctx.user} config={ctx.config}>
      <Overview rootCfg={ctx.config} />
    </Shell>
  );
}

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <App />
    </ThemeProvider>
  </StrictMode>,
);
