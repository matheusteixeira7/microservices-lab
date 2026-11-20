import type {
  InventoryGateway,
  ProductInfo,
  ReservationResult,
  ReserveItem,
} from '../application/ports/inventory.gateway';

const CATALOG: ProductInfo[] = [
  { sku: 'KB-1', name: 'Mechanical keyboard', priceCents: 45000, currency: 'BRL' },
  { sku: 'MS-1', name: 'Wireless mouse', priceCents: 15000, currency: 'BRL' },
  { sku: 'MN-1', name: '27" monitor', priceCents: 189900, currency: 'BRL' },
  { sku: 'OUT-1', name: 'Always out of stock', priceCents: 1000, currency: 'BRL' },
];

const ALWAYS_OUT_OF_STOCK = new Set(['OUT-1']);

/**
 * TEMPORARY: a fake inventory with a fixed catalog, so POST /api/orders works
 * before the estoque service speaks gRPC. Phase 1 replaces it with a gRPC
 * client that implements the same InventoryGateway port.
 */
export class StubInventoryGateway implements InventoryGateway {
  async findProducts(skus: string[]): Promise<ProductInfo[]> {
    return CATALOG.filter((product) => skus.includes(product.sku));
  }

  async reserve(_orderId: string, items: ReserveItem[]): Promise<ReservationResult> {
    const unavailableSkus = items.map((item) => item.sku).filter((sku) => ALWAYS_OUT_OF_STOCK.has(sku));
    return unavailableSkus.length > 0 ? { status: 'OUT_OF_STOCK', unavailableSkus } : { status: 'RESERVED' };
  }
}
