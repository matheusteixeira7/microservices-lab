import type { Order } from './order';

/**
 * Port: what the domain needs from storage. The domain only knows this
 * interface; the Drizzle implementation lives in infrastructure/.
 */
export interface OrderRepository {
  findById(id: string): Promise<Order | null>;
  save(order: Order): Promise<void>;
}
