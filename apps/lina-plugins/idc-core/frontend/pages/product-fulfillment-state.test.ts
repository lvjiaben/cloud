import { describe, expect, it } from "vitest";

import {
  createFulfillmentActionAttempt,
  fulfillmentFailureTranslationKey,
  fulfillmentPhaseTranslationKey,
  fulfillmentEventTranslationKey,
  fulfillmentEventReasonLabel,
  fulfillmentModeTranslationKey,
  fulfillmentQuantityText,
  fulfillmentStatusColor,
  fulfillmentStatusOptions,
  fulfillmentStatusTranslationKey,
  resetFulfillmentPaging,
  requiresFulfillmentActionNote,
  settleNextFulfillmentPage,
  settlePreviousFulfillmentPage,
} from "./product-fulfillment-state";

describe("product fulfillment presentation", () => {
  it.each([
    "awaiting_payment",
    "awaiting_review",
    "processing",
    "manual_processing",
    "completed",
    "not_completed",
    "cancelled",
  ])("maps customer-safe status %s to localized text", (status) => {
    expect(fulfillmentStatusTranslationKey(status)).toBe(
      `plugin.idc-core.fulfillment.status.${status}`,
    );
  });

  it("does not offer the pre-settlement customer state as a task filter", () => {
    const values = fulfillmentStatusOptions((key) => key).map(({ value }) => value);
    expect(values).not.toContain("awaiting_payment");
    expect(values).toEqual([
      "awaiting_review",
      "processing",
    "manual_processing",
      "completed",
      "not_completed",
      "cancelled",
    ]);
  });

  it("shows generic quantity according to frozen granularity", () => {
    const t = (key: string, values?: Record<string, number>) =>
      `${key}:${JSON.stringify(values ?? {})}`;
    expect(fulfillmentQuantityText("per_order_item", 1, 4, t)).toContain(
      '"quantity":4',
    );
    expect(fulfillmentQuantityText("per_quantity_unit", 2, 4, t)).toContain(
      '"current":2,"total":4',
    );
    expect(fulfillmentQuantityText("server", 1, 4, t)).toContain(
      "common.unavailable",
    );
  });

  it("never exposes an unknown internal status", () => {
    expect(fulfillmentStatusTranslationKey("retry_wait")).toBe(
      "plugin.idc-core.fulfillment.common.unavailable",
    );
    expect(fulfillmentStatusColor("provider_secret_error")).toBe("default");
    expect(fulfillmentModeTranslationKey("server_only")).toBe(
      "plugin.idc-core.fulfillment.common.unavailable",
    );
  });

  it.each([
    "review_approve",
    "review_reject",
    "takeover",
    "manual_completed",
    "manual_unable",
  ])("reuses one action key when a %s response is lost", () => {
    let generated = 0;
    const attempt = createFulfillmentActionAttempt(
      () => `stable-${++generated}`,
    );
    const firstBody = { ...attempt, expectedVersion: 3 };
    const retryBody = { ...attempt, expectedVersion: 3 };
    expect(retryBody).toEqual(firstBody);
    expect(generated).toBe(1);
  });

  it.each(["search", "reset"])(
    "%s starts again from the first authoritative page",
    () => {
      expect(resetFulfillmentPaging()).toEqual({
        cursor: "",
        history: [],
        nextCursor: "",
      });
    },
  );

  it("commits forward and backward history only after a successful request", () => {
    const original = ["page-0"];
    expect(settleNextFulfillmentPage("page-1", original, true)).toEqual([
      "page-0",
      "page-1",
    ]);
    expect(original).toEqual(["page-0"]);
    expect(
      settlePreviousFulfillmentPage(["page-0", "page-1"], true),
    ).toEqual(["page-0"]);
  });

  it("keeps failed paging requests from mutating caller history", () => {
    const history = ["page-0", "page-1"];
    expect(settleNextFulfillmentPage("page-2", history, false)).toEqual(
      history,
    );
    expect(settlePreviousFulfillmentPage(history, false)).toEqual(history);
    expect(history).toEqual(["page-0", "page-1"]);
  });

  it("maps history events through a safe whitelist", () => {
    expect(fulfillmentEventTranslationKey("review_approved")).toBe(
      "plugin.idc-core.fulfillment.event.review_approved",
    );
    expect(fulfillmentEventTranslationKey("provider_secret_failure")).toBe(
      "plugin.idc-core.fulfillment.common.unavailable",
    );
    expect(fulfillmentEventTranslationKey("moved_to_manual")).toBe(
      "plugin.idc-core.fulfillment.event.moved_to_manual",
    );
  });

  it("requires a note only for manual conclusions", () => {
    expect(requiresFulfillmentActionNote("manual_completed")).toBe(true);
    expect(requiresFulfillmentActionNote("manual_unable")).toBe(true);
    expect(requiresFulfillmentActionNote("review_approve")).toBe(false);
    expect(requiresFulfillmentActionNote("review_reject")).toBe(false);
    expect(requiresFulfillmentActionNote("takeover")).toBe(false);
  });

  it("uses frozen history reason labels and fails closed when absent", () => {
    const event = {
      reasonCode: "verified",
      reasonLabelEnUS: "Verified at the time",
      reasonLabelZhCN: "当时已核对",
    };
    expect(fulfillmentEventReasonLabel(event, false, "不可用")).toBe("当时已核对");
    expect(fulfillmentEventReasonLabel(event, true, "Unavailable")).toBe("Verified at the time");
    expect(
      fulfillmentEventReasonLabel({ reasonCode: "retired" }, false, "不可用"),
    ).toBe("不可用");
    expect(fulfillmentEventReasonLabel({}, false, "不可用")).toBe("");
  });
});

describe("operator provisioning diagnostics", () => {
 it.each([
  ["request_invalid","localRequest"], ["lookup_failed","localLookup"], ["parameters_unavailable","localParameters"],
  ["credential_failed","localCredential"], ["callback_unavailable","localCallback"], ["task_input_invalid","localTaskInput"], ["task_storage_failed","localTaskStorage"],
 ])("local preparation %s has a reviewed operator reason",(code,label)=>{
  expect(fulfillmentFailureTranslationKey({statusCode:"failed",failureCode:"resource_preparation_"+code})).toBe("plugin.idc-core.fulfillment.diagnostic.reason."+label);
 });
 it("maps only reviewed causes and never renders arbitrary supplier text", () => {
  expect(fulfillmentFailureTranslationKey({statusCode: "failed", failureCode: "provider_request_not_sent"})).toBe("plugin.idc-core.fulfillment.diagnostic.reason.notSent");
  expect(fulfillmentFailureTranslationKey({statusCode: "result_unknown", failureCode: "IDC_RESOURCE_PROVIDER_TIMEOUT"})).toBe("plugin.idc-core.fulfillment.diagnostic.reason.timeout");
  expect(fulfillmentFailureTranslationKey({statusCode: "failed", failureCode: "private_secret_canary"})).toBe("plugin.idc-core.fulfillment.diagnostic.reason.unrecorded");
  expect(fulfillmentPhaseTranslationKey("frozen_unknown")).toBe("plugin.idc-core.fulfillment.diagnostic.phase.reconciliation");
 });
});
