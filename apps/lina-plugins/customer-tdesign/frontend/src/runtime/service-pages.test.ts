import { describe, it, expect } from "vitest";
import { servicePages, type ServicePagePlugin } from "./service-pages";
describe("plugin-owned service pages", () => {
  const plugin: ServicePagePlugin = {
    id: "ssl",
    version: "v1.0.0",
    enabled: 1,
    installed: 1,
    customerSurfaces: [
      {
        pluginId: "ssl",
        pluginType: "dynamic",
        key: "certificates",
        version: "v1",
        title: "SSL",
        slot: "customer.products",
        bridgeCapabilities: ["purchased-services.customer.v1"],
        entry: "customer/list.html",
        entryUrl: "/x-assets/ssl/v1.0.0/customer/list.html",
      },
    ],
  };
  it("uses only enabled governed purchased-service surfaces", () => {
    expect(servicePages([plugin])[0].source).toBe(
      "/x-assets/ssl/v1.0.0/customer/list.html",
    );
    expect(servicePages([{ ...plugin, enabled: 0 }])).toEqual([]);
    expect(
      servicePages([{ ...plugin, runtimeState: "pending_upgrade" }]),
    ).toEqual([]);
    expect(
      servicePages([
        {
          ...plugin,
          customerSurfaces: [
            { ...plugin.customerSurfaces![0], bridgeCapabilities: [] },
          ],
        },
      ]),
    ).toEqual([]);
  });
  it("rejects foreign, remote and traversing entries", () => {
    for (const patch of [
      { pluginId: "other" },
      { entry: "../other.html" },
      { entryUrl: "https://evil.example/" },
      { entryUrl: "/x-assets/other/v1.0.0/customer/list.html" },
    ])
      expect(
        servicePages([
          {
            ...plugin,
            customerSurfaces: [{ ...plugin.customerSurfaces![0], ...patch }],
          },
        ]),
      ).toEqual([]);
  });
  it("rejects ambiguous or inactive content pages", () => {
    const surface = plugin.customerSurfaces![0];
    expect(
      servicePages([
        {
          ...plugin,
          customerSurfaces: [surface, { ...surface, key: "another-page" }],
        },
      ]),
    ).toEqual([]);
    expect(
      servicePages([
        { ...plugin, customerSurfaces: [{ ...surface, status: "inactive" }] },
      ]),
    ).toEqual([]);
    expect(
      servicePages([
        {
          ...plugin,
          customerSurfaces: [{ ...surface, slot: "customer.finance" }],
        },
      ]),
    ).toEqual([]);
  });
});
