import type { DirectoryService } from "#/api/idc/product-directories";

import { managementPreparationLabelKey } from "../commerce/customer-fulfillment-state";

// The connector is not the service owner. Resolve the customer's public server
// reference only after clicking, without a detail request for every list row.
export function serviceManagementPath(service: DirectoryService) {
  if (
    service.entitlementOwnerId !== "idc-resource" ||
    !["active", "suspended", "expired", "terminated"].includes(service.statusCode) ||
    managementPreparationLabelKey(service.managementPreparation)
  ) return "";
  return `/account/native-servers/by-entitlement/${encodeURIComponent(service.entitlementNo)}`;
}
