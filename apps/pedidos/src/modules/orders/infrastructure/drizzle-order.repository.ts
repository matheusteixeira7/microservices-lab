import { asc, eq } from 'drizzle-orm';
import type { Database } from '../../../shared/infrastructure/database/database';
import { outboxEvents } from '../../../shared/infrastructure/outbox/outbox.table';
import { Order } from '../domain/order';
import type { OrderRepository } from '../domain/order.repository';
import { orderItems, orders } from './orders.table';

/** OrderRepository backed by Postgres through Drizzle. */
export class DrizzleOrderRepository implements OrderRepository {
  constructor(private readonly db: Database) {}

  async findById(id: string): Promise<Order | null> {
    const [row] = await this.db.select().from(orders).where(eq(orders.id, id));
    if (!row) return null;

    const items = await this.db
      .select()
      .from(orderItems)
      .where(eq(orderItems.orderId, id))
      .orderBy(asc(orderItems.position));

    return Order.restore({
      id: row.id,
      customerId: row.customerId,
      status: row.status,
      cancellationReason: row.cancellationReason,
      createdAt: row.createdAt,
      items: items.map((item) => ({
        sku: item.sku,
        productName: item.productName,
        unitPriceCents: item.unitPriceCents,
        currency: item.currency,
        quantity: item.quantity,
      })),
    });
  }

  /**
   * Order, items and events go in ONE transaction: either all are saved or
   * none is. That is what makes the outbox reliable.
   */
  async save(order: Order): Promise<void> {
    const snapshot = order.toSnapshot();
    // If the transaction fails, these events are gone from the aggregate too.
    // That is fine: the error propagates and the caller drops this instance.
    const events = order.pullEvents();

    await this.db.transaction(async (tx) => {
      await tx
        .insert(orders)
        .values({
          id: snapshot.id,
          customerId: snapshot.customerId,
          status: snapshot.status,
          cancellationReason: snapshot.cancellationReason,
          createdAt: snapshot.createdAt,
        })
        .onConflictDoUpdate({
          target: orders.id,
          set: {
            status: snapshot.status,
            cancellationReason: snapshot.cancellationReason,
            updatedAt: new Date(),
          },
        });

      // Items never change after the order is created.
      await tx
        .insert(orderItems)
        .values(snapshot.items.map((item, position) => ({ ...item, orderId: snapshot.id, position })))
        .onConflictDoNothing();

      if (events.length > 0) {
        await tx.insert(outboxEvents).values(
          events.map((event) => ({
            id: crypto.randomUUID(),
            aggregateType: 'Order',
            aggregateId: event.orderId,
            eventType: event.type,
            payload: event,
            occurredAt: event.occurredAt,
          })),
        );
      }
    });
  }
}
