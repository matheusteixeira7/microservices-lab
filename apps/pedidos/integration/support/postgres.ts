import { PostgreSqlContainer, type StartedPostgreSqlContainer } from '@testcontainers/postgresql';
import { drizzle } from 'drizzle-orm/node-postgres';
import { migrate } from 'drizzle-orm/node-postgres/migrator';
import { join } from 'node:path';
import { Pool } from 'pg';
import * as schema from '../../src/shared/infrastructure/database/schema';

/**
 * Starts a throwaway Postgres (same image as compose) and applies the real
 * migrations from apps/pedidos/drizzle. Needs Docker running.
 */
export async function startPostgres() {
  const container: StartedPostgreSqlContainer = await new PostgreSqlContainer('postgres:18-alpine').start();
  const pool = new Pool({ connectionString: container.getConnectionUri() });
  const db = drizzle(pool, { schema });

  await migrate(db, { migrationsFolder: join(__dirname, '../../drizzle') });

  return {
    db,
    pool,
    async stop() {
      await pool.end();
      await container.stop();
    },
  };
}
