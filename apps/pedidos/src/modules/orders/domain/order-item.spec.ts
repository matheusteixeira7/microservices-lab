import { Money } from '../../../shared/domain/money';
import { InvalidOrderItemError, OrderItem } from './order-item';

const validItem = {
  sku: 'SKU-1',
  productName: 'Keyboard',
  unitPrice: Money.of(15000),
  quantity: 2,
};

describe('OrderItem', () => {
  it('computes the subtotal as unit price x quantity', () => {
    const item = OrderItem.create(validItem);

    expect(item.subtotal.equals(Money.of(30000))).toBe(true);
  });

  it.each([0, -1, 1.5])('rejects quantity %s', (quantity) => {
    expect(() => OrderItem.create({ ...validItem, quantity })).toThrow(InvalidOrderItemError);
  });

  it('rejects an empty sku', () => {
    expect(() => OrderItem.create({ ...validItem, sku: '  ' })).toThrow(InvalidOrderItemError);
  });

  it('rejects an empty product name', () => {
    expect(() => OrderItem.create({ ...validItem, productName: '' })).toThrow(InvalidOrderItemError);
  });
});
