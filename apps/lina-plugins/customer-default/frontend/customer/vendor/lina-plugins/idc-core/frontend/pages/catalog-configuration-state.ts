export type CatalogConfigurationSurface = "customer" | "interface" | "product";

export type CatalogConfigurationFieldType =
  | "address"
  | "boolean"
  | "date"
  | "datetime"
  | "decimal"
  | "file"
  | "integer"
  | "multi_line"
  | "multi_select"
  | "secret"
  | "single_line"
  | "single_select"
  | "tree_select";

export type CatalogConfigurationConditionOperator =
  | "and"
  | "contains"
  | "empty"
  | "equal"
  | "greater_or_equal"
  | "greater_than"
  | "less_or_equal"
  | "less_than"
  | "not_empty"
  | "not_equal"
  | "or";

export type CatalogConfigurationActionType =
  | "disable"
  | "excludes"
  | "filter_candidates"
  | "hide"
  | "require"
  | "requires"
  | "show";

export interface CatalogLocalizedText {
  enUS: string;
  zhCN: string;
}

export interface CatalogConfigurationCandidate {
  label: CatalogLocalizedText;
  parentValue?: string;
  value: string;
}

export interface CatalogDynamicCandidateDeclaration {
  method: string;
  version: string;
}

export interface CatalogConfigurationCondition {
  children?: CatalogConfigurationCondition[];
  field?: string;
  operator: CatalogConfigurationConditionOperator;
  value?: string;
}

export interface CatalogConfigurationAction {
  action: CatalogConfigurationActionType;
  field: string;
  values?: string[];
}

export interface CatalogConfigurationRule {
  actions: CatalogConfigurationAction[];
  when: CatalogConfigurationCondition;
}

export interface CatalogConfigurationField {
  description?: CatalogLocalizedText;
  dynamicCandidates?: CatalogDynamicCandidateDeclaration;
  key: string;
  label: CatalogLocalizedText;
  maximum?: string;
  maximumLength?: number;
  minimum?: string;
  minimumLength?: number;
  required?: boolean;
  staticCandidates?: CatalogConfigurationCandidate[];
  textFormat?: "domain" | "email" | "phone" | "url";
  type: CatalogConfigurationFieldType;
  uploadPolicy?: string;
}

export interface CatalogConfigurationDeclaration {
  conditions?: CatalogConfigurationRule[];
  fields: CatalogConfigurationField[];
  uploadPolicies?: CatalogConfigurationUploadPolicy[];
  version: string;
}

export interface CatalogConfigurationUploadPolicy {
  allowedExtensions: string[];
  allowedMimeTypes: string[];
  id: string;
  maximumFileBytes: number;
  maximumFiles: number;
  maximumTotalBytes: number;
  safetyScanRequired: boolean;
}

export interface CatalogStructuredConfigurationValue {
  addressOptionTokens?: string[];
  booleanValue?: boolean;
  dateTimeValue?: string;
  dateValue?: string;
  decimalValue?: string;
  fieldKey: string;
  fileTokens?: string[];
  integerValue?: string;
  optionTokens?: string[];
  textValue?: string;
  treeOptionTokens?: string[];
}

export type CatalogConfigurationDraftValue = boolean | string | string[];
export type CatalogConfigurationValues = Record<
  string,
  CatalogConfigurationDraftValue | undefined
>;

export interface CatalogConfigurationFieldState {
  candidateValues?: Set<string>;
  disabled: boolean;
  excludes: Set<string>;
  required: boolean;
  requires: Set<string>;
  visible: boolean;
}

export interface CatalogConfigurationAuthoritativeFieldState {
  disabled: boolean;
  required: boolean;
  visible: boolean;
}

export interface CatalogConfigurationIssue {
  fieldKey?: string;
  issueCode: string;
}

export interface CatalogConfigurationEvaluation {
  candidates: Record<string, CatalogConfigurationCandidate[]>;
  fieldStates: Record<string, CatalogConfigurationAuthoritativeFieldState>;
  issues: CatalogConfigurationIssue[];
  valid: boolean;
  values: CatalogStructuredConfigurationValue[];
}

const MAX_CONDITION_DEPTH = 4;
const MAX_CONDITION_NODES = 64;
const INTEGER_PATTERN = /^-?(?:0|[1-9]\d*)$/;
const DECIMAL_PATTERN = /^-?(?:0|[1-9]\d*)(?:\.\d+)?$/;

