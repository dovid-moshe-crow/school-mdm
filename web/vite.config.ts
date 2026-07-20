import type { IncomingMessage } from 'node:http'
import type { Socket } from 'node:net'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/',
  build: { outDir: '../internal/webui/dist', emptyOutDir: true },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        // Air rebuilds briefly drop the listener; return JSON instead of crashing the log.
        configure(proxy) {
          proxy.on('error', (err: Error, _req: IncomingMessage, res: Socket | import('http').ServerResponse) => {
            if (res && 'writeHead' in res && typeof res.writeHead === 'function' && !res.headersSent) {
              res.writeHead(502, { 'Content-Type': 'application/json' })
              res.end(JSON.stringify({ error: 'api starting or restarting', detail: err.message }))
            }
          })
        },
      },
      '/healthz': 'http://127.0.0.1:8080',
    },
  },
})
