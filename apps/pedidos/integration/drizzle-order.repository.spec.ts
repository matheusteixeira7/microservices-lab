import { sql } from 'drizzle-orm';
import { Money } from '../src/shared/domain/money';
import { outboxEvents } from '../src/shared/infrastructure/outbox/outbox.table';
import { Order } from '../src/modules/orders/domain/order';
import { DrizzleOrderRepository } from '../src/modules/orders/infrastructure/drizzle-order.repository';
import { orders } from '../src/modules/orders/infrastructure/orders.table';
import { orderRepositoryContract } from '../src/modules/orders/infrastructure/order.repository.contract';
import { startPostgres } from './support/postgres';

describe('DrizzleOrderRepository (real Postgres)', () => {
  let postgres: Awaited<ReturnType<typeof startPostgres>>;

  beforeAll(async () => {
    postgres = await startPostgres();
  });

  afterAll(async () => {
    await postgres?.stop();
  });

  async function reset() {
    await postgres.db.execute(sql`truncate orders, order_items, outbox_events restart identity cascade`);
  }

  describe('contract', () => {
    orderRepositoryContract(async () => {
      await reset();
      const repository = new DrizzleOrderRepository(postgres.db);
      return {
        repository,
        storedEvents: async () =>
          postgres.db
            .select({ eventType: outboxEvents.eventType, aggregateId: outboxEvents.aggregateId })
            .from(outboxEvents)
            .orderBy(outboxEvents.sequence),
      };
    });
  });

  it('saves order and events atomically: if the outbox write fails, nothing is saved', async () => {
    await reset();
    const repository = new DrizzleOrderRepository(postgres.db);
    const order = Order.create({
      customerId: 'customer-1',
      items: [{ sku: 'KB-1', productName: 'Keyboard', unitPrice: Money.of(15000), quantity: 1 }],
    });

    // Make every outbox insert fail.
    await postgres.db.execute(sql`alter table outbox_events add constraint always_fail check (false)`);
    try {
      // Drizzle wraps the Postgres error; the original one is in `cause`.
      await expect(repository.save(order)).rejects.toMatchObject({
        cause: expect.objectContaining({ constraint: 'always_fail' }),
      });
    } finally {
      await postgres.db.execute(sql`alter table outbox_events drop constraint always_fail`);
    }

    const rows = await postgres.db.select().from(orders);
    expect(rows).toEqual([]);
  });
});
