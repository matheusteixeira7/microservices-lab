import type { Currency } from '../../../../shared/domain/money';

/*
 * Port: what the orders module needs from the inventory (estoque) service.
 * Plain data only. This shape is the first draft of the gRPC contract.
 */

export interface ProductInfo {
  sku: string;
  name: string;
  priceCents: number;
  currency: Currency;
}

export interface ReserveItem {
  sku: string;
  quantity: number;
}

/**
 * A business "no" (out of stock) is a normal result, not an exception.
 * Technical failures (timeout, network) are thrown. Keeping them apart lets a
 * circuit breaker react only to real failures.
 */
export type ReservationResult =
  | { status: 'RESERVED' }
  | { status: 'OUT_OF_STOCK'; unavailableSkus: string[] };

export interface InventoryGateway {
  /** Returns only the products that exist. Missing SKUs are simply absent. */
  findProducts(skus: string[]): Promise<ProductInfo[]>;

  /** `orderId` doubles as the idempotency key: reserving twice for the same order is safe. */
  reserve(orderId: string, items: ReserveItem[]): Promise<ReservationResult>;
}
