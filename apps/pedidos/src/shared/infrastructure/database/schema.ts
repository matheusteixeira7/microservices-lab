// Every Drizzle table of the pedidos service. Each module defines its own
// tables in modules/<module>/infrastructure/ and re-exports them here.
// drizzle-kit reads this file to generate migrations.
export * from '../../../modules/orders/infrastructure/orders.table';
export * from '../outbox/outbox.table';
