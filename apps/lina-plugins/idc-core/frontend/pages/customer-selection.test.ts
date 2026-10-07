import { beforeEach, describe, expect, it, vi } from "vitest";

const { customerList } = vi.hoisted(() => ({ customerList: vi.fn() }));
vi.mock("./customer-client", () => ({ customerList }));

import { customerDisplay, searchCustomerOptions } from "./customer-selection";

describe("customer selection", () => {
  beforeEach(() => customerList.mockReset());

  it("shows only the safe customer summary", () => {
    expect(customerDisplay({ customerCode: "10008", customerDisplayName: "张三", customerPrimaryPhone: "+8613800138000" }))
      .toBe("张三 · +8613800138000 · #10008");
  });

  it("deduplicates bounded data-scoped search results without exposing the internal value as label", async () => {
    customerList.mockResolvedValue({ customers: [{ customerNo: "CUS_internal", customerCode: "10008", displayName: "张三", primaryPhone: "+8613800138000" }] });
    const options = await searchCustomerOptions("13800138000");
    expect(customerList).toHaveBeenCalledTimes(3);
    expect(options).toEqual([{ label: "张三 · +8613800138000 · #10008", value: "CUS_internal" }]);
    expect(options[0]?.label).not.toContain("CUS_internal");
  });
});
