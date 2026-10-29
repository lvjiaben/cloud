const deliveryStatuses = new Set([
  "awaiting_payment",
  "awaiting_review",
  "processing",
  "completed",
  "not_completed",
  "cancelled",
]);

const serviceStatuses = new Set(["active", "suspended", "expired", "terminated"]);

export function deliveryStatusKey(status: string) {
  if(status === "manual_processing") return "plugin.idc-core.fulfillment.status.manual_processing";
  return deliveryStatuses.has(status)
    ? `plugin.idc-core.commerce.orders.status.${status}`
    : "plugin.idc-core.commerce.orders.delivery.statusUnavailable";
}

export function serviceStatusKey(status: string) {
  return serviceStatuses.has(status)
    ? `plugin.idc-core.commerce.orders.status.${status}`
    : "plugin.idc-core.commerce.orders.service.statusUnavailable";
}
