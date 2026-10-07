import { describe, expect, it } from 'vitest';
import { formatMinorAmount } from './format-amount';

describe('formatMinorAmount', () => {
  it('formats two-decimal currencies from minor units', () => expect(formatMinorAmount('1000', 'CNY')).toBe('CNY 10.00'));
  it('formats zero-decimal currencies without dividing the amount', () => expect(formatMinorAmount('1000', 'JPY')).toBe('JPY 1000'));
  it('preserves values larger than the JavaScript safe integer', () => expect(formatMinorAmount('900719925474099301', 'CNY')).toBe('CNY 9007199254740993.01'));
});
