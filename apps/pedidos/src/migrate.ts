/**
 * Applies the Drizzle migrations and exits. Runs as a one-shot container
 * (`pedidos-migrate` in compose) BEFORE the service starts, so the service
 * itself never depends on the database to boot.
 *
 * Local: node apps/pedidos/dist/migrate.js   (or: pnpm nx db:migrate pedidos)
 */
import { Logger } from '@nestjs/common';
import { drizzle } from 'drizzle-orm/node-postgres';
import { migrate } from 'drizzle-orm/node-postgres/migrator';
import { join } from 'node:path';
import { Pool } from 'pg';
import { databaseUrl } from './shared/infrastructure/database/database';

async function main() {
  const pool = new Pool({ connectionString: databaseUrl() });
  try {
    // The build copies drizzle/ next to this file (see webpack.config.js).
    await migrate(drizzle(pool), { migrationsFolder: join(__dirname, 'drizzle') });
    Logger.log('Migrations applied', 'Migrate');
  } finally {
    await pool.end();
  }
}

main().catch((error: unknown) => {
  Logger.error(`Migration failed: ${error instanceof Error ? error.message : String(error)}`, 'Migrate');
  process.exit(1);
});
