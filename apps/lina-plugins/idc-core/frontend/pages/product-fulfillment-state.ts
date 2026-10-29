export type FulfillmentMode =
  | "approval_then_plugin"
  | "manual"
  | "payment_confirmed_plugin";

export type FulfillmentTaskStatus =
  | "awaiting_payment"
  | "awaiting_review"
  | "processing"
  | "manual_processing"
  | "completed"
  | "not_completed"
  | "cancelled";

export type FulfillmentActionMode =
  | "verify_non_acceptance"
  | "manual_completed"
  | "manual_unable"
  | "review_approve"
  | "review_reject"
  | "recover_preparation"
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
  "manual_processing",
  "completed",
  "not_completed",
  "cancelled",
]);

const managementTaskStatuses: FulfillmentTaskStatus[] = [
  "awaiting_review",
  "processing",
  "manual_processing",
  "completed",
  "not_completed",
  "cancelled",
];

const knownEvents = new Set([
  "preparation_recovered",
  "management_repair_queued",
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
  return (
    mode === "verify_non_acceptance" ||
    mode === "manual_completed" ||
    mode === "manual_unable" ||
    mode === "recover_preparation"
  );
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
    case "manual_processing":
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

export function createFulfillmentActionAttempt(keyFactory: () => string) {
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

// Operator text is selected from reviewed categories; unknown codes are never
// rendered verbatim and never enable a provisioning or financial action.
export function fulfillmentFailureTranslationKey(execution?: {
  failureCode?: string;
  lastResultCode?: string;
  statusCode: string;
}) {
  if (execution?.lastResultCode === "operator_non_acceptance_verified")
    return "plugin.idc-core.fulfillment.diagnostic.reason.verifiedAbsent";
  const categories: Record<string, string> = {
    resource_preparation_request_invalid: "localRequest",
    resource_preparation_lookup_failed: "localLookup",
    resource_preparation_parameters_unavailable: "localParameters",
    resource_preparation_credential_failed: "localCredential",
    resource_preparation_callback_unavailable: "localCallback",
    resource_preparation_task_input_invalid: "localTaskInput",
    resource_preparation_task_storage_failed: "localTaskStorage",
    provider_request_not_sent: "notSent",
    upstream_request_rejected: "rejected",
    upstream_task_failed: "taskFailed",
    upstream_result_unknown: "unknown",
    idc_resource_provider_timeout: "timeout",
    idc_resource_provider_dns: "dns",
    idc_resource_provider_network: "network",
    idc_resource_provider_endpoint_not_found: "endpoint",
    idc_resource_provider_invalid_response: "invalidResponse",
    idc_resource_provider_auth_rejected: "authentication",
    idc_resource_provider_local_auth_policy: "configuration",
  };
  const category =
    categories[(execution?.failureCode || "").toLowerCase()] ||
    (execution?.lastResultCode === "provider_result_unavailable"
      ? "unavailable"
      : "unrecorded");
  return `plugin.idc-core.fulfillment.diagnostic.reason.${category}`;
}

export function fulfillmentPhaseTranslationKey(phase: string) {
  const phases: Record<string, string> = {
    pre_submit_query: "preparation",
    submit_allowed: "submission",
    post_submit_query: "observation",
    frozen_unknown: "reconciliation",
  };
  return `plugin.idc-core.fulfillment.diagnostic.phase.${phases[phase] || "unrecorded"}`;
}
