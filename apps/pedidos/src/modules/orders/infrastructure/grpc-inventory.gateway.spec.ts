import { create } from '@bufbuild/protobuf';
import { Code, ConnectError, type ServiceImpl } from '@connectrpc/connect';
import { connectNodeAdapter } from '@connectrpc/connect-node';
import { createServer, type Http2Server } from 'node:http2';
import type { AddressInfo } from 'node:net';
import {
  InventoryService,
  ReserveStockResponseSchema,
} from '../../../gen/inventory/v1/inventory_pb';
import { GrpcInventoryGateway } from './grpc-inventory.gateway';

/*
 * The adapter talks to a REAL gRPC server (HTTP/2, random port) running inside
 * the test. Real protocol, no estoque needed.
 */

let currency = 'BRL';
let reserveDelayMs = 0;

const fakeInventory: Partial<ServiceImpl<typeof InventoryService>> = {
  async findProducts(req) {
    return {
      products: req.skus
        .filter((sku) => sku !== 'NOPE')
        .map((sku) => ({ sku, name: `Product ${sku}`, price: { cents: 15000n, currency } })),
    };
  },
  async reserveStock(req) {
    await new Promise((resolve) => setTimeout(resolve, reserveDelayMs));
    const unavailable = req.items.filter((item) => item.sku.startsWith('OUT')).map((item) => item.sku);
    return create(ReserveStockResponseSchema, {
      result:
        unavailable.length > 0
          ? { case: 'outOfStock', value: { unavailableSkus: unavailable } }
          : { case: 'reserved', value: {} },
    });
  },
};

let server: Http2Server;
let baseUrl: string;

beforeAll(async () => {
  server = createServer(
    connectNodeAdapter({ routes: (router) => router.service(InventoryService, fakeInventory) }),
  );
  await new Promise<void>((resolve) => server.listen(0, resolve));
  baseUrl = `http://localhost:${(server.address() as AddressInfo).port}`;
});

afterAll(async () => {
  await new Promise((resolve) => server.close(resolve));
});

beforeEach(() => {
  currency = 'BRL';
  reserveDelayMs = 0;
});

describe('GrpcInventoryGateway', () => {
  describe('findProducts', () => {
    it('maps products and converts int64 cents (bigint) to number', async () => {
      const gateway = new GrpcInventoryGateway({ baseUrl });

      const products = await gateway.findProducts(['KB-1', 'NOPE']);

      expect(products).toEqual([{ sku: 'KB-1', name: 'Product KB-1', priceCents: 15000, currency: 'BRL' }]);
    });

    it('rejects an unknown currency as a contract violation (not a 4xx domain error)', async () => {
      currency = 'XYZ';
      const gateway = new GrpcInventoryGateway({ baseUrl });

      await expect(gateway.findProducts(['KB-1'])).rejects.toThrow(/currency/);
    });
  });

  describe('reserve', () => {
    it('RESERVED', async () => {
      const gateway = new GrpcInventoryGateway({ baseUrl });

      await expect(gateway.reserve('order-1', [{ sku: 'KB-1', quantity: 1 }])).resolves.toEqual({
        status: 'RESERVED',
      });
    });

    it('OUT_OF_STOCK is a result, with every unavailable SKU', async () => {
      const gateway = new GrpcInventoryGateway({ baseUrl });

      const result = await gateway.reserve('order-1', [
        { sku: 'OUT-1', quantity: 1 },
        { sku: 'KB-1', quantity: 1 },
        { sku: 'OUT-2', quantity: 1 },
      ]);

      expect(result).toEqual({ status: 'OUT_OF_STOCK', unavailableSkus: ['OUT-1', 'OUT-2'] });
    });

    it('gives up after the deadline instead of hanging', async () => {
      reserveDelayMs = 1_000;
      const gateway = new GrpcInventoryGateway({ baseUrl, timeoutMs: 100 });

      const error = await gateway.reserve('order-1', [{ sku: 'KB-1', quantity: 1 }]).catch((e: unknown) => e);

      expect(ConnectError.from(error).code).toBe(Code.DeadlineExceeded);
    });
  });

  it('fails as a technical error when estoque is down', async () => {
    const gateway = new GrpcInventoryGateway({ baseUrl: 'http://localhost:1' });

    const error = await gateway.findProducts(['KB-1']).catch((e: unknown) => e);

    expect(ConnectError.from(error).code).toBe(Code.Unavailable);
  });
});
