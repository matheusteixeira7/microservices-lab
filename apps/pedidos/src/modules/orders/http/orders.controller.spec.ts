import type { INestApplication } from '@nestjs/common';
import { Test } from '@nestjs/testing';
import request from 'supertest';
import { ErrorHandlingModule } from '../../../shared/http/error-handling.module';
import type {
  InventoryGateway,
  ProductInfo,
  ReservationResult,
  ReserveItem,
} from '../application/ports/inventory.gateway';
import { InMemoryOrderRepository } from '../infrastructure/in-memory-order.repository';
import { OrdersModule } from '../orders.module';
import { INVENTORY_GATEWAY, ORDER_REPOSITORY } from '../orders.tokens';

/** Deterministic inventory for HTTP tests: no estoque, no network. */
class FakeInventory implements InventoryGateway {
  private readonly catalog: ProductInfo[] = [
    { sku: 'KB-1', name: 'Mechanical keyboard', priceCents: 45000, currency: 'BRL' },
    { sku: 'MS-1', name: 'Wireless mouse', priceCents: 15000, currency: 'BRL' },
    { sku: 'OUT-1', name: 'Always out of stock', priceCents: 1000, currency: 'BRL' },
  ];

  async findProducts(skus: string[]): Promise<ProductInfo[]> {
    return this.catalog.filter((product) => skus.includes(product.sku));
  }

  async reserve(_orderId: string, items: ReserveItem[]): Promise<ReservationResult> {
    const unavailableSkus = items.map((item) => item.sku).filter((sku) => sku === 'OUT-1');
    return unavailableSkus.length > 0 ? { status: 'OUT_OF_STOCK', unavailableSkus } : { status: 'RESERVED' };
  }
}

/*
 * HTTP tests without a database or estoque: the real module, with the
 * repository and the inventory swapped for in-memory fakes.
 */
describe('OrdersController (HTTP)', () => {
  let app: INestApplication;

  beforeEach(async () => {
    const module = await Test.createTestingModule({ imports: [OrdersModule, ErrorHandlingModule] })
      .overrideProvider(ORDER_REPOSITORY)
      .useValue(new InMemoryOrderRepository())
      .overrideProvider(INVENTORY_GATEWAY)
      .useValue(new FakeInventory())
      .compile();

    app = module.createNestApplication();
    app.setGlobalPrefix('api'); // same as main.ts
    await app.init();
  });

  afterEach(async () => {
    await app.close();
  });

  const post = (body: unknown) => request(app.getHttpServer()).post('/api/orders').send(body as object);

  describe('POST /api/orders', () => {
    it('201: creates the order and reserves stock', async () => {
      const res = await post({ customerId: 'customer-1', items: [{ sku: 'KB-1', quantity: 1 }] });

      expect(res.status).toBe(201);
      expect(res.body).toEqual({ orderId: expect.any(String), status: 'STOCK_RESERVED' });
    });

    it('201 with status CANCELLED when out of stock (the order exists)', async () => {
      const res = await post({ customerId: 'customer-1', items: [{ sku: 'OUT-1', quantity: 1 }] });

      expect(res.status).toBe(201);
      expect(res.body.status).toBe('CANCELLED');
    });

    it('400: wrong shape (types / missing fields)', async () => {
      const res = await post({ customerId: 42, items: 'not-a-list' });

      expect(res.status).toBe(400);
      expect(res.body.code).toBe('INVALID_REQUEST');
    });

    it('422: business rule from the domain (duplicate sku)', async () => {
      const res = await post({
        customerId: 'customer-1',
        items: [
          { sku: 'KB-1', quantity: 1 },
          { sku: 'KB-1', quantity: 2 },
        ],
      });

      expect(res.status).toBe(422);
      expect(res.body).toEqual({ code: 'DUPLICATE_ITEM', message: expect.any(String) });
    });

    it('422: unknown product', async () => {
      const res = await post({ customerId: 'customer-1', items: [{ sku: 'NOPE', quantity: 1 }] });

      expect(res.status).toBe(422);
      expect(res.body.code).toBe('UNKNOWN_PRODUCT');
    });
  });

  describe('GET /api/orders/:id', () => {
    it('200: returns the order with prices and total', async () => {
      const created = await post({ customerId: 'customer-1', items: [{ sku: 'MS-1', quantity: 2 }] });

      const res = await request(app.getHttpServer()).get(`/api/orders/${created.body.orderId}`);

      expect(res.status).toBe(200);
      expect(res.body).toMatchObject({
        id: created.body.orderId,
        status: 'STOCK_RESERVED',
        totalCents: 30000,
        currency: 'BRL',
        items: [{ sku: 'MS-1', productName: 'Wireless mouse', unitPriceCents: 15000, quantity: 2, subtotalCents: 30000 }],
      });
    });

    it('404: unknown order', async () => {
      const res = await request(app.getHttpServer()).get(`/api/orders/${crypto.randomUUID()}`);

      expect(res.status).toBe(404);
      expect(res.body.code).toBe('ORDER_NOT_FOUND');
    });

    it('400: id is not a UUID (the database column is uuid)', async () => {
      const res = await request(app.getHttpServer()).get('/api/orders/abc');

      expect(res.status).toBe(400);
    });
  });
});
