export type FulfillmentMode =
  | "approval_then_plugin"
  | "manual"
  | "payment_confirmed_plugin";

export type FulfillmentTaskStatus =
  | "awaiting_payment"
  | "awaiting_review"
  | "processing"
  | "completed"
  | "not_completed"
  | "cancelled";

export type FulfillmentActionMode =
  | "manual_completed"
  | "manual_unable"
  | "review_approve"
  | "review_reject"
  | "takeover";

const knownModes = new Set<FulfillmentMode>([
  "manual",
  "approval_then_plugin",
  "payment_confirmed_plugin",
]);

const knownStatuses = new Set<FulfillmentTaskStatus>([
  "awaiting_payment",
  "awaiting_review",
  "processing",
  "completed",
  "not_completed",
  "cancelled",
]);

const managementTaskStatuses: FulfillmentTaskStatus[] = [
  "awaiting_review",
  "processing",
  "completed",
  "not_completed",
  "cancelled",
];

const knownEvents = new Set([
  "intent_created",
  "review_approved",
  "review_rejected",
  "automation_started",
  "automation_processing",
  "automation_completed",
  "automation_failed",
  "taken_over",
  "manual_completed",
  "manual_unable_to_complete",
  "moved_to_manual",
  "cancelled",
]);

export function requiresFulfillmentActionNote(mode: FulfillmentActionMode) {
  return mode === "manual_completed" || mode === "manual_unable";
}

export function fulfillmentModeTranslationKey(value: string) {
  return knownModes.has(value as FulfillmentMode)
    ? `plugin.idc-core.fulfillment.mode.${value}`
    : "plugin.idc-core.fulfillment.common.unavailable";
}

export function fulfillmentStatusTranslationKey(value: string) {
  return knownStatuses.has(value as FulfillmentTaskStatus)
    ? `plugin.idc-core.fulfillment.status.${value}`
    : "plugin.idc-core.fulfillment.common.unavailable";
}

export function fulfillmentStatusColor(value: string) {
  switch (value) {
    case "awaiting_payment":
      return "default";
    case "awaiting_review":
      return "warning";
    case "processing":
      return "processing";
    case "completed":
      return "success";
    case "not_completed":
      return "error";
    case "cancelled":
      return "default";
    default:
      return "default";
  }
}

export function fulfillmentModeOptions(t: (key: string) => string) {
  return [...knownModes].map((value) => ({
    label: t(fulfillmentModeTranslationKey(value)),
    value,
  }));
}

export function fulfillmentStatusOptions(t: (key: string) => string) {
  return managementTaskStatuses.map((value) => ({
    label: t(fulfillmentStatusTranslationKey(value)),
    value,
  }));
}

export function fulfillmentQuantityText(
  granularityCode: string,
  unitSequence: number,
  quantity: number,
  t: (key: string, values?: Record<string, number>) => string,
) {
  if (quantity < 1) return t("plugin.idc-core.fulfillment.common.unavailable");
  if (granularityCode === "per_order_item") {
    return t("plugin.idc-core.fulfillment.quantity.total", { quantity });
  }
  if (
    granularityCode === "per_quantity_unit" &&
    unitSequence >= 1 &&
    unitSequence <= quantity
  ) {
    return t("plugin.idc-core.fulfillment.quantity.sequence", {
      current: unitSequence,
      total: quantity,
    });
  }
  return t("plugin.idc-core.fulfillment.common.unavailable");
}

export function createFulfillmentActionAttempt(
  keyFactory: () => string,
) {
  return Object.freeze({ idempotencyKey: keyFactory() });
}

export function resetFulfillmentPaging() {
  return { cursor: "", history: [] as string[], nextCursor: "" };
}

export function fulfillmentEventTranslationKey(value: string) {
  return knownEvents.has(value)
    ? `plugin.idc-core.fulfillment.event.${value}`
    : "plugin.idc-core.fulfillment.common.unavailable";
}

export function fulfillmentEventReasonLabel(
  event: {
    reasonCode?: string;
    reasonLabelEnUS?: string;
    reasonLabelZhCN?: string;
  },
  english: boolean,
  unavailable: string,
) {
  if (!event.reasonCode) return "";
  const label = english ? event.reasonLabelEnUS : event.reasonLabelZhCN;
  return label?.trim() || unavailable;
}

export function settleNextFulfillmentPage(
  previousCursor: string,
  history: string[],
  succeeded: boolean,
) {
  return succeeded ? [...history, previousCursor] : [...history];
}

export function settlePreviousFulfillmentPage(
  history: string[],
  succeeded: boolean,
) {
  return succeeded ? history.slice(0, -1) : [...history];
}
