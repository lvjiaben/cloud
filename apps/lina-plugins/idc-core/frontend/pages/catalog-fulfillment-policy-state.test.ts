import { describe, expect, it } from "vitest";

import {
  catalogFulfillmentStrategies,
  fulfillmentStrategyRequiresBinding,
} from "./catalog-fulfillment-policy-state";

describe("catalog fulfillment policy", () => {
  it("offers only the three confirmed merchant choices", () => {
    expect(catalogFulfillmentStrategies).toEqual([
      "manual",
      "approval_then_plugin",
      "payment_confirmed_plugin",
    ]);
  });

  it("requires an interface only for plugin fulfillment", () => {
    expect(fulfillmentStrategyRequiresBinding("manual")).toBe(false);
    expect(fulfillmentStrategyRequiresBinding("approval_then_plugin")).toBe(
      true,
    );
    expect(fulfillmentStrategyRequiresBinding("payment_confirmed_plugin")).toBe(
      true,
    );
  });
});
