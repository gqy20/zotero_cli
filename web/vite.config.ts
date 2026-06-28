import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    // Output directly to where internal/server/embed.go expects it
    // (//go:embed all:web/dist, resolved relative to internal/server/).
    // This makes `go build -tags embed` pick up the frontend without a copy step.
    outDir: '../internal/server/web/dist',
    emptyOutDir: true,
  },
})
