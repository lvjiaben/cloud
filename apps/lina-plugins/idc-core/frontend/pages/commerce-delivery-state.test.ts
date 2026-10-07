import { describe, expect, it } from "vitest";

import { deliveryStatusKey, serviceStatusKey } from "./commerce-delivery-state";

describe("commerce delivery display safety", () => {
  it("maps only the six customer-safe delivery states", () => {
    expect(deliveryStatusKey("processing")).toBe(
      "plugin.idc-core.commerce.orders.status.processing",
    );
    expect(deliveryStatusKey("result_unknown")).toBe(
      "plugin.idc-core.commerce.orders.delivery.statusUnavailable",
    );
  });

  it("does not expose unknown service state codes", () => {
    expect(serviceStatusKey("active")).toBe(
      "plugin.idc-core.commerce.orders.status.active",
    );
    expect(serviceStatusKey("plugin_error")).toBe(
      "plugin.idc-core.commerce.orders.service.statusUnavailable",
    );
  });
});
