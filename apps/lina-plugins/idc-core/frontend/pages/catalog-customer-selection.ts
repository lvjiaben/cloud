import type { CatalogCustomerFieldSelection } from "./catalog-client";
import type { CatalogConfigurationDeclaration } from "./catalog-configuration-state";

export class CustomerSelectionError extends Error {
  constructor(
    public readonly reason: "bounds" | "choices" | "step" | "pricing",
    public readonly fieldKey: string,
  ) {
    super(reason);
  }
}

// Discrete specifications define their own sale bounds. Provider limits remain
// authoritative; an old fixed range is never carried into a new choice list.
export function normalizeCustomerNumberSelection(
  field: CatalogConfigurationDeclaration["fields"][number],
  selection: CatalogCustomerFieldSelection,
  preserveBounds = false,
): CatalogCustomerFieldSelection {
  const integer = (
    raw: string | undefined,
    reason: "bounds" | "choices" | "step",
  ) => {
    if (
      !raw ||
      !/^(0|[1-9]\d*)$/u.test(raw) ||
      !Number.isSafeInteger(Number(raw))
    ) {
      throw new CustomerSelectionError(reason, field.key);
    }
    return BigInt(raw);
  };
  const lower = integer(field.minimum, "bounds");
  const upper = integer(field.maximum, "bounds");
  const discrete =
    selection.displayControl === "select" ||
    selection.displayControl === "radio";
  if (selection.numericPricing === "per_choice" && !discrete)
    throw new CustomerSelectionError("pricing", field.key);
  const step = integer(selection.quantityStep || "1", "step");
  if (step < BigInt(1)) throw new CustomerSelectionError("step", field.key);
  let min = BigInt(0),
    max = BigInt(0);
  if (discrete) {
    const choices = selection.numericChoices || [];
    if (
      !choices.length ||
      choices.length > 512 ||
      new Set(choices).size !== choices.length
    ) {
      throw new CustomerSelectionError("choices", field.key);
    }
    const numbers = choices.map((choice) => integer(choice, "choices"));
    min = preserveBounds ? lower : numbers.reduce((a, b) => (a < b ? a : b));
    max = preserveBounds ? upper : numbers.reduce((a, b) => (a > b ? a : b));
    if (numbers.some((n) => n < lower || n > upper))
      throw new CustomerSelectionError("bounds", field.key);
    if (numbers.some((n) => (n - min) % step !== BigInt(0)))
      throw new CustomerSelectionError("step", field.key);
  } else {
    min = integer(selection.minimum, "bounds");
    max = integer(selection.maximum, "bounds");
  }
  if (min < lower || max > upper || min > max)
    throw new CustomerSelectionError("bounds", field.key);
  return {
    ...selection,
    minimum: String(min),
    maximum: String(max),
    quantityStep: String(step),
  };
}
