import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/auth': 'http://localhost:8080',
      '/rooms': {
        target: 'http://localhost:8080',
        ws: true,
      },
      '/server': {
        target: 'http://localhost:8080',
        ws: true,
      },
      '/files': 'http://localhost:8080',
      '/users': 'http://localhost:8080',
      '/crypto': 'http://localhost:8080',
      '/admin': 'http://localhost:8080',
      '/health': 'http://localhost:8080',
      '/ready': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
