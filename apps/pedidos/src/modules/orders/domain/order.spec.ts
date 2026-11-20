import { CurrencyMismatchError, Money } from '../../../shared/domain/money';
import { InvalidOrderItemError } from './order-item';
import { Order } from './order';
import {
  DuplicateItemError,
  EmptyOrderError,
  InvalidCustomerError,
  InvalidOrderTransitionError,
} from './order.errors';

const keyboard = { sku: 'KB-1', productName: 'Keyboard', unitPrice: Money.of(15000), quantity: 1 };
const mouse = { sku: 'MS-1', productName: 'Mouse', unitPrice: Money.of(5000), quantity: 2 };

function newOrder() {
  return Order.create({
    id: 'order-1',
    customerId: 'customer-1',
    items: [keyboard, mouse],
    now: new Date('2026-11-20T12:00:00Z'),
  });
}

/** An order in each status, with the creation event already consumed. */
function orderIn(status: 'PENDING' | 'STOCK_RESERVED' | 'CONFIRMED' | 'CANCELLED') {
  const order = newOrder();
  if (status === 'STOCK_RESERVED' || status === 'CONFIRMED') order.markStockReserved();
  if (status === 'CONFIRMED') order.confirm();
  if (status === 'CANCELLED') order.cancel('CUSTOMER_REQUESTED');
  order.pullEvents();
  return order;
}