function isEmpty(value: unknown) {
  return (
    value === undefined ||
    value === null ||
    value === "" ||
    (Array.isArray(value) && value.length === 0)
  );
}

function scalarValue(value: unknown) {
  if (typeof value === "boolean") return value ? "true" : "false";
  if (typeof value === "string") return value;
  return "";
}

function decimalParts(value: string) {
  if (!DECIMAL_PATTERN.test(value)) return undefined;
  const negative = value.startsWith("-");
  const unsigned = negative ? value.slice(1) : value;
  const [integer = "0", fraction = ""] = unsigned.split(".");
  return {
    fraction: fraction.replace(/0+$/, ""),
    integer: integer.replace(/^0+(?=\d)/, ""),
    negative: negative && !/^0(?:\.0*)?$/.test(unsigned),
  };
}

function compareDecimal(left: string, right: string) {
  const first = decimalParts(left);
  const second = decimalParts(right);
  if (!first || !second) return undefined;
  if (first.negative !== second.negative) return first.negative ? -1 : 1;
  const direction = first.negative ? -1 : 1;
  if (first.integer.length !== second.integer.length)
    return first.integer.length > second.integer.length
      ? direction
      : -direction;
  if (first.integer !== second.integer)
    return first.integer > second.integer ? direction : -direction;
  const length = Math.max(first.fraction.length, second.fraction.length);
  const firstFraction = first.fraction.padEnd(length, "0");
  const secondFraction = second.fraction.padEnd(length, "0");
  if (firstFraction === secondFraction) return 0;
  return firstFraction > secondFraction ? direction : -direction;
}

function validDomain(value: string) {
  if (
    !value ||
    value.length > 253 ||
    value.startsWith(".") ||
    value.endsWith(".")
  )
    return false;
  return value.split(".").every((label) => {
    if (
      !label ||
      label.length > 63 ||
      label.startsWith("-") ||
      label.endsWith("-")
    )
      return false;
    return [...label].every((character) => /[\p{L}\p{N}-]/u.test(character));
  });
}

function validTextFormat(field: CatalogConfigurationField, value: string) {
  if (field.type === "single_line" && /[\r\n]/.test(value)) return false;
  if (!field.textFormat) return true;
  if (field.textFormat === "phone")
    return value.length >= 7 && value.length <= 16 && /^\+?\d+$/.test(value);
  if (field.textFormat === "email")
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
  if (field.textFormat === "domain") return validDomain(value);
  try {
    const parsed = new URL(value);
    return (
      ["http:", "https:"].includes(parsed.protocol) && Boolean(parsed.host)
    );
  } catch {
    return false;
  }
}

function conditionWithinBounds(
  condition: CatalogConfigurationCondition,
  depth = 0,
  counter = { value: 0 },
): boolean {
  if (depth > MAX_CONDITION_DEPTH) return false;
  counter.value += 1;
  if (counter.value > MAX_CONDITION_NODES) return false;
  return (condition.children ?? []).every((child) =>
    conditionWithinBounds(child, depth + 1, counter),
  );
}

export function evaluateCatalogConfigurationCondition(
  condition: CatalogConfigurationCondition | undefined,
  values: CatalogConfigurationValues,
  depth = 0,
  counter = { value: 0 },
): boolean {
  if (!condition || depth > MAX_CONDITION_DEPTH) return false;
  if (depth === 0 && !conditionWithinBounds(condition)) return false;
  counter.value += 1;
  if (counter.value > MAX_CONDITION_NODES) return false;
  if (condition.operator === "and" || condition.operator === "or") {
    const children = condition.children ?? [];
    if (children.length < 2) return false;
    const evaluate = (item: CatalogConfigurationCondition) =>
      evaluateCatalogConfigurationCondition(item, values, depth + 1, counter);
    return condition.operator === "and"
      ? children.every(evaluate)
      : children.some(evaluate);
  }
  if (!condition.field) return false;
  const current = values[condition.field];
  switch (condition.operator) {
    case "contains":
      return Array.isArray(current)
        ? current.includes(condition.value ?? "")
        : scalarValue(current).includes(condition.value ?? "");
    case "empty":
      return isEmpty(current);
    case "equal":
      return Array.isArray(current)
        ? current.includes(condition.value ?? "")
        : scalarValue(current) === (condition.value ?? "");
    case "greater_or_equal":
    case "greater_than":
    case "less_or_equal":
    case "less_than": {
      const compared = compareDecimal(
        scalarValue(current),
        condition.value ?? "",
      );
      if (compared === undefined) return false;
      if (condition.operator === "greater_or_equal") return compared >= 0;
      if (condition.operator === "greater_than") return compared > 0;
      if (condition.operator === "less_or_equal") return compared <= 0;
      return compared < 0;
    }
    case "not_empty":
      return !isEmpty(current);
    case "not_equal":
      return Array.isArray(current)
        ? !current.includes(condition.value ?? "")
        : scalarValue(current) !== (condition.value ?? "");
    default:
      return false;
  }
}

