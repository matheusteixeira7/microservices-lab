import { DomainError } from '../../../shared/domain/domain-error';
import { Money } from '../../../shared/domain/money';

export class InvalidOrderItemError extends DomainError {
  readonly code = 'INVALID_ORDER_ITEM';
}

export interface OrderItemProps {
  sku: string;
  /** Snapshot of the product name at purchase time (see ADR 0001). */
  productName: string;
  /** Snapshot of the price at purchase time. Never looked up by the domain. */
  unitPrice: Money;
  quantity: number;
}

/**
 * One line of an order. Value object: immutable, no identity of its own.
 * It only exists inside an Order (the aggregate root).
 */
export class OrderItem {
  private constructor(private readonly props: OrderItemProps) {}

  static create(props: OrderItemProps): OrderItem {
    if (!props.sku.trim()) {
      throw new InvalidOrderItemError('SKU is required');
    }
    if (!props.productName.trim()) {
      throw new InvalidOrderItemError('Product name is required');
    }
    if (!Number.isSafeInteger(props.quantity) || props.quantity <= 0) {
      throw new InvalidOrderItemError(`Quantity must be a positive integer, got ${props.quantity}`);
    }
    return new OrderItem({ ...props });
  }

  get sku(): string {
    return this.props.sku;
  }

  get productName(): string {
    return this.props.productName;
  }

  get unitPrice(): Money {
    return this.props.unitPrice;
  }

  get quantity(): number {
    return this.props.quantity;
  }

  get subtotal(): Money {
    return this.props.unitPrice.multiply(this.props.quantity);
  }
}
