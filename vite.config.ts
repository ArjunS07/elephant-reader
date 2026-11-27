import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vite'

// Preview has no HTML history fallback of its own once a request misses a file.
// Rewrite extension-less GET routes to index.html so /feed and /item/:id refresh.
function spaFallback(): Plugin {
  return {
    name: 'spa-fallback',
    configurePreviewServer(server) {
      server.middlewares.use((req, _res, next) => {
        const path = req.url?.split('?')[0] ?? ''
        if (
          req.method === 'GET' &&
          path !== '/' &&
          !path.startsWith('/assets/') &&
          !path.includes('.')
        ) {
          req.url = '/index.html'
        }
        next()
      })
    },
  }
}

export default defineConfig({
  base: '/',
  appType: 'spa',
  plugins: [react(), spaFallback()],
  server: {
    host: '0.0.0.0',
    port: 4731,
    strictPort: true,
  },
  preview: {
    host: '0.0.0.0',
    port: 4731,
    strictPort: true,
  },
})
