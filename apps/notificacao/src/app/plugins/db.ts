import type { FastifyInstance } from 'fastify';
import fp from 'fastify-plugin';
import { drizzle, type NodePgDatabase } from 'drizzle-orm/node-postgres';
import pg from 'pg';
import * as schema from '../../db/schema.ts';

export type Database = NodePgDatabase<typeof schema>;

declare module 'fastify' {
  interface FastifyInstance {
    db: Database;
  }
}

// Só para desenvolvimento local (Postgres do docker compose na porta 5444).
// Em container, o compose passa DATABASE_URL.
const LOCAL_DATABASE_URL = 'postgres://notificacao:notificacao@localhost:5444/notificacao';

/**
 * Cria o pool do Postgres e expõe o Drizzle em `fastify.db`.
 * O pool é preguiçoso: só conecta na primeira query, então o serviço sobe
 * mesmo com o banco fora do ar.
 */
export default fp(async function (fastify: FastifyInstance) {
  const pool = new pg.Pool({
    connectionString: process.env.DATABASE_URL ?? LOCAL_DATABASE_URL,
    connectionTimeoutMillis: 2_000,
  });
  // Sem este listener, um erro numa conexão ociosa (ex: o Postgres reiniciou)
  // derruba o processo inteiro.
  pool.on('error', (err) => fastify.log.error({ err }, 'Postgres pool error'));

  fastify.decorate('db', drizzle(pool, { schema }));
  fastify.addHook('onClose', async () => {
    await pool.end();
  });
});
