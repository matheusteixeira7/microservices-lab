import { DomainError } from './domain-error';

export type Currency = 'BRL' | 'USD';

export class InvalidMoneyError extends DomainError {
  readonly code = 'INVALID_MONEY';
}

export class CurrencyMismatchError extends DomainError {
  readonly code = 'CURRENCY_MISMATCH';

  constructor(a: Currency, b: Currency) {
    super(`Cannot combine ${a} with ${b}`);
  }
}

/**
 * Value object: an amount of money in integer cents.
 *
 * Integer cents avoid floating point errors (0.1 + 0.2 !== 0.3).
 * Immutable: every operation returns a new instance.
 */
export class Money {
  private constructor(
    readonly cents: number,
    readonly currency: Currency,
  ) {}

  static of(cents: number, currency: Currency = 'BRL'): Money {
    if (!Number.isSafeInteger(cents) || cents < 0) {
      throw new InvalidMoneyError(`Amount must be a non-negative integer number of cents, got ${cents}`);
    }
    return new Money(cents, currency);
  }

  static zero(currency: Currency = 'BRL'): Money {
    return new Money(0, currency);
  }

  add(other: Money): Money {
    this.assertSameCurrency(other);
    return Money.of(this.cents + other.cents, this.currency);
  }

  multiply(factor: number): Money {
    if (!Number.isSafeInteger(factor) || factor <= 0) {
      throw new InvalidMoneyError(`Factor must be a positive integer, got ${factor}`);
    }
    return Money.of(this.cents * factor, this.currency);
  }

  equals(other: Money): boolean {
    return this.cents === other.cents && this.currency === other.currency;
  }

  private assertSameCurrency(other: Money): void {
    if (other.currency !== this.currency) {
      throw new CurrencyMismatchError(this.currency, other.currency);
    }
  }
}
