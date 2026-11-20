import { sql } from 'drizzle-orm';
import {
  check,
  integer,
  pgTable,
  primaryKey,
  smallint,
  text,
  timestamp,
  unique,
  uuid,
} from 'drizzle-orm/pg-core';
import type { Currency } from '../../../shared/domain/money';
import type { CancellationReason, OrderStatus } from '../domain/order';

/*
 * The CHECK constraints repeat the domain rules on purpose: the domain is the
 * first line of defense, the database is the last one.
 * The total is not stored: it is always computed from the items.
 */

export const orders = pgTable(
  'orders',
  {
    id: uuid('id').primaryKey(),
    customerId: text('customer_id').notNull(),
    status: text('status').$type<OrderStatus>().notNull(),
    cancellationReason: text('cancellation_reason').$type<CancellationReason>(),
    createdAt: timestamp('created_at', { withTimezone: true }).notNull(),
    updatedAt: timestamp('updated_at', { withTimezone: true }).notNull().defaultNow(),
  },
  (t) => [
    check('orders_status_check', sql`${t.status} in ('PENDING', 'STOCK_RESERVED', 'CONFIRMED', 'CANCELLED')`),
    check(
      'orders_cancellation_reason_check',
      sql`${t.cancellationReason} in ('OUT_OF_STOCK', 'PAYMENT_DECLINED', 'CUSTOMER_REQUESTED')`,
    ),
    // A reason exists if and only if the order is cancelled.
    check(
      'orders_cancellation_reason_matches_status',
      sql`(${t.status} = 'CANCELLED') = (${t.cancellationReason} is not null)`,
    ),
  ],
);

export const orderItems = pgTable(
  'order_items',
  {
    orderId: uuid('order_id')
      .notNull()
      .references(() => orders.id, { onDelete: 'cascade' }),
    /** Postgres does not keep insertion order; this column does. */
    position: smallint('position').notNull(),
    sku: text('sku').notNull(),
    productName: text('product_name').notNull(),
    unitPriceCents: integer('unit_price_cents').notNull(),
    currency: text('currency').$type<Currency>().notNull(),
    quantity: integer('quantity').notNull(),
  },
  (t) => [
    primaryKey({ columns: [t.orderId, t.sku] }),
    unique('order_items_order_position_unique').on(t.orderId, t.position),
    check('order_items_quantity_check', sql`${t.quantity} > 0`),
    check('order_items_unit_price_check', sql`${t.unitPriceCents} >= 0`),
  ],
);