export function catalogConfigurationFieldState(
  declaration: CatalogConfigurationDeclaration,
  field: CatalogConfigurationField,
  values: CatalogConfigurationValues,
): CatalogConfigurationFieldState {
  const rules = declaration.conditions ?? [];
  const hasShowRule = rules.some((rule) =>
    rule.actions.some(
      (action) => action.field === field.key && action.action === "show",
    ),
  );
  const state: CatalogConfigurationFieldState = {
    disabled: false,
    excludes: new Set(),
    required: field.required === true,
    requires: new Set(),
    visible: !hasShowRule,
  };
  for (const rule of rules) {
    if (!evaluateCatalogConfigurationCondition(rule.when, values)) continue;
    for (const action of rule.actions) {
      if (action.field !== field.key) continue;
      if (action.action === "show") state.visible = true;
      if (action.action === "hide") state.visible = false;
      if (action.action === "require") state.required = true;
      if (action.action === "disable") state.disabled = true;
      if (action.action === "filter_candidates") {
        const candidates = new Set(action.values ?? []);
        state.candidateValues = state.candidateValues
          ? new Set(
              [...state.candidateValues].filter((value) =>
                candidates.has(value),
              ),
            )
          : candidates;
      }
      if (action.action === "requires") {
        for (const value of action.values ?? []) state.requires.add(value);
      }
      if (action.action === "excludes") {
        for (const value of action.values ?? []) state.excludes.add(value);
      }
    }
  }
  return state;
}

export function catalogConfigurationCandidates(
  declaration: CatalogConfigurationDeclaration,
  field: CatalogConfigurationField,
  values: CatalogConfigurationValues,
) {
  const state = catalogConfigurationFieldState(declaration, field, values);
  return (field.staticCandidates ?? []).filter(
    (candidate) =>
      !state.candidateValues || state.candidateValues.has(candidate.value),
  );
}

export function catalogConfigurationPresentation(
  declaration: CatalogConfigurationDeclaration,
) {
  const complexTypes = new Set<CatalogConfigurationFieldType>([
    "address",
    "file",
    "multi_line",
    "tree_select",
  ]);
  return declaration.fields.length > 6 ||
    declaration.fields.some((field) => complexTypes.has(field.type)) ||
    Boolean(declaration.conditions?.length)
    ? "drawer"
    : "modal";
}

export function catalogConfigurationFileTokensWithout(
  value: unknown,
  token: string,
) {
  return Array.isArray(value)
    ? value.filter(
        (item): item is string => typeof item === "string" && item !== token,
      )
    : [];
}

function candidateValues(field: CatalogConfigurationField) {
  return new Set((field.staticCandidates ?? []).map((item) => item.value));
}

export function sanitizeCatalogConfigurationValues(
  declaration: CatalogConfigurationDeclaration,
  surface: CatalogConfigurationSurface,
  values: CatalogConfigurationValues,
) {
  const result: CatalogConfigurationValues = {};
  for (const field of declaration.fields) {
    if (field.type === "secret" && surface !== "interface") continue;
    const state = catalogConfigurationFieldState(declaration, field, values);
    if (!state.visible || state.disabled) continue;
    const value = values[field.key];
    if (value === undefined || (field.type === "secret" && isEmpty(value)))
      continue;
    const allowed = candidateValues(field);
    if (["single_select", "tree_select"].includes(field.type)) {
      if (field.dynamicCandidates || allowed.has(String(value)))
        result[field.key] = value;
      continue;
    }
    if (["address", "multi_select"].includes(field.type)) {
      if (!Array.isArray(value)) continue;
      result[field.key] = field.dynamicCandidates
        ? [...value]
        : value.filter((token) => allowed.has(token));
      continue;
    }
    result[field.key] = value;
  }
  return result;
}

