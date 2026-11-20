import Fastify from 'fastify';
import health from './health.ts';

function createServer(execute: () => Promise<unknown>) {
  const server = Fastify();
  server.decorate('db', { execute } as never);
  server.register(health);
  return server;
}

describe('GET /health', () => {
  it('returns ok when the database answers', async () => {
    const server = createServer(async () => ({ rows: [] }));
    const response = await server.inject({ method: 'GET', url: '/health' });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({ status: 'ok', db: 'up' });
  });

  it('returns 503 when the database is down', async () => {
    const server = createServer(async () => {
      throw new Error('connect ECONNREFUSED');
    });
    const response = await server.inject({ method: 'GET', url: '/health' });

    expect(response.statusCode).toBe(503);
    expect(response.json()).toEqual({ status: 'error', db: 'down' });
  });
});
