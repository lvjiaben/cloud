import { describe, expect, it } from "vitest";
import type { CatalogConfigurationField } from "./catalog-configuration-state";
import { normalizeCustomerNumberSelection } from "./catalog-customer-selection";
const field: CatalogConfigurationField = {
  key: "memory",
  type: "integer",
  minimum: "1",
  maximum: "1024",
  label: { zhCN: "内存", enUS: "Memory" },
};
describe("merchant discrete specifications", () => {
  it("replaces stale 1024–1024 bounds with the chosen 2 GiB specification", () => {
    expect(
      normalizeCustomerNumberSelection(field, {
        fieldKey: "memory",
        displayControl: "select",
        minimum: "1024",
        maximum: "1024",
        numericChoices: ["2"],
      }),
    ).toMatchObject({ minimum: "2", maximum: "2", numericChoices: ["2"] });
  });
  it("keeps order but derives bounds from unsorted choices", () => {
    expect(
      normalizeCustomerNumberSelection(field, {
        fieldKey: "memory",
        displayControl: "radio",
        numericChoices: ["8", "2", "4"],
        quantityStep: "2",
      }),
    ).toMatchObject({
      minimum: "2",
      maximum: "8",
      numericChoices: ["8", "2", "4"],
    });
  });
  it.each([["2", "2"], ["0"], ["1025"], ["02"], ["2.5"], ["9007199254740992"]])(
    "rejects invalid values: %s",
    (...numericChoices) => {
      expect(() =>
        normalizeCustomerNumberSelection(field, {
          fieldKey: "memory",
          displayControl: "select",
          numericChoices,
        }),
      ).toThrow();
    },
  );
  it("rejects invalid step and range without silently adjusting either", () => {
    expect(() =>
      normalizeCustomerNumberSelection(field, {
        fieldKey: "memory",
        displayControl: "radio",
        numericChoices: ["2", "5"],
        quantityStep: "2",
      }),
    ).toThrow("step");
    expect(() =>
      normalizeCustomerNumberSelection(field, {
        fieldKey: "memory",
        displayControl: "slider",
        minimum: "8",
        maximum: "2",
      }),
    ).toThrow("bounds");
  });
});

it("keeps required core bounds and uses the provider minimum as the step anchor", () => {
  expect(
    normalizeCustomerNumberSelection(
      field,
      { fieldKey: "memory", displayControl: "select", numericChoices: ["2"] },
      true,
    ),
  ).toMatchObject({ minimum: "1", maximum: "1024" });
  expect(() =>
    normalizeCustomerNumberSelection(
      field,
      {
        fieldKey: "memory",
        displayControl: "select",
        numericChoices: ["2"],
        quantityStep: "2",
      },
      true,
    ),
  ).toThrow("step");
});