describe('Order', () => {
  describe('create', () => {
    it('starts as PENDING with the given data', () => {
      const order = newOrder();

      expect(order.id).toBe('order-1');
      expect(order.customerId).toBe('customer-1');
      expect(order.status).toBe('PENDING');
      expect(order.createdAt).toEqual(new Date('2026-11-20T12:00:00Z'));
      expect(order.items).toHaveLength(2);
    });

    it('generates an id when none is given', () => {
      const order = Order.create({ customerId: 'customer-1', items: [keyboard] });

      expect(order.id).toMatch(/^[0-9a-f-]{36}$/);
    });

    it('computes the total from the items', () => {
      // 1 x 150.00 + 2 x 50.00 = 250.00
      expect(newOrder().total.equals(Money.of(25000))).toBe(true);
    });

    it('emits OrderCreated with what consumers need', () => {
      const events = newOrder().pullEvents();

      expect(events).toEqual([
        {
          type: 'OrderCreated',
          orderId: 'order-1',
          customerId: 'customer-1',
          items: [
            { sku: 'KB-1', quantity: 1 },
            { sku: 'MS-1', quantity: 2 },
          ],
          totalCents: 25000,
          currency: 'BRL',
          occurredAt: expect.any(Date),
        },
      ]);
    });

    it('rejects an order without items', () => {
      expect(() => Order.create({ customerId: 'customer-1', items: [] })).toThrow(EmptyOrderError);
    });

    it('rejects an empty customer id', () => {
      expect(() => Order.create({ customerId: ' ', items: [keyboard] })).toThrow(InvalidCustomerError);
    });

    it('rejects the same sku twice (send one line with a bigger quantity)', () => {
      expect(() => Order.create({ customerId: 'customer-1', items: [keyboard, keyboard] })).toThrow(
        DuplicateItemError,
      );
    });

    it('rejects items in different currencies', () => {
      const inDollars = { ...mouse, unitPrice: Money.of(5000, 'USD') };

      expect(() => Order.create({ customerId: 'customer-1', items: [keyboard, inDollars] })).toThrow(
        CurrencyMismatchError,
      );
    });

    it('validates each item', () => {
      expect(() =>
        Order.create({ customerId: 'customer-1', items: [{ ...keyboard, quantity: 0 }] }),
      ).toThrow(InvalidOrderItemError);
    });
  });

  describe('encapsulation', () => {
    it('does not let callers change the items from outside', () => {
      const order = newOrder();
      const items = order.items as unknown as unknown[];

      expect(() => items.pop()).toThrow(TypeError); // the array is frozen
      expect(order.items).toHaveLength(2);
    });

    it('pullEvents returns the events once and clears them', () => {
      const order = newOrder();

      expect(order.pullEvents()).toHaveLength(1);
      expect(order.pullEvents()).toEqual([]);
    });
  });

  describe('happy path: PENDING -> STOCK_RESERVED -> CONFIRMED', () => {
    it('marks stock as reserved', () => {
      const order = orderIn('PENDING');
      order.markStockReserved();

      expect(order.status).toBe('STOCK_RESERVED');
      expect(order.pullEvents()).toEqual([]);
    });

    it('confirms and emits OrderConfirmed', () => {
      const order = orderIn('STOCK_RESERVED');
      order.confirm();

      expect(order.status).toBe('CONFIRMED');
      expect(order.pullEvents()).toEqual([
        {
          type: 'OrderConfirmed',
          orderId: 'order-1',
          customerId: 'customer-1',
          totalCents: 25000,
          currency: 'BRL',
          occurredAt: expect.any(Date),
        },
      ]);
    });
  });

  describe('cancel', () => {
    it('from PENDING: no stock to release', () => {
      const order = orderIn('PENDING');
      order.cancel('OUT_OF_STOCK');

      expect(order.status).toBe('CANCELLED');
      expect(order.cancellationReason).toBe('OUT_OF_STOCK');
      expect(order.pullEvents()).toEqual([
        {
          type: 'OrderCancelled',
          orderId: 'order-1',
          customerId: 'customer-1',
          reason: 'OUT_OF_STOCK',
          stockWasReserved: false,
          occurredAt: expect.any(Date),
        },
      ]);
    });

    it('from STOCK_RESERVED: tells consumers to release the stock', () => {
      const order = orderIn('STOCK_RESERVED');
      order.cancel('PAYMENT_DECLINED');

      expect(order.status).toBe('CANCELLED');
      expect(order.pullEvents()).toEqual([
        expect.objectContaining({ type: 'OrderCancelled', reason: 'PAYMENT_DECLINED', stockWasReserved: true }),
      ]);
    });
  });

  describe('illegal transitions throw', () => {
    it.each([
      ['PENDING', 'confirm'],
      ['CONFIRMED', 'markStockReserved'],
      ['CONFIRMED', 'cancel'],
      ['CANCELLED', 'markStockReserved'],
      ['CANCELLED', 'confirm'],
    ] as const)('%s -> %s', (status, action) => {
      const order = orderIn(status);
      const act = () => (action === 'cancel' ? order.cancel('CUSTOMER_REQUESTED') : order[action]());

      expect(act).toThrow(InvalidOrderTransitionError);
      expect(order.status).toBe(status);
      expect(order.pullEvents()).toEqual([]);
    });
  });

  describe('repeating a transition is a no-op (safe for at-least-once messages)', () => {
    it.each([
      ['STOCK_RESERVED', 'markStockReserved'],
      ['CONFIRMED', 'confirm'],
      ['CANCELLED', 'cancel'],
    ] as const)('%s -> %s again', (status, action) => {
      const order = orderIn(status);
      const act = () => (action === 'cancel' ? order.cancel('CUSTOMER_REQUESTED') : order[action]());

      expect(act).not.toThrow();
      expect(order.status).toBe(status);
      expect(order.pullEvents()).toEqual([]);
    });
  });

  describe('persistence', () => {
    it('round-trips through a snapshot without emitting events', () => {
      const original = orderIn('STOCK_RESERVED');
      const restored = Order.restore(original.toSnapshot());

      expect(restored.toSnapshot()).toEqual(original.toSnapshot());
      expect(restored.total.equals(original.total)).toBe(true);
      expect(restored.pullEvents()).toEqual([]);
    });

    it('the snapshot holds only plain data', () => {
      expect(orderIn('CANCELLED').toSnapshot()).toEqual({
        id: 'order-1',
        customerId: 'customer-1',
        status: 'CANCELLED',
        cancellationReason: 'CUSTOMER_REQUESTED',
        createdAt: new Date('2026-11-20T12:00:00Z'),
        items: [
          { sku: 'KB-1', productName: 'Keyboard', unitPriceCents: 15000, currency: 'BRL', quantity: 1 },
          { sku: 'MS-1', productName: 'Mouse', unitPriceCents: 5000, currency: 'BRL', quantity: 2 },
        ],
      });
    });
  });
});
