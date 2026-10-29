import { describe, it, expect } from "vitest";
import { money, minorAmount, stateLabel } from "./business";
describe("金额边界", () => {
  it("不通过浮点数丢失大额分币精度", () => {
    expect(money("900719925474099312", "CNY")).toBe(
      "CNY 9,007,199,254,740,993.12",
    );
    expect(minorAmount("9007199254740993.12", "CNY")).toBe(
      "900719925474099312",
    );
  });
  it("按币种精度处理零位、三位与负数", () => {
    expect(money("-125", "CNY")).toBe("CNY -1.25");
    expect(minorAmount("12", "JPY")).toBe("12");
    expect(minorAmount("1.234", "KWD")).toBe("1234");
    expect(() => minorAmount("1.1", "JPY")).toThrow();
    expect(() => minorAmount("0", "CNY")).toThrow();
    expect(() => minorAmount("1.234", "CNY")).toThrow();
  });
  it("未知状态不冒充已处理", () =>
    expect(stateLabel("new_unrecognized_status")).toBe("状态待确认"));
});
