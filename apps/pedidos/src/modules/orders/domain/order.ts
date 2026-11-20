import { Currency, CurrencyMismatchError, Money } from '../../../shared/domain/money';
import { OrderItem, OrderItemProps } from './order-item';
import {
  DuplicateItemError,
  EmptyOrderError,
  InvalidCustomerError,
  InvalidOrderTransitionError,
} from './order.errors';
import type { OrderEvent } from './order.events';

/**
 *   PENDING ──markStockReserved()──▶ STOCK_RESERVED ──confirm()──▶ CONFIRMED
 *      │                                  │
 *      └────────cancel(reason)────────────┴──────▶ CANCELLED
 *
 * CONFIRMED and CANCELLED are final.
 */
export type OrderStatus = 'PENDING' | 'STOCK_RESERVED' | 'CONFIRMED' | 'CANCELLED';

export type CancellationReason = 'OUT_OF_STOCK' | 'PAYMENT_DECLINED' | 'CUSTOMER_REQUESTED';

export interface CreateOrderInput {
  customerId: string;
  items: OrderItemProps[];
  /** Optional: lets tests (and idempotent retries) choose the id. */
  id?: string;
  /** Optional: lets tests control the clock. */
  now?: Date;
}

/** Plain data used to save and load an order. No classes, no behavior. */
export interface OrderSnapshot {
  id: string;
  customerId: string;
  status: OrderStatus;
  cancellationReason: CancellationReason | null;
  createdAt: Date;
  items: {
    sku: string;
    productName: string;
    unitPriceCents: number;
    currency: Currency;
    quantity: number;
  }[];
}

/**
 * Aggregate root. Every change to an order (and its items) goes through here,
 * so the rules can never be skipped.
 */
export class Order {
  private events: OrderEvent[] = [];

  private constructor(
    readonly id: string,
    readonly customerId: string,
    private readonly lines: readonly OrderItem[],
    private currentStatus: OrderStatus,
    private reason: CancellationReason | null,
    readonly createdAt: Date,
  ) {}

  static create(input: CreateOrderInput): Order {
    if (!input.customerId.trim()) {
      throw new InvalidCustomerError();
    }
    if (input.items.length === 0) {
      throw new EmptyOrderError();
    }

    const items = input.items.map((props) => OrderItem.create(props));
    Order.assertNoDuplicateSkus(items);
    Order.assertSingleCurrency(items);

    const order = new Order(
      input.id ?? crypto.randomUUID(),
      input.customerId,
      Object.freeze(items),
      'PENDING',
      null,
      input.now ?? new Date(),
    );

    order.record({
      type: 'OrderCreated',
      orderId: order.id,
      customerId: order.customerId,
      items: items.map((item) => ({ sku: item.sku, quantity: item.quantity })),
      totalCents: order.total.cents,
      currency: order.total.currency,
      occurredAt: new Date(),
    });
    return order;
  }

  /** Rebuilds an order loaded from the database. Skips creation rules and emits no events. */
  static restore(snapshot: OrderSnapshot): Order {
    const items = snapshot.items.map((item) =>
      OrderItem.create({
        sku: item.sku,
        productName: item.productName,
        unitPrice: Money.of(item.unitPriceCents, item.currency),
        quantity: item.quantity,
      }),
    );
    return new Order(
      snapshot.id,
      snapshot.customerId,
      Object.freeze(items),
      snapshot.status,
      snapshot.cancellationReason,
      snapshot.createdAt,
    );
  }

  get status(): OrderStatus {
    return this.currentStatus;
  }

  get cancellationReason(): CancellationReason | null {
    return this.reason;
  }

  get items(): readonly OrderItem[] {
    return this.lines;
  }

  get total(): Money {
    return this.lines.reduce((sum, item) => sum.add(item.subtotal), Money.zero(this.lines[0].unitPrice.currency));
  }

  /** Stock service reserved every item. */
  markStockReserved(): void {
    if (this.currentStatus === 'STOCK_RESERVED') return; // repeated message: no-op
    this.assertStatus('PENDING', 'mark stock reserved on');
    this.currentStatus = 'STOCK_RESERVED';
  }

  /** Payment was approved. */
  confirm(): void {
    if (this.currentStatus === 'CONFIRMED') return; // repeated message: no-op
    this.assertStatus('STOCK_RESERVED', 'confirm');
    this.currentStatus = 'CONFIRMED';
    this.record({
      type: 'OrderConfirmed',
      orderId: this.id,
      customerId: this.customerId,
      totalCents: this.total.cents,
      currency: this.total.currency,
      occurredAt: new Date(),
    });
  }

  cancel(reason: CancellationReason): void {
    if (this.currentStatus === 'CANCELLED') return; // repeated message: no-op
    if (this.currentStatus === 'CONFIRMED') {
      throw new InvalidOrderTransitionError(this.currentStatus, 'cancel');
    }
    const stockWasReserved = this.currentStatus === 'STOCK_RESERVED';
    this.currentStatus = 'CANCELLED';
    this.reason = reason;
    this.record({
      type: 'OrderCancelled',
      orderId: this.id,
      customerId: this.customerId,
      reason,
      stockWasReserved,
      occurredAt: new Date(),
    });
  }

  /** Returns the events recorded since the last call and clears them. */
  pullEvents(): OrderEvent[] {
    const events = this.events;
    this.events = [];
    return events;
  }

  toSnapshot(): OrderSnapshot {
    return {
      id: this.id,
      customerId: this.customerId,
      status: this.currentStatus,
      cancellationReason: this.reason,
      createdAt: this.createdAt,
      items: this.lines.map((item) => ({
        sku: item.sku,
        productName: item.productName,
        unitPriceCents: item.unitPrice.cents,
        currency: item.unitPrice.currency,
        quantity: item.quantity,
      })),
    };
  }

  private record(event: OrderEvent): void {
    this.events.push(event);
  }

  private assertStatus(expected: OrderStatus, action: string): void {
    if (this.currentStatus !== expected) {
      throw new InvalidOrderTransitionError(this.currentStatus, action);
    }
  }

  private static assertNoDuplicateSkus(items: OrderItem[]): void {
    const seen = new Set<string>();
    for (const item of items) {
      if (seen.has(item.sku)) throw new DuplicateItemError(item.sku);
      seen.add(item.sku);
    }
  }

  private static assertSingleCurrency(items: OrderItem[]): void {
    const currency = items[0].unitPrice.currency;
    const other = items.find((item) => item.unitPrice.currency !== currency);
    if (other) throw new CurrencyMismatchError(currency, other.unitPrice.currency);
  }
}
