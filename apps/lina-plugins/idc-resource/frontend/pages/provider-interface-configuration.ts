export type InterfaceConfigurationFieldType =
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

export interface InterfaceLocalizedText {
  enUS: string;
  zhCN: string;
}

export interface InterfaceConfigurationCandidate {
  label: InterfaceLocalizedText;
  parentValue?: string;
  value: string;
}

export interface InterfaceConfigurationField {
  description?: InterfaceLocalizedText;
  dynamicCandidates?: { method: string; version: string };
  key: string;
  label: InterfaceLocalizedText;
  maximum?: string;
  maximumLength?: number;
  minimum?: string;
  minimumLength?: number;
  required?: boolean;
  staticCandidates?: InterfaceConfigurationCandidate[];
  textFormat?: "domain" | "email" | "phone" | "url";
  type: InterfaceConfigurationFieldType;
  uploadPolicy?: string;
}

export interface InterfaceConfigurationSchema {
  fields: InterfaceConfigurationField[];
  version: string;
}

export type InterfaceConfigurationValues = Record<string, unknown>;

const supportedTypes = new Set<InterfaceConfigurationFieldType>([
  "address",
  "boolean",
  "date",
  "datetime",
  "decimal",
  "file",
  "integer",
  "multi_line",
  "multi_select",
  "secret",
  "single_line",
  "single_select",
  "tree_select",
]);

export function normalizeInterfaceConfigurationSchema(
  value: unknown,
): InterfaceConfigurationSchema {
  if (!value || typeof value !== "object") return { fields: [], version: "" };
  const candidate = value as Partial<InterfaceConfigurationSchema>;
  const fields = Array.isArray(candidate.fields)
    ? candidate.fields.filter((field): field is InterfaceConfigurationField =>
        Boolean(
          field &&
          typeof field.key === "string" &&
          field.key.length > 0 &&
          supportedTypes.has(field.type),
        ),
      )
    : [];
  return {
    fields,
    version: typeof candidate.version === "string" ? candidate.version : "",
  };
}

export function interfaceFieldLabel(
  field: InterfaceConfigurationField,
  locale: string,
) {
  return locale.toLowerCase().startsWith("zh")
    ? field.label?.zhCN || field.label?.enUS || ""
    : field.label?.enUS || field.label?.zhCN || "";
}

export function interfaceFieldDescription(
  field: InterfaceConfigurationField,
  locale: string,
) {
  return locale.toLowerCase().startsWith("zh")
    ? field.description?.zhCN || field.description?.enUS || ""
    : field.description?.enUS || field.description?.zhCN || "";
}

export function interfaceCandidateLabel(
  candidate: InterfaceConfigurationCandidate,
  locale: string,
) {
  return locale.toLowerCase().startsWith("zh")
    ? candidate.label?.zhCN || candidate.label?.enUS || ""
    : candidate.label?.enUS || candidate.label?.zhCN || "";
}

export function splitInterfaceConfigurationValues(
  schema: InterfaceConfigurationSchema,
  values: InterfaceConfigurationValues,
) {
  const nonSecretSettings: InterfaceConfigurationValues = {};
  const secrets: Record<string, string> = {};
  for (const field of schema.fields) {
    const value = values[field.key];
    if (value === undefined || value === null || value === "") continue;
    if (field.type === "secret") {
      if (typeof value === "string" && value.length > 0)
        secrets[field.key] = value;
      continue;
    }
    if (field.type === "datetime" || field.type === "date") {
      const formattable = value as { format?: (pattern: string) => string };
      nonSecretSettings[field.key] =
        typeof formattable.format === "function"
          ? formattable.format(
              field.type === "date" ? "YYYY-MM-DD" : "YYYY-MM-DDTHH:mm:ssZ",
            )
          : value;
      continue;
    }
    if (field.type === "address" && Array.isArray(value)) {
      nonSecretSettings[field.key] = { path: [...value] };
      continue;
    }
    if (field.type === "file") {
      const tokens = Array.isArray(value)
        ? value.filter((item): item is string => typeof item === "string")
        : [];
      if (tokens.length > 0) nonSecretSettings[field.key] = tokens;
      continue;
    }
    nonSecretSettings[field.key] = value;
  }
  return { nonSecretSettings, secrets };
}

export function requiredSecretKeys(schema: InterfaceConfigurationSchema) {
  return schema.fields
    .filter((field) => field.type === "secret" && field.required)
    .map((field) => field.key);
}

export function secretConfigurationComplete(
  schema: InterfaceConfigurationSchema,
  configured: Record<string, boolean>,
) {
  return requiredSecretKeys(schema).every((key) => configured[key] === true);
}

export function interfaceTreeOptions(
  field: InterfaceConfigurationField,
  locale: string,
) {
  const candidates = field.staticCandidates ?? [];
  const nodes = new Map<
    string,
    { children: Array<unknown>; label: string; value: string }
  >();
  for (const candidate of candidates) {
    nodes.set(candidate.value, {
      children: [],
      label: interfaceCandidateLabel(candidate, locale),
      value: candidate.value,
    });
  }
  const roots: Array<{
    children: Array<unknown>;
    label: string;
    value: string;
  }> = [];
  for (const candidate of candidates) {
    const node = nodes.get(candidate.value)!;
    const parent = candidate.parentValue
      ? nodes.get(candidate.parentValue)
      : undefined;
    if (parent) parent.children.push(node);
    else roots.push(node);
  }
  return roots.map((node) =>
    node.children.length > 0 ? node : { label: node.label, value: node.value },
  );
}
