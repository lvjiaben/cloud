import type { CustomerProduct } from "./customer-client";

export type ProductManagementKind = "legacy-server" | "native-server" | "ssl";

// Generic resource entitlements differ from the legacy core service-instance projection.
export function productManagementKind(row: CustomerProduct): ProductManagementKind | undefined {
  if (row.deliveryKind === "server") return "legacy-server";
  if (row.deliveryKind !== "entitlement") return;
  if (row.pluginId === "idc-zjmfcloud" && row.productType === "idc-resource") return "native-server";
  if (row.pluginId === "yunsu-ssl") return "ssl";
}

export function productManagementPermissions(kind: ProductManagementKind): string[] {
  if (kind === "ssl") return ["idc:admin:service-entitlement:list"];
  return [
    kind === "legacy-server" ? "idc:admin:resource:server-accessory:view" : "idc:admin:resource:server-batch:view",
    "idc:admin:resource:task:list",
    "idc:admin:resource:api-log:list",
  ];
}
