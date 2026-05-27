import { defineConfig } from 'astro/config';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  vite: {
    plugins: [tailwindcss()],
  },
  server: {
    port: 4321,
    host: 'localhost',
  },
  outDir: 'cmd/livepaper/dist',
  build: {
    assets: 'assets',
  },
});
