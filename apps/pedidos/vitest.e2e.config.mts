import { defineConfig } from 'vitest/config';

export default defineConfig({
  root: import.meta.dirname,
  cacheDir: '../../node_modules/.vite/apps/pedidos-e2e',
  test: {
    name: 'pedidos-e2e',
    watch: false,
    globals: true,
    environment: 'node',
    include: ['e2e/**/*.spec.ts'],
    globalSetup: ['e2e/support/global-setup.ts'],
    setupFiles: ['e2e/support/test-setup.ts'],
  },
});
