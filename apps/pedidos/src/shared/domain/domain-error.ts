/**
 * Base class for every business-rule violation.
 *
 * The HTTP layer maps these to 4xx responses (e.g. 409/422). Anything that is
 * not a DomainError is a bug or an infrastructure failure (5xx).
 */
export abstract class DomainError extends Error {
  /** Stable, machine-readable code. Safe to expose in API responses. */
  abstract readonly code: string;

  constructor(message: string) {
    super(message);
    this.name = new.target.name;
  }
}
