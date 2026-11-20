import { waitForPortOpen } from '@nx/node/utils';

const host = process.env.HOST ?? 'localhost';
const port = process.env.PORT ? Number(process.env.PORT) : 3004;

export async function setup() {
  // Start what the service needs before the tests (database, docker-compose, etc.).
  await waitForPortOpen(port, { host });
}

export async function teardown() {
  // Clean up here. Nx stops the `serve` task by itself.
}
