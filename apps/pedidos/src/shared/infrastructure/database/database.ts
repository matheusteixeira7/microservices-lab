import type { NodePgDatabase } from 'drizzle-orm/node-postgres';
import type * as schema from './schema';

/** Drizzle client typed with every table of this service. Framework-free on purpose. */
export type Database = NodePgDatabase<typeof schema>;

// Local development only (the compose Postgres on port 5441).
// In containers, compose sets DATABASE_URL.
const LOCAL_DATABASE_URL = 'postgres://pedidos:pedidos@localhost:5441/pedidos';

export function databaseUrl(): string {
  return process.env.DATABASE_URL ?? LOCAL_DATABASE_URL;
}
