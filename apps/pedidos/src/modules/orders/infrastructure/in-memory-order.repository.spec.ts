import { InMemoryOrderRepository } from './in-memory-order.repository';
import { orderRepositoryContract } from './order.repository.contract';

describe('InMemoryOrderRepository', () => {
  orderRepositoryContract(async () => {
    const repository = new InMemoryOrderRepository();
    return {
      repository,
      storedEvents: async () =>
        repository.outbox.map((event) => ({ eventType: event.type, aggregateId: event.orderId })),
    };
  });
});
