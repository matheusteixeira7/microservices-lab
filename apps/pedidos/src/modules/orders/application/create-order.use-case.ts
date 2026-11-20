import { Money } from '../../../shared/domain/money';
import { Order, OrderStatus } from '../domain/order';
import type { OrderItemProps } from '../domain/order-item';
import type { OrderRepository } from '../domain/order.repository';
import { UnknownProductError } from './create-order.errors';
import type { InventoryGateway, ProductInfo, ReserveItem } from './ports/inventory.gateway';

export interface CreateOrderCommand {
  customerId: string;
  items: ReserveItem[];
}

export interface CreateOrderResult {
  orderId: string;
  status: OrderStatus;
}

/**
 * Application service: orchestrates the steps. The business rules live in the
 * Order aggregate; this class only decides the order of things.
 *
 * Plain class (no Nest decorators), so it stays testable without a framework.
 */
export class CreateOrderUseCase {
  constructor(
    private readonly orders: OrderRepository,
    private readonly inventory: InventoryGateway,
  ) {}

  async execute(command: CreateOrderCommand): Promise<CreateOrderResult> {
    // 1. Name and price come from the inventory (it owns the catalog, see ADR 0001).
    const products = await this.findProducts(command.items.map((item) => item.sku));

    // 2. The aggregate validates everything else (empty order, duplicates, quantities...).
    const order = Order.create({
      customerId: command.customerId,
      items: command.items.map((item) => this.toOrderItem(item, products)),
    });

    // 3. Save as PENDING *before* reserving, so a reservation never points to an
    //    order that does not exist.
    await this.orders.save(order);

    // 4. Reserve. A technical failure throws here and the order stays PENDING.
    const reservation = await this.inventory.reserve(order.id, command.items);
    if (reservation.status === 'RESERVED') {
      order.markStockReserved();
    } else {
      order.cancel('OUT_OF_STOCK');
    }

    // KNOWN GAP (dual write): if the process dies between step 4 and step 5, the
    // stock stays reserved and the order stays PENDING. Phase 2 (saga + outbox)
    // closes this gap.
    // 5. Save the new status.
    await this.orders.save(order);

    return { orderId: order.id, status: order.status };
  }

  private async findProducts(skus: string[]): Promise<Map<string, ProductInfo>> {
    const found = await this.inventory.findProducts([...new Set(skus)]);
    const bySku = new Map(found.map((product) => [product.sku, product]));

    const unknown = [...new Set(skus.filter((sku) => !bySku.has(sku)))];
    if (unknown.length > 0) throw new UnknownProductError(unknown);

    return bySku;
  }

  private toOrderItem(item: ReserveItem, products: Map<string, ProductInfo>): OrderItemProps {
    const product = products.get(item.sku);
    if (!product) throw new UnknownProductError([item.sku]); // unreachable: checked in findProducts
    return {
      sku: item.sku,
      productName: product.name,
      unitPrice: Money.of(product.priceCents, product.currency),
      quantity: item.quantity,
    };
  }
}
