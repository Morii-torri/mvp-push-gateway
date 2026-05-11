import react from '@vitejs/plugin-react';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '..', '');
  const apiTarget = env.VITE_API_PROXY_TARGET || `http://127.0.0.1:${env.MGP_PORT || '18080'}`;

  return {
    plugins: [react()],
    build: {
      chunkSizeWarningLimit: 1100,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.includes('/node_modules/react/') || id.includes('/node_modules/react-dom/')) {
              return 'react-vendor';
            }
            if (id.includes('/node_modules/@xyflow/') || id.includes('/node_modules/d3-')) {
              return 'react-flow';
            }
            if (
              id.includes('/node_modules/antd/') ||
              id.includes('/node_modules/@ant-design/') ||
              id.includes('/node_modules/@rc-component/') ||
              id.includes('/node_modules/rc-') ||
              id.includes('/node_modules/dayjs/') ||
              id.includes('/node_modules/async-validator/')
            ) {
              return 'antd-vendor';
            }
            if (id.includes('/src/pages/ConsolePages')) {
              return 'console-pages';
            }
            if (id.includes('/src/api/') || id.includes('/src/utils/')) {
              return 'console-data';
            }
          },
        },
      },
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
