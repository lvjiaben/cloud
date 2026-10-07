import assert from "node:assert/strict";
import test from "node:test";
import {
  purchaseValueTypes,
  serializePurchaseValues,
} from "./channel-purchase-values.ts";

test("publication drafts preserve all twelve established field contracts", () => {
  const input = purchaseValueTypes.map((fieldType) => ({
    fieldToken: " cpu ",
    fieldType,
    fieldValue: fieldType === "boolean" ? "false" : " a, b ",
  }));
  const expected = [
    { textValue: "a, b" },
    { textValue: "a, b" },
    { integerValue: "a, b" },
    { decimalValue: "a, b" },
    { booleanValue: false },
    { optionTokens: ["a", "b"] },
    { optionTokens: ["a", "b"] },
    { dateValue: "a, b" },
    { dateTimeValue: "a, b" },
    { treeOptionTokens: ["a", "b"] },
    { addressOptionTokens: ["a", "b"] },
    { fileTokens: ["a", "b"] },
  ];
  assert.deepEqual(
    serializePurchaseValues(input),
    expected.map((value, index) => ({
      fieldToken: "cpu",
      type: purchaseValueTypes[index],
      ...value,
    })),
  );
});

test("publication drafts reject missing field tokens and nonboolean values", () => {
  assert.throws(
    () =>
      serializePurchaseValues([
        { fieldToken: " ", fieldType: "integer", fieldValue: "1" },
      ]),
    /field token required/,
  );
  assert.throws(
    () =>
      serializePurchaseValues([
        { fieldToken: "cpu", fieldType: "boolean", fieldValue: "yes" },
      ]),
    /invalid boolean/,
  );
  assert.deepEqual(
    serializePurchaseValues([
      { fieldToken: "flag", fieldType: "boolean", fieldValue: "true" },
    ]),
    [{ fieldToken: "flag", type: "boolean", booleanValue: true }],
  );
});
