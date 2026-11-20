import { type Client, createClient } from '@connectrpc/connect';
import { createGrpcTransport } from '@connectrpc/connect-node';
import { InventoryService, type Money as MoneyMessage } from '../../../gen/inventory/v1/inventory_pb';
import type { Currency } from '../../../shared/domain/money';
import type {
  InventoryGateway,
  ProductInfo,
  ReservationResult,
  ReserveItem,
} from '../application/ports/inventory.gateway';

const CURRENCIES: readonly Currency[] = ['BRL', 'USD'];

export interface GrpcInventoryGatewayOptions {
  /** e.g. http://estoque:50051 */
  baseUrl: string;
  /** Deadline for every call. Without one, a stuck estoque hangs the request forever. */
  timeoutMs?: number;
}

/**
 * InventoryGateway over gRPC (the estoque service, written in Go).
 *
 * Only translation lives here: protobuf <-> port types. Any non-OK gRPC status
 * (unavailable, deadline, internal...) is thrown as a technical error.
 * The connection is lazy: pedidos boots even when estoque is down.
 */
export class GrpcInventoryGateway implements InventoryGateway {
  private readonly client: Client<typeof InventoryService>;

  constructor({ baseUrl, timeoutMs = 2_000 }: GrpcInventoryGatewayOptions) {
    // createGrpcTransport speaks real gRPC (grpc-go does not speak the Connect protocol).
    this.client = createClient(InventoryService, createGrpcTransport({ baseUrl, defaultTimeoutMs: timeoutMs }));
  }

  async findProducts(skus: string[]): Promise<ProductInfo[]> {
    const response = await this.client.findProducts({ skus });
    return response.products.map((product) => {
      const { cents, currency } = toMoney(product.sku, product.price);
      return { sku: product.sku, name: product.name, priceCents: cents, currency };
    });
  }

  async reserve(orderId: string, items: ReserveItem[]): Promise<ReservationResult> {
    const response = await this.client.reserveStock({ orderId, items });
    switch (response.result.case) {
      case 'reserved':
        return { status: 'RESERVED' };
      case 'outOfStock':
        return { status: 'OUT_OF_STOCK', unavailableSkus: response.result.value.unavailableSkus };
      default:
        throw new Error('Contract violation: ReserveStock returned no result');
    }
  }
}

/** Data from another service is checked, never cast. A bad value is a contract violation (5xx). */
function toMoney(sku: string, price: MoneyMessage | undefined): { cents: number; currency: Currency } {
  if (!price) {
    throw new Error(`Contract violation: product ${sku} has no price`);
  }
  const cents = Number(price.cents); // int64 arrives as bigint
  if (!Number.isSafeInteger(cents) || cents < 0) {
    throw new Error(`Contract violation: product ${sku} has an invalid price (${price.cents})`);
  }
  const currency = CURRENCIES.find((c) => c === price.currency);
  if (!currency) {
    throw new Error(`Contract violation: product ${sku} has an unknown currency "${price.currency}"`);
  }
  return { cents, currency };
}
