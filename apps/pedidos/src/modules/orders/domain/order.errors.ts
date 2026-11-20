import { DomainError } from '../../../shared/domain/domain-error';
import type { OrderStatus } from './order';

export class EmptyOrderError extends DomainError {
  readonly code = 'EMPTY_ORDER';

  constructor() {
    super('An order needs at least one item');
  }
}

export class InvalidCustomerError extends DomainError {
  readonly code = 'INVALID_CUSTOMER';

  constructor() {
    super('Customer id is required');
  }
}

export class DuplicateItemError extends DomainError {
  readonly code = 'DUPLICATE_ITEM';

  constructor(sku: string) {
    super(`SKU ${sku} appears more than once; send one line with the total quantity`);
  }
}

export class InvalidOrderTransitionError extends DomainError {
  readonly code = 'INVALID_ORDER_TRANSITION';

  constructor(
    readonly from: OrderStatus,
    readonly action: string,
  ) {
    super(`Cannot ${action} an order that is ${from}`);
  }
}
