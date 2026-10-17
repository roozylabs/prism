import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';

// Component and unit tests for the dashboard. `pnpm test` runs them once;
// `pnpm test:watch` keeps them running while editing. TSX is compiled by
// esbuild with the automatic JSX runtime, so no React plugin is needed.
export default defineConfig({
  esbuild: {
    jsx: 'automatic',
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('.', import.meta.url)),
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./vitest.setup.ts'],
    include: ['**/*.test.{ts,tsx}'],
    exclude: ['node_modules/**', '.next/**'],
    css: false,
    clearMocks: true,
  },
});
