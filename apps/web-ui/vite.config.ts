import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import pkg from './package.json';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  // Expose the app version (single source: package.json) as a compile-time
  // constant so the UI can display it.
  define: {
    __TABVM_VERSION__: JSON.stringify(pkg.version),
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:5230',
        changeOrigin: true,
        // ws enables proxying the screen-stream WebSocket to the agent in dev.
        ws: true,
      },
      '/health': {
        target: 'http://127.0.0.1:5230',
        changeOrigin: true,
      },
    },
  },
  test: {
    allowOnly: false,
    environment: 'jsdom',
  },
});
