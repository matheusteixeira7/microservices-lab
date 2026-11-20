import { CurrencyMismatchError, InvalidMoneyError, Money } from './money';

describe('Money', () => {
  it('stores the amount in integer cents with a currency', () => {
    const money = Money.of(1050, 'BRL');

    expect(money.cents).toBe(1050);
    expect(money.currency).toBe('BRL');
  });

  it('defaults to BRL', () => {
    expect(Money.of(100).currency).toBe('BRL');
  });

  it.each([10.5, -1, Number.NaN, Number.POSITIVE_INFINITY])('rejects %s cents', (cents) => {
    expect(() => Money.of(cents)).toThrow(InvalidMoneyError);
  });

  it('adds two amounts in the same currency', () => {
    expect(Money.of(100).add(Money.of(250)).equals(Money.of(350))).toBe(true);
  });

  it('refuses to add different currencies', () => {
    expect(() => Money.of(100, 'BRL').add(Money.of(100, 'USD'))).toThrow(CurrencyMismatchError);
  });

  it('multiplies by a positive integer', () => {
    expect(Money.of(199).multiply(3).equals(Money.of(597))).toBe(true);
  });

  it.each([0, -2, 1.5])('refuses to multiply by %s', (factor) => {
    expect(() => Money.of(100).multiply(factor)).toThrow(InvalidMoneyError);
  });

  it('is immutable: operations return a new instance', () => {
    const original = Money.of(100);
    original.add(Money.of(50));

    expect(original.cents).toBe(100);
  });

  it('compares by value, not by reference', () => {
    expect(Money.of(100, 'BRL').equals(Money.of(100, 'BRL'))).toBe(true);
    expect(Money.of(100, 'BRL').equals(Money.of(100, 'USD'))).toBe(false);
    expect(Money.of(100, 'BRL').equals(Money.of(101, 'BRL'))).toBe(false);
  });
});
