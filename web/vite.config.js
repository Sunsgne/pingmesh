import { readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

const pagesDir = resolve(__dirname, 'pages');
const input = Object.fromEntries(
  readdirSync(pagesDir).filter((f) => f.endsWith('.html')).map((f) => [f.replace(/\.html$/, ''), resolve(pagesDir, f)]),
);

// 多页构建: 每个旧 URL(/index.html、/pingmesh.html ...) 对应一个入口, 产物直接写入 Go 内嵌的 html/
export default defineConfig({
  root: pagesDir,
  base: '/',
  publicDir: false,
  plugins: [react()],
  build: {
    outDir: resolve(__dirname, '../html'),
    emptyOutDir: false,
    assetsDir: 'assets/app',
    chunkSizeWarningLimit: 900,
    rollupOptions: {
      input,
      output: {
        manualChunks: (id) => (id.includes('node_modules') ? 'vendor' : undefined),
      },
    },
  },
  server: { proxy: { '/api': 'http://127.0.0.1:8899' } },
});
