export interface PurchaseValueDraft {
  fieldToken: string;
  fieldType: string;
  fieldValue: string;
}

export const purchaseValueTypes = [
  "single_line",
  "multi_line",
  "integer",
  "decimal",
  "boolean",
  "single_select",
  "multi_select",
  "date",
  "datetime",
  "tree_select",
  "address",
  "file",
] as const;

// Preserve the established channel publication contract for every field type.
export function serializePurchaseValues(values: PurchaseValueDraft[]) {
  return values.map(({ fieldToken, fieldType: type, fieldValue }) => {
    const token = fieldToken.trim();
    const raw = fieldValue.trim();
    if (!token) throw new Error("field token required");
    const result: Record<string, unknown> = { fieldToken: token, type };
    if (type === "single_line" || type === "multi_line") result.textValue = raw;
    else if (type === "integer") result.integerValue = raw;
    else if (type === "decimal") result.decimalValue = raw;
    else if (type === "boolean") {
      if (raw !== "true" && raw !== "false") throw new Error("invalid boolean");
      result.booleanValue = raw === "true";
    } else if (type === "date") result.dateValue = raw;
    else if (type === "datetime") result.dateTimeValue = raw;
    else {
      const tokens = raw
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean);
      if (type === "single_select" || type === "multi_select")
        result.optionTokens = tokens;
      if (type === "tree_select") result.treeOptionTokens = tokens;
      if (type === "address") result.addressOptionTokens = tokens;
      if (type === "file") result.fileTokens = tokens;
    }
    return result;
  });
}
