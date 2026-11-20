import { DomainError } from '../../../shared/domain/domain-error';
import type { OrderRepository } from '../domain/order.repository';
import { OrderView, toOrderView } from './order.view';

export class OrderNotFoundError extends DomainError {
  readonly code = 'ORDER_NOT_FOUND';

  constructor(id: string) {
    super(`Order ${id} not found`);
  }
}

export class GetOrderUseCase {
  constructor(private readonly orders: OrderRepository) {}

  async execute(id: string): Promise<OrderView> {
    const order = await this.orders.findById(id);
    if (!order) throw new OrderNotFoundError(id);
    return toOrderView(order);
  }
}
