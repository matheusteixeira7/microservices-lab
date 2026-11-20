import type { Currency } from '../../../shared/domain/money';
import type { CancellationReason, Order, OrderStatus } from '../domain/order';

/** Read model returned to callers. Plain data, safe to serialize as JSON. */
export interface OrderView {
  id: string;
  customerId: string;
  status: OrderStatus;
  cancellationReason: CancellationReason | null;
  totalCents: number;
  currency: Currency;
  createdAt: string;
  items: {
    sku: string;
    productName: string;
    unitPriceCents: number;
    quantity: number;
    subtotalCents: number;
  }[];
}

export function toOrderView(order: Order): OrderView {
  return {
    id: order.id,
    customerId: order.customerId,
    status: order.status,
    cancellationReason: order.cancellationReason,
    totalCents: order.total.cents,
    currency: order.total.currency,
    createdAt: order.createdAt.toISOString(),
    items: order.items.map((item) => ({
      sku: item.sku,
      productName: item.productName,
      unitPriceCents: item.unitPrice.cents,
      quantity: item.quantity,
      subtotalCents: item.subtotal.cents,
    })),
  };
}
