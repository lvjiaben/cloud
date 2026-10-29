import { describe, expect, it, vi } from "vitest";

import { multiplyMajorAmount, compareMinorAmounts, majorToMinorAmount, minorToMajorAmount } from "./catalog-money";

describe("catalog money conversion", () => {
  it("converts two-decimal currencies without floating point", () => {
    expect(minorToMajorAmount("8800", "CNY")).toBe("88.00");
    expect(majorToMinorAmount("88.05", "CNY")).toBe("8805");
  });

  it("supports zero-decimal currencies", () => {
    expect(minorToMajorAmount("8800", "JPY")).toBe("8800");
    expect(majorToMinorAmount("8800", "JPY")).toBe("8800");
  });

  it("rejects excess decimals and integer overflow", () => {
    expect(() => majorToMinorAmount("1.001", "CNY")).toThrow();
    expect(() => majorToMinorAmount("92233720368547758.08", "CNY")).toThrow();
  });
});

// Vue runtime scopes may omit the BigInt constructor; prices must still be
// converted exactly rather than falling back to unscaled minor-unit values.
it('formats and compares money without a global BigInt constructor', () => {
 vi.stubGlobal('BigInt', undefined);
 try {
  expect(minorToMajorAmount('500', 'CNY')).toBe('5.00');
  expect(minorToMajorAmount('300', 'CNY')).toBe('3.00');
  expect(majorToMinorAmount('1.00', 'CNY')).toBe('100');
  expect(majorToMinorAmount('92233720368547758.07', 'CNY')).toBe('9223372036854775807');
  expect(minorToMajorAmount('000500', 'JPY')).toBe('500');
  expect(compareMinorAmounts('0300', '301')).toBe(-1);
  expect(compareMinorAmounts('9223372036854775807', '9223372036854775806')).toBe(1);
 } finally {vi.unstubAllGlobals();}
});


describe('merchant coefficient price calculation',()=>{
 it('rounds half up in minor units without binary floating-point drift',()=>{
  expect(multiplyMajorAmount('0.05','0.1','CNY')).toBe('0.01');
  expect(multiplyMajorAmount('5','3','CNY')).toBe('15.00');
  expect(multiplyMajorAmount('1','0.5','JPY')).toBe('1');
  expect(multiplyMajorAmount('1.234','1.5','KWD')).toBe('1.851');
  expect(multiplyMajorAmount('92233720368547758.07','1','CNY')).toBe('92233720368547758.07');
 });
 it('rejects invalid coefficients and overflow',()=>{
  for(const coefficient of ['-1','NaN','1e2','1000000','0.0000001']) expect(()=>multiplyMajorAmount('5',coefficient,'CNY')).toThrow();
  expect(()=>multiplyMajorAmount('92233720368547758.07','2','CNY')).toThrow();
 });
});
