import type { Currency } from '../../../shared/domain/money';
import type { CancellationReason } from './order';

/*
 * Domain events: facts that already happened. Plain data only, because they
 * will become Kafka messages. Each one carries what consumers need, so they
 * never have to call this service back.
 */

export interface OrderCreated {
  type: 'OrderCreated';
  orderId: string;
  customerId: string;
  items: { sku: string; quantity: number }[];
  totalCents: number;
  currency: Currency;
  occurredAt: Date;
}

export interface OrderConfirmed {
  type: 'OrderConfirmed';
  orderId: string;
  customerId: string;
  totalCents: number;
  currency: Currency;
  occurredAt: Date;
}

export interface OrderCancelled {
  type: 'OrderCancelled';
  orderId: string;
  customerId: string;
  reason: CancellationReason;
  /** true = stock was reserved and must be released (saga compensation). */
  stockWasReserved: boolean;
  occurredAt: Date;
}

export type OrderEvent = OrderCreated | OrderConfirmed | OrderCancelled;
