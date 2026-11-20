import type { FastifyInstance } from 'fastify';
import { sql } from 'drizzle-orm';

export default async function (fastify: FastifyInstance) {
  fastify.get('/health', async function (_request, reply) {
    try {
      await fastify.db.execute(sql`select 1`);
      return { status: 'ok', db: 'up' };
    } catch {
      return reply.code(503).send({ status: 'error', db: 'down' });
    }
  });
}
