import { z } from 'zod';

/**
 * Only the SHAPE of the request (types, required fields) -> 400 if wrong.
 * Business rules (empty order, duplicate SKU, quantity > 0...) belong to the
 * domain -> 422 with a domain error code.
 */
export const createOrderSchema = z.object({
  customerId: z.string(),
  items: z.array(
    z.object({
      sku: z.string(),
      quantity: z.number(),
    }),
  ),
});

export type CreateOrderDto = z.infer<typeof createOrderSchema>;
