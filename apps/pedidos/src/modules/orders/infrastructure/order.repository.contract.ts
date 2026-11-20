import { beforeEach, expect, it } from 'vitest';
import { Money } from '../../../shared/domain/money';
import { Order } from '../domain/order';
import type { OrderRepository } from '../domain/order.repository';

/*
 * Contract test: the behavior EVERY OrderRepository must have.
 * It runs against the in-memory version (unit tests) and the Drizzle version
 * (integration tests), so the fake can never drift away from the real one.
 */

export interface OrderRepositoryContext {
  repository: OrderRepository;
  /** Events stored by `save`, in order. */
  storedEvents(): Promise<{ eventType: string; aggregateId: string }[]>;
}

function newOrder() {
  return Order.create({
    customerId: 'customer-1',
    // Not in alphabetical order on purpose: the repository must keep this order.
    items: [
      { sku: 'ZZ-1', productName: 'Monitor', unitPrice: Money.of(120000), quantity: 1 },
      { sku: 'AA-1', productName: 'Cable', unitPrice: Money.of(2500), quantity: 3 },
    ],
    now: new Date('2026-11-20T12:00:00.123Z'),
  });
}

export function orderRepositoryContract(setup: () => Promise<OrderRepositoryContext>) {
  let ctx: OrderRepositoryContext;

  beforeEach(async () => {
    ctx = await setup();
  });

  it('returns null for an unknown id', async () => {
    expect(await ctx.repository.findById(crypto.randomUUID())).toBeNull();
  });

  it('loads exactly what was saved (item order and createdAt included)', async () => {
    const order = newOrder();
    await ctx.repository.save(order);

    const loaded = await ctx.repository.findById(order.id);

    expect(loaded?.toSnapshot()).toEqual(order.toSnapshot());
  });

  it('a second save updates status and cancellation reason', async () => {
    const order = newOrder();
    await ctx.repository.save(order);
    order.cancel('OUT_OF_STOCK');
    await ctx.repository.save(order);

    const loaded = await ctx.repository.findById(order.id);

    expect(loaded?.status).toBe('CANCELLED');
    expect(loaded?.cancellationReason).toBe('OUT_OF_STOCK');
  });

  it('stores each event exactly once, in order', async () => {
    const order = newOrder();
    await ctx.repository.save(order);
    order.markStockReserved();
    order.confirm();
    await ctx.repository.save(order);
    await ctx.repository.save(order); // nothing new to store

    expect(await ctx.storedEvents()).toEqual([
      { eventType: 'OrderCreated', aggregateId: order.id },
      { eventType: 'OrderConfirmed', aggregateId: order.id },
    ]);
  });
}
