export type Locale = "en-US" | "zh-CN";
export type Audience = "admin" | "customer";
export type CertificateAction =
  | "cancel"
  | "download"
  | "revalidate"
  | "sync"
  | "update-validation";

export interface ValidationRecord {
  domain: string;
  fileName: string;
  host: string;
  method: "dns" | "file";
  path: string;
  recordType: string;
  value: string;
}

export interface CertificateItem {
  availableActions: CertificateAction[];
  entitlementNo: string;
  customerName: string;
  expiresAt: number;
  issuedAt: number;
  primaryDomain: string;
  productNameEnUs: string;
  productNameZhCn: string;
  statusCode: string;
  updatedAt: number;
  validation: ValidationRecord[];
  validationMethod: "dns" | "file";
}

export interface CertificatePage {
  items: CertificateItem[];
  nextCursor: string;
}

const allowedActions = new Set<CertificateAction>([
  "cancel",
  "download",
  "revalidate",
  "sync",
  "update-validation",
]);

function safeText(value: unknown, maximum: number) {
  return typeof value === "string" && value.trim().length > 0 && value.length <= maximum
    ? value.trim()
    : "";
}

function safeTime(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) && value >= 0
    ? value
    : 0;
}

function parseValidation(value: unknown): ValidationRecord[] | null {
  if (!Array.isArray(value) || value.length > 100) return null;
  const result: ValidationRecord[] = [];
  for (const raw of value) {
    if (!raw || typeof raw !== "object") return null;
    const item = raw as Record<string, unknown>;
    const method = item.method === "file" ? "file" : item.method === "dns" ? "dns" : null;
    if (!method) return null;
    result.push({
      domain: safeText(item.domain, 255),
      fileName: safeText(item.fileName, 255),
      host: safeText(item.host, 255),
      method,
      path: safeText(item.path, 1024),
      recordType: safeText(item.recordType, 32),
      value: safeText(item.value, 8192),
    });
  }
  return result;
}

export function parseCertificateItem(value: unknown): CertificateItem | null {
  if (!value || typeof value !== "object") return null;
  const raw = value as Record<string, unknown>;
  const entitlementNo = safeText(raw.entitlementNo, 128);
  const primaryDomain = safeText(raw.primaryDomain, 255);
  const statusCode = safeText(raw.statusCode, 64);
  const validationMethod = raw.validationMethod === "file" ? "file" : raw.validationMethod === "dns" ? "dns" : null;
  const validation = parseValidation(raw.validation ?? []);
  const actions = Array.isArray(raw.availableActions)
    ? raw.availableActions.filter((item): item is CertificateAction =>
        typeof item === "string" && allowedActions.has(item as CertificateAction),
      )
    : [];
  if (!entitlementNo || !primaryDomain || !statusCode || !validationMethod || !validation) return null;
  return {
    availableActions: [...new Set(actions)],
    entitlementNo,
    customerName: safeText(raw.customerName, 255),
    expiresAt: safeTime(raw.expiresAt),
    issuedAt: safeTime(raw.issuedAt),
    primaryDomain,
    productNameEnUs: safeText(raw.productNameEnUs, 255),
    productNameZhCn: safeText(raw.productNameZhCn, 255),
    statusCode,
    updatedAt: safeTime(raw.updatedAt),
    validation,
    validationMethod,
  };
}

export function parseCertificatePage(value: unknown): CertificatePage | null {
  if (!value || typeof value !== "object") return null;
  const raw = value as { items?: unknown; nextCursor?: unknown };
  if (!Array.isArray(raw.items) || raw.items.length > 100) return null;
  const items = raw.items.map(parseCertificateItem);
  if (items.some((item) => !item)) return null;
  const nextCursor = typeof raw.nextCursor === "string" && raw.nextCursor.length <= 256 ? raw.nextCursor : "";
  return { items: items as CertificateItem[], nextCursor };
}

export function parseCertificateDetail(value: unknown): CertificateItem | null {
  return value && typeof value === "object"
    ? parseCertificateItem((value as { item?: unknown }).item)
    : null;
}

export function normalizeLocale(value: unknown): Locale {
  return value === "en-US" ? "en-US" : "zh-CN";
}

export function displayProduct(item: CertificateItem, locale: Locale) {
  return (locale === "en-US" ? item.productNameEnUs : item.productNameZhCn) || item.primaryDomain;
}

export function formatTime(value: number, locale: Locale) {
  if (!value) return "—";
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}
