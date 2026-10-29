// Local preview uses the existing declarative condition evaluator. It neither
// attests supplier values nor submits a quote, order or automation request.
import type {
  CatalogConfigurationDeclaration,
  CatalogConfigurationField,
  CatalogConfigurationValues,
  CatalogConfigurationRule,
  CatalogConfigurationCondition,
} from "./catalog-configuration-state";
import type { CatalogDirectoryRelationship } from "./catalog-client";
import {
  catalogConfigurationCandidates,
  catalogConfigurationFieldState,
} from "./catalog-configuration-state";

export type OfferPreviewField = CatalogConfigurationField & {
  customerVisible?: boolean;
  defaultValue?: unknown;
  productFieldKey?: string;
  productChoiceBindings?: Record<string, string>;
  displayControl?: string;
  quantityStep?: string;
  unit?: string;
  group?: string;
  disabled?: boolean;
};

function ruleIdentity(rule: CatalogConfigurationRule) {
  const condition = (node: CatalogConfigurationCondition): unknown[] => [
    node.operator,
    node.field || "",
    node.value || "",
    (node.children || []).map(condition),
  ];
  return JSON.stringify([
    condition(rule.when),
    rule.actions.map((action) => [
      action.action,
      action.field,
      action.values || [],
    ]),
  ]);
}

// Rebuild only owner-derived filters whose parameter mappings are present in
// this draft. Provider rules and unrelated saved commercial rules remain. The
// generated rules use the same evaluator as stored forms; no request is sent.
export function previewRelationshipConditions(
  fields: OfferPreviewField[],
  conditions: CatalogConfigurationRule[] = [],
  savedRules: Record<string, CatalogConfigurationRule[]> = {},
  relationships: CatalogDirectoryRelationship[] = [],
): CatalogConfigurationRule[] {
  if (!relationships.length) return conditions;
  const byParameter = new Map<string, OfferPreviewField | null>();
  for (const field of fields) {
    if (!field.productFieldKey) continue;
    byParameter.set(
      field.productFieldKey,
      byParameter.has(field.productFieldKey) ? null : field,
    );
  }
  const rebuilt = new Set<string>();
  const generated: CatalogConfigurationRule[] = [];
  for (const relation of relationships.slice(0, 16)) {
    const parent = byParameter.get(relation.parentFieldKey);
    const child = byParameter.get(relation.childFieldKey);
    if (
      !parent ||
      !child ||
      parent.type !== "single_select" ||
      child.type !== "single_select"
    )
      continue;
    rebuilt.add(child.key);
    const allowed = new Map<string, Set<string>>();
    for (const link of relation.links) {
      const children = allowed.get(link.parentValue) || new Set<string>();
      children.add(link.childValue);
      allowed.set(link.parentValue, children);
    }
    for (const choice of parent.staticCandidates || [])
      generated.push({
        when: { operator: "equal", field: parent.key, value: choice.value },
        actions: [
          {
            action: "filter_candidates",
            field: child.key,
            values: (child.staticCandidates || [])
              .filter((value) =>
                allowed
                  .get(
                    parent.productChoiceBindings?.[choice.value] ??
                      choice.value,
                  )
                  ?.has(
                    child.productChoiceBindings?.[value.value] ?? value.value,
                  ),
              )
              .map((value) => value.value),
          },
        ],
      });
  }
  const removable = new Map<string, number>();
  for (const key of rebuilt)
    for (const rule of savedRules[key] || []) {
      const identity = ruleIdentity(rule);
      removable.set(identity, (removable.get(identity) || 0) + 1);
    }
  const retained = conditions.filter((rule) => {
    const identity = ruleIdentity(rule),
      remaining = removable.get(identity) || 0;
    if (!remaining) return true;
    removable.set(identity, remaining - 1);
    return false;
  });
  return [...retained, ...generated];
}

function conditionValues(
  values: Record<string, unknown>,
): CatalogConfigurationValues {
  return Object.fromEntries(
    Object.entries(values).map(([key, value]) => {
      if (typeof value === "number" && Number.isFinite(value))
        return [key, String(value)];
      if (typeof value === "string" || typeof value === "boolean")
        return [key, value];
      if (Array.isArray(value))
        return [key, value.filter((item) => typeof item === "string")];
      return [key, undefined];
    }),
  );
}

function normalizeValue(
  field: OfferPreviewField,
  value: unknown,
  discrete: boolean,
) {
  const choices = field.staticCandidates || [];
  if (
    field.type === "single_select" ||
    (field.type === "integer" && discrete)
  ) {
    return choices.some((choice) => choice.value === String(value))
      ? String(value)
      : choices[0]?.value;
  }
  if (field.type === "multi_select") {
    return Array.isArray(value)
      ? value.filter((item) => choices.some((choice) => choice.value === item))
      : [];
  }
  if (field.type === "integer") {
    const minimum = Number(field.minimum || "0");
    const maximum = Number(field.maximum || Number.MAX_SAFE_INTEGER);
    const step = Number(field.quantityStep || "1");
    const number = Number(value);
    return value !== undefined &&
      value !== null &&
      value !== "" &&
      Number.isSafeInteger(number) &&
      number >= minimum &&
      number <= maximum &&
      step > 0 &&
      (number - minimum) % step === 0
      ? number
      : minimum;
  }
  if (field.type === "boolean")
    return typeof value === "boolean" ? value : false;
  return value;
}

// Re-evaluate to a bounded fixed point so field ordering never determines which
// valid child survives. Only invalid choices reset; merchant input is immutable.
export function resolveOfferPreview(
  declaration: CatalogConfigurationDeclaration,
  supplied: Record<string, unknown>,
) {
  const fields = (declaration.fields as OfferPreviewField[]).filter(
    (field) => field.type !== "secret",
  );
  let values: Record<string, unknown> = Object.fromEntries(
    fields.map((field) => [field.key, supplied[field.key]]),
  );
  let visible: OfferPreviewField[] = [];
  for (let pass = 0; pass <= Math.min(fields.length, 128); pass++) {
    const next: Record<string, unknown> = {};
    const conditions = conditionValues(values);
    visible = [];
    for (const field of fields) {
      const state = catalogConfigurationFieldState(
        declaration,
        field,
        conditions,
      );
      if (!state.visible) continue;
      const projected = {
        ...field,
        required: state.required,
        disabled: state.disabled,
        staticCandidates: catalogConfigurationCandidates(
          declaration,
          field,
          conditions,
        ),
      };
      if (field.customerVisible === false) { next[field.key] = field.defaultValue; continue; }
      visible.push(projected);
      if (!state.disabled && field.type !== "file")
        next[field.key] = normalizeValue(
          projected,
          values[field.key],
          !!field.staticCandidates?.length ||
            ["radio", "select"].includes(field.displayControl || ""),
        );
    }
    if (JSON.stringify(values) === JSON.stringify(next))
      return {
        fields: visible,
        values: next,
        validationValues: conditionValues(next),
      };
    values = next;
  }
  return { fields: visible, values, validationValues: conditionValues(values) };
}
