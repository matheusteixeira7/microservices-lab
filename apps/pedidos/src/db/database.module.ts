import { Global, Inject, Logger, Module, OnApplicationShutdown } from '@nestjs/common';
import { drizzle, NodePgDatabase } from 'drizzle-orm/node-postgres';
import { Pool } from 'pg';
import * as schema from './schema';

export const DB = Symbol('DB');
const PG_POOL = Symbol('PG_POOL');

export type Database = NodePgDatabase<typeof schema>;

// Só para desenvolvimento local (Postgres do docker compose na porta 5441).
// Em container, o compose passa DATABASE_URL.
const LOCAL_DATABASE_URL = 'postgres://pedidos:pedidos@localhost:5441/pedidos';

@Global()
@Module({
  providers: [
    {
      provide: PG_POOL,
      useFactory: () => {
        // O Pool é preguiçoso: só conecta na primeira query.
        // Assim o serviço sobe mesmo com o banco fora do ar.
        const pool = new Pool({
          connectionString: process.env.DATABASE_URL ?? LOCAL_DATABASE_URL,
          connectionTimeoutMillis: 2_000,
        });
        // Sem este listener, um erro numa conexão ociosa (ex: o Postgres
        // reiniciou) derruba o processo inteiro.
        pool.on('error', (err) => Logger.error(`Postgres pool error: ${err.message}`, 'Database'));
        return pool;
      },
    },
    {
      provide: DB,
      inject: [PG_POOL],
      useFactory: (pool: Pool): Database => drizzle(pool, { schema }),
    },
  ],
  exports: [DB],
})
export class DatabaseModule implements OnApplicationShutdown {
  constructor(@Inject(PG_POOL) private readonly pool: Pool) {}

  async onApplicationShutdown() {
    await this.pool.end();
  }
}
