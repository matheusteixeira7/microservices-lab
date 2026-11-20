import { DomainError } from '../../../shared/domain/domain-error';

/**
 * Not an Order rule, but still the caller's fault (4xx), so it extends
 * DomainError: the HTTP layer maps every DomainError to a 4xx response.
 */
export class UnknownProductError extends DomainError {
  readonly code = 'UNKNOWN_PRODUCT';

  constructor(readonly skus: string[]) {
    super(`Unknown product(s): ${skus.join(', ')}`);
  }
}
