import { Body, Controller, Get, Param, ParseUUIDPipe, Post } from '@nestjs/common';
import { ZodValidationPipe } from '../../../shared/http/zod-validation.pipe';
import { type CreateOrderResult, CreateOrderUseCase } from '../application/create-order.use-case';
import { GetOrderUseCase } from '../application/get-order.use-case';
import type { OrderView } from '../application/order.view';
import { type CreateOrderDto, createOrderSchema } from './create-order.dto';

/**
 * HTTP adapter: translates HTTP <-> use cases. No business rules here.
 *
 * An out-of-stock order still answers 201: the order exists (as CANCELLED)
 * and the body says so.
 */
@Controller('orders')
export class OrdersController {
  constructor(
    private readonly createOrder: CreateOrderUseCase,
    private readonly getOrder: GetOrderUseCase,
  ) {}

  @Post()
  create(@Body(new ZodValidationPipe(createOrderSchema)) body: CreateOrderDto): Promise<CreateOrderResult> {
    return this.createOrder.execute(body);
  }

  @Get(':id')
  findOne(@Param('id', new ParseUUIDPipe()) id: string): Promise<OrderView> {
    return this.getOrder.execute(id);
  }
}
