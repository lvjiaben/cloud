import type { MarketingMissingCostItem } from "./marketing-client";

import { describe, expect, it } from "vitest";

describe("generic marketing cost contract", () => {
  it("uses product version and generic quantity without server binding fields", () => {
    const item: MarketingMissingCostItem = {
      orderItemNo: "order-item-public",
      orderNo: "order-public",
      versionNo: "version-public",
      currencyCode: "CNY",
      periodUnitCode: "once",
      periodCount: 1,
      quantity: 2,
      purchaseCycles: 1,
      productNameZhCn: "手机",
      productNameEnUs: "Phone",
      statusCode: "missing",
      unitCostMinor: null,
      totalCostMinor: null,
      correctionReason: "",
      version: 1,
      createdAt: 1,
      updatedAt: 1,
    };

    expect(item.versionNo).toBe("version-public");
    expect(item.quantity).toBe(2);
    expect(Object.keys(item)).not.toContain("bindingNo");
    expect(Object.keys(item)).not.toContain("serverQuantity");
  });
});
