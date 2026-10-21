import { writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import react from '@vitejs/plugin-react'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'

const fromRoot = (path: string) => fileURLToPath(new URL(path, import.meta.url))

// web/embed.go osadza dist, więc katalog nie może zostać bez pliku.
function keepDistPlaceholder(): Plugin {
  return {
    name: 'denar:keep-dist-placeholder',
    apply: 'build',
    closeBundle() {
      writeFileSync(fromRoot('./dist/.gitkeep'), '')
    },
  }
}

export default defineConfig({
  base: '/',
  plugins: [react(), keepDistPlaceholder()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    emptyOutDir: true,
    sourcemap: false,
    // Each font weight and subset is a file of its own; a data: URL would break font-src 'self'.
    assetsInlineLimit: 0,
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
    setupFiles: ['./vitest.setup.ts'],
    restoreMocks: true,
  },
})
