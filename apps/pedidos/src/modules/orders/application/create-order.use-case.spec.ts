import { InMemoryOrderRepository } from '../infrastructure/in-memory-order.repository';
import { UnknownProductError } from './create-order.errors';
import { CreateOrderUseCase } from './create-order.use-case';
import type { InventoryGateway, ProductInfo, ReservationResult, ReserveItem } from './ports/inventory.gateway';

/** Test double for the inventory (estoque) service. */
class FakeInventory implements InventoryGateway {
  readonly products = new Map<string, ProductInfo>();
  readonly outOfStock = new Set<string>();
  readonly reserveCalls: { orderId: string; items: ReserveItem[] }[] = [];
  reserveError: Error | null = null;

  addProduct(product: ProductInfo) {
    this.products.set(product.sku, product);
  }

  async findProducts(skus: string[]): Promise<ProductInfo[]> {
    return skus.flatMap((sku) => this.products.get(sku) ?? []);
  }

  async reserve(orderId: string, items: ReserveItem[]): Promise<ReservationResult> {
    this.reserveCalls.push({ orderId, items });
    if (this.reserveError) throw this.reserveError;

    const unavailableSkus = items.filter((item) => this.outOfStock.has(item.sku)).map((item) => item.sku);
    return unavailableSkus.length > 0 ? { status: 'OUT_OF_STOCK', unavailableSkus } : { status: 'RESERVED' };
  }
}

function setup() {
  const orders = new InMemoryOrderRepository();
  const inventory = new FakeInventory();
  inventory.addProduct({ sku: 'KB-1', name: 'Keyboard', priceCents: 15000, currency: 'BRL' });
  inventory.addProduct({ sku: 'MS-1', name: 'Mouse', priceCents: 5000, currency: 'BRL' });
  const useCase = new CreateOrderUseCase(orders, inventory);
  return { orders, inventory, useCase };
}

const command = {
  customerId: 'customer-1',
  items: [
    { sku: 'KB-1', quantity: 1 },
    { sku: 'MS-1', quantity: 2 },
  ],
};

describe('CreateOrderUseCase', () => {
  describe('when every item is in stock', () => {
    it('saves the order as STOCK_RESERVED', async () => {
      const { orders, useCase } = setup();

      const result = await useCase.execute(command);

      expect(result.status).toBe('STOCK_RESERVED');
      const saved = await orders.findById(result.orderId);
      expect(saved?.status).toBe('STOCK_RESERVED');
    });

    it('stores a snapshot of name and price taken from the inventory', async () => {
      const { orders, useCase } = setup();

      const { orderId } = await useCase.execute(command);

      const saved = await orders.findById(orderId);
      expect(saved?.toSnapshot().items).toEqual([
        { sku: 'KB-1', productName: 'Keyboard', unitPriceCents: 15000, currency: 'BRL', quantity: 1 },
        { sku: 'MS-1', productName: 'Mouse', unitPriceCents: 5000, currency: 'BRL', quantity: 2 },
      ]);
    });

    it('reserves using the order id (future idempotency key)', async () => {
      const { inventory, useCase } = setup();

      const { orderId } = await useCase.execute(command);

      expect(inventory.reserveCalls).toEqual([{ orderId, items: command.items }]);
    });

    it('stores each event exactly once', async () => {
      const { orders, useCase } = setup();

      await useCase.execute(command);

      expect(orders.outbox.map((event) => event.type)).toEqual(['OrderCreated']);
    });
  });

  describe('when an item is out of stock', () => {
    it('saves the order as CANCELLED with OUT_OF_STOCK', async () => {
      const { orders, inventory, useCase } = setup();
      inventory.outOfStock.add('MS-1');

      const result = await useCase.execute(command);

      expect(result.status).toBe('CANCELLED');
      const saved = await orders.findById(result.orderId);
      expect(saved?.status).toBe('CANCELLED');
      expect(saved?.cancellationReason).toBe('OUT_OF_STOCK');
    });

    it('records OrderCreated and OrderCancelled (no stock to release)', async () => {
      const { orders, inventory, useCase } = setup();
      inventory.outOfStock.add('MS-1');

      await useCase.execute(command);

      expect(orders.outbox).toEqual([
        expect.objectContaining({ type: 'OrderCreated' }),
        expect.objectContaining({ type: 'OrderCancelled', reason: 'OUT_OF_STOCK', stockWasReserved: false }),
      ]);
    });
  });

  describe('when a product does not exist', () => {
    it('fails without saving or reserving anything', async () => {
      const { orders, inventory, useCase } = setup();

      const promise = useCase.execute({ customerId: 'customer-1', items: [{ sku: 'NOPE', quantity: 1 }] });

      await expect(promise).rejects.toBeInstanceOf(UnknownProductError);
      expect(orders.size).toBe(0);
      expect(inventory.reserveCalls).toEqual([]);
    });
  });

  describe('when the inventory service fails (network, timeout...)', () => {
    it('propagates the error and leaves the order saved as PENDING', async () => {
      const { orders, inventory, useCase } = setup();
      const networkError = new Error('connect ECONNREFUSED');
      inventory.reserveError = networkError;

      await expect(useCase.execute(command)).rejects.toBe(networkError);

      const [saved] = orders.all();
      expect(saved.status).toBe('PENDING');
    });
  });
});