export function catalogConfigurationRowsToValues(
  rows: CatalogStructuredConfigurationValue[] | undefined,
  declaration?: CatalogConfigurationDeclaration,
) {
  const result: CatalogConfigurationValues = {};
  const fields = new Map(
    (declaration?.fields ?? []).map((field) => [field.key, field]),
  );
  for (const row of rows ?? []) {
    const arms = [
      row.textValue,
      row.integerValue,
      row.decimalValue,
      row.booleanValue,
      row.dateValue,
      row.dateTimeValue,
      row.optionTokens,
      row.treeOptionTokens,
      row.addressOptionTokens,
      row.fileTokens,
    ].filter((value) => value !== undefined);
    if (arms.length !== 1) continue;
    let value = arms[0] as CatalogConfigurationDraftValue;
    const field = fields.get(row.fieldKey);
    if (
      field &&
      ["single_select", "tree_select"].includes(field.type) &&
      Array.isArray(value)
    ) {
      if (value.length !== 1) continue;
      value = value[0] as string;
    }
    result[row.fieldKey] = value;
  }
  return result;
}

export function catalogConfigurationValuesToRows(
  declaration: CatalogConfigurationDeclaration,
  surface: CatalogConfigurationSurface,
  values: CatalogConfigurationValues,
) {
  const sanitized = sanitizeCatalogConfigurationValues(
    declaration,
    surface,
    values,
  );
  const rows: CatalogStructuredConfigurationValue[] = [];
  for (const field of declaration.fields) {
    const value = sanitized[field.key];
    if (value === undefined) continue;
    const row: CatalogStructuredConfigurationValue = { fieldKey: field.key };
    if (["multi_line", "secret", "single_line"].includes(field.type)) {
      row.textValue = String(value);
    } else if (field.type === "integer") {
      // Keep the exact decimal string for authoritative whole-form
      // evaluation. Local validation gives immediate feedback, while the
      // server remains the final authority for range and canonical form.
      row.integerValue = String(value);
    } else if (field.type === "decimal") {
      row.decimalValue = String(value);
    } else if (field.type === "boolean") {
      row.booleanValue = value === true;
    } else if (field.type === "date") {
      row.dateValue = String(value);
    } else if (field.type === "datetime") {
      row.dateTimeValue = String(value);
    } else if (["single_select", "multi_select"].includes(field.type)) {
      row.optionTokens = Array.isArray(value) ? value : [String(value)];
    } else if (field.type === "tree_select") {
      row.treeOptionTokens = Array.isArray(value) ? value : [String(value)];
    } else if (field.type === "address") {
      row.addressOptionTokens = Array.isArray(value) ? value : [String(value)];
    } else if (field.type === "file") {
      row.fileTokens = Array.isArray(value) ? value : [String(value)];
    }
    rows.push(row);
  }
  return rows;
}

export function validateCatalogConfigurationDraftValue(
  declaration: CatalogConfigurationDeclaration,
  field: CatalogConfigurationField,
  values: CatalogConfigurationValues,
) {
  const state = catalogConfigurationFieldState(declaration, field, values);
  if (!state.visible || state.disabled) return "";
  const value = values[field.key];
  if (state.required && isEmpty(value)) return "required";
  if (isEmpty(value)) return "";
  const text = scalarValue(value);
  if (field.type === "integer") {
    if (!INTEGER_PATTERN.test(text)) return "integer";
    try {
      const numeric = BigInt(text);
      if (
        numeric < -9_223_372_036_854_775_808n ||
        numeric > 9_223_372_036_854_775_807n
      )
        return "integer";
    } catch {
      return "integer";
    }
  }
  if (field.type === "decimal" && !DECIMAL_PATTERN.test(text)) return "decimal";
  if (field.minimum && compareDecimal(text, field.minimum) === -1)
    return "minimum";
  if (field.maximum && compareDecimal(text, field.maximum) === 1)
    return "maximum";
  const textLength = [...text].length;
  if (field.minimumLength && textLength < field.minimumLength)
    return "minimumLength";
  if (field.maximumLength && textLength > field.maximumLength)
    return "maximumLength";
  if (
    ["single_line", "multi_line"].includes(field.type) &&
    !validTextFormat(field, text)
  )
    return "invalid";
  const selected = new Set(Array.isArray(value) ? value : [text]);
  if ([...state.requires].some((item) => !selected.has(item)))
    return "requires";
  if ([...state.excludes].some((item) => selected.has(item))) return "excludes";
  return "";
}
