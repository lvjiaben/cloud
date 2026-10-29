import { describe, expect, it } from "vitest";

import { deliveryStatusKey, serviceStatusKey } from "./commerce-delivery-state";

describe("commerce delivery display safety", () => {
  it("preserves published manual attention and rejects private status codes", () => {
    expect(deliveryStatusKey("processing")).toBe(
      "plugin.idc-core.commerce.orders.status.processing",
    );
    expect(deliveryStatusKey("manual_processing")).toBe("plugin.idc-core.fulfillment.status.manual_processing");
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
