import { Module } from '@nestjs/common';
import { DB, type Database } from '../../shared/infrastructure/database/database.module';
import { CreateOrderUseCase } from './application/create-order.use-case';
import { GetOrderUseCase } from './application/get-order.use-case';
import type { InventoryGateway } from './application/ports/inventory.gateway';
import type { OrderRepository } from './domain/order.repository';
import { OrdersController } from './http/orders.controller';
import { DrizzleOrderRepository } from './infrastructure/drizzle-order.repository';
import { StubInventoryGateway } from './infrastructure/stub-inventory.gateway';
import { INVENTORY_GATEWAY, ORDER_REPOSITORY } from './orders.tokens';

/**
 * Composition root of the orders module: the only place that knows which
 * adapter implements each port. Domain and application stay framework-free.
 */
@Module({
  controllers: [OrdersController],
  providers: [
    {
      provide: ORDER_REPOSITORY,
      inject: [DB],
      useFactory: (db: Database) => new DrizzleOrderRepository(db),
    },
    {
      provide: INVENTORY_GATEWAY,
      useFactory: () => new StubInventoryGateway(), // TEMPORARY: gRPC client in Phase 1
    },
    {
      provide: CreateOrderUseCase,
      inject: [ORDER_REPOSITORY, INVENTORY_GATEWAY],
      useFactory: (orders: OrderRepository, inventory: InventoryGateway) =>
        new CreateOrderUseCase(orders, inventory),
    },
    {
      provide: GetOrderUseCase,
      inject: [ORDER_REPOSITORY],
      useFactory: (orders: OrderRepository) => new GetOrderUseCase(orders),
    },
  ],
})
export class OrdersModule {}
