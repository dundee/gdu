/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// The bundle is emitted into ../dist, which is committed and embedded into the
// Gdu binary via go:embed. `base: './'` keeps asset URLs relative so the SPA
// works regardless of the host/port the Go server binds to.
export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: '../dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1500,
  },
  server: {
    // For local development run: gdu --web --web-listen localhost:8080
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
  },
});
