export type CustomerFulfillmentStatus =
  | "awaiting_payment"
  | "awaiting_review"
  | "cancelled"
  | "completed"
  | "not_completed"
  | "manual_processing"
  | "processing";

const safeStatuses = new Set<CustomerFulfillmentStatus>([
  "awaiting_payment",
  "awaiting_review",
  "processing",
  "completed",
  "not_completed",
  "manual_processing",
  "cancelled",
]);

export function customerFulfillmentLabelKey(status: string) {
  return safeStatuses.has(status as CustomerFulfillmentStatus)
    ? `commerce.fulfillment.status.${status}`
    : "commerce.fulfillment.statusUnavailable";
}

export function customerFulfillmentColor(status: string) {
  switch (status) {
    case "completed":
      return "success";
    case "not_completed":
      return "error";
    case "awaiting_review":
    case "manual_processing":
      return "warning";
    case "processing":
      return "processing";
    default:
      return "default";
  }
}

export function customerDeliveryRows<T>(detail?: {
  deliveries?: readonly T[];
}) {
  return detail?.deliveries ?? [];
}

const safeServiceStatuses = new Set([
  "active",
  "expired",
  "suspended",
  "terminated",
]);

export function customerServiceLabelKey(status: string) {
  return safeServiceStatuses.has(status)
    ? `commerce.fulfillment.serviceStatus.${status}`
    : "commerce.fulfillment.serviceStatusUnavailable";
}

// A local management handoff does not reinterpret delivery or service status.
// Unknown readiness is not an invitation to enter an unconfirmed detail page.
export function managementPreparationLabelKey(status?: string) {
  if (!status || status === "ready") return "";
  return status === "preparing"
    ? "productDirectories.management.preparing"
    : "productDirectories.management.needs_attention";
}
