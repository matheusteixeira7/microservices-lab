import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    name: 'notificacao-e2e',
    globals: true,
    environment: 'node',
    include: ['src/**/*.spec.ts'],
    globalSetup: ['src/support/global-setup.ts'],
    setupFiles: ['src/support/test-setup.ts'],
  },
});
