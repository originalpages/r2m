import { defineConfig } from 'vite';

// Set BASE_PATH (e.g. "/r2m/") when deploying to GitHub Pages project sites.
export default defineConfig({
  base: process.env.BASE_PATH ?? './',
  build: { outDir: 'dist', chunkSizeWarningLimit: 4000 },
  server: { fs: { allow: ['..'] } },
});
