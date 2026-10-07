import { describe, expect, it } from "vitest";

import { majorToMinorAmount, minorToMajorAmount } from "./catalog-money";

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
