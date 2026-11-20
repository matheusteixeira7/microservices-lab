import { defineConfig } from 'vitest/config';

export default defineConfig({
  root: import.meta.dirname,
  cacheDir: '../../node_modules/.vite/apps/pedidos-integration',
  test: {
    name: 'pedidos-integration',
    watch: false,
    globals: true,
    environment: 'node',
    include: ['integration/**/*.spec.ts'],
    // Starting a Postgres container takes a few seconds.
    hookTimeout: 120_000,
    testTimeout: 30_000,
  },
});
