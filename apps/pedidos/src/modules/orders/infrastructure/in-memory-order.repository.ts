import { Order, OrderSnapshot } from '../domain/order';
import type { OrderEvent } from '../domain/order.events';
import type { OrderRepository } from '../domain/order.repository';

/**
 * OrderRepository kept in memory. Used by tests (and as a reference for the
 * Drizzle version). It behaves like a real database on purpose:
 * - it stores copies (snapshots), never the object itself, so a missing
 *   `save()` shows up in tests;
 * - `save()` moves the aggregate's events into `outbox`, the way the future
 *   transactional outbox will (Phase 2).
 */
export class InMemoryOrderRepository implements OrderRepository {
  private readonly rows = new Map<string, OrderSnapshot>();
  readonly outbox: OrderEvent[] = [];

  async findById(id: string): Promise<Order | null> {
    const snapshot = this.rows.get(id);
    return snapshot ? Order.restore(structuredClone(snapshot)) : null;
  }

  async save(order: Order): Promise<void> {
    this.rows.set(order.id, structuredClone(order.toSnapshot()));
    this.outbox.push(...order.pullEvents());
  }

  /** Test helper: number of saved orders. */
  get size(): number {
    return this.rows.size;
  }

  /** Test helper: every saved order. */
  all(): Order[] {
    return [...this.rows.values()].map((snapshot) => Order.restore(structuredClone(snapshot)));
  }
}
