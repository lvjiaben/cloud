import { describe, expect, it } from 'vitest';
import { consoleMoney } from './console-money';

describe('console balances retain integer minor-unit precision', () => {
  it('formats signed fractions and zero using currency precision', () => {
    expect(consoleMoney('-1', 'CNY', 'en-US')).toBe('-CN¥0.01');
    expect(consoleMoney('0', 'CNY', 'zh-CN')).toBe('¥0.00');
    expect(consoleMoney('1234', 'JPY', 'en-US')).toBe('¥1,234');
    expect(consoleMoney('1234', 'BHD', 'en-US')).toContain('1.234');
  });
  it('does not round balances beyond Number safe integer limits', () => {
    expect(consoleMoney('9223372036854775807', 'USD', 'en-US')).toBe('$92,233,720,368,547,758.07');
    expect(consoleMoney('-9223372036854775807', 'USD', 'en-US')).toBe('-$92,233,720,368,547,758.07');
    expect(consoleMoney('invalid', 'USD', 'en-US')).toBe('—');
  });
});
