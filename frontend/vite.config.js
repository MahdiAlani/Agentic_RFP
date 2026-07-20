import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Dev proxy: browser calls /api/* -> Go backend on :8080 (avoids CORS in dev).
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
    },
  },
})
