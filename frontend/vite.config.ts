import react from '@vitejs/plugin-react';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '..', '');
  const apiTarget = env.VITE_API_PROXY_TARGET || `http://127.0.0.1:${env.MGP_PORT || '18080'}`;

  return {
    plugins: [react()],
    build: {
      chunkSizeWarningLimit: 650,
    },
    server: {
      port: 5173,
      proxy: {
        '/api/v1': {
          target: apiTarget,
          changeOrigin: true,
        },
      },
    },
  };
});
