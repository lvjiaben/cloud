export const SITE_CONTRACT_VERSION = '1.0' as const;
export const SITE_MANIFEST_FILE = 'site-package.json' as const;

export type SiteLocale = 'en-US' | 'zh-CN';
export type SiteRuntimeKind = 'nuxt-node-1';

export interface LocalizedText {
  enUS: string;
  zhCN: string;
}

export interface SitePackageFile {
  path: string;
  sha256: string;
  size: number;
}

export interface SitePackageManifest {
  capabilities: string[];
  compatibility: {
    siteContract: typeof SITE_CONTRACT_VERSION;
    platformApi: string;
  };
  files: SitePackageFile[];
  locales: SiteLocale[];
  name: LocalizedText;
  packageId: string;
  publisher: {
    keyId: string;
    publisherId: string;
    signature: string;
    signatureAlgorithm: 'ed25519';
  };
  routes: {
    public: string[];
  };
  runtime: {
    entry: string;
    healthPath: string;
    kind: SiteRuntimeKind;
    publicAssets: string;
  };
  schemaVersion: 1;
  version: string;
}

export interface SiteApiEnvelope<T> {
  code?: number | string;
  data?: T;
  message?: string;
}

export interface SiteGuestCartReference {
  reference: string;
}

export interface SitePurchaseConfigurationValue {
  addressOptionTokens?: string[];
  booleanValue?: boolean;
  dateTimeValue?: string;
  dateValue?: string;
  decimalValue?: string;
  fieldToken: string;
  fileTokens?: string[];
  integerValue?: string;
  optionTokens?: string[];
  textValue?: string;
  treeOptionTokens?: string[];
  type:
    | 'address'
    | 'boolean'
    | 'date'
    | 'datetime'
    | 'decimal'
    | 'file'
    | 'integer'
    | 'multi_line'
    | 'multi_select'
    | 'single_line'
    | 'single_select'
    | 'tree_select';
}

export interface SiteGuestCartItemInput {
  billingTermToken: string;
  configurationRevision: string;
  currencyCode: string;
  publicSlug: string;
  purchaseCycles: number;
  quantity: number;
  values: SitePurchaseConfigurationValue[];
}

export interface SiteCatalogFilters {
  categoryNo?: string;
  currency?: string;
  cursor?: string;
  limit?: number;
  namePrefix?: string;
  productType?: 'bare_metal' | 'cloud_server';
}

export interface SiteCatalogCategory {
  categoryNo: string;
  nameEnUS: string;
  nameZhCN: string;
  productType: 'bare_metal' | 'cloud_server';
}

export interface SiteCatalogProduct {
  availability: 'available' | 'manual_confirmation' | 'unavailable';
  categoryNameEnUS: string;
  categoryNameZhCN: string;
  categoryNo: string;
  configurationMode: 'configurable' | 'fixed';
  currencies: string[];
  inventoryAsOf?: number;
  nameEnUS: string;
  nameZhCN: string;
  pricingPresentation: 'configuration_required';
  productType: 'bare_metal' | 'cloud_server';
  publicSlug: string;
  summaryEnUS: string;
  summaryZhCN: string;
}

export interface SiteCatalogPage {
  availableCurrencies: string[];
  categories: SiteCatalogCategory[];
  nextCursor: string;
  products: SiteCatalogProduct[];
}

export interface SiteBrandProjection {
  contactEmail?: string;
  contactPhone?: string;
  logoUrl?: string;
  name: LocalizedText;
  revision: string;
  seoDescription: LocalizedText;
  siteUrl: string;
}

export interface SiteBrandConfig {
  revision: string;
  siteName: LocalizedText;
  logoUrl?: string;
  primaryColor: string;
  contactEmail?: string;
  contactPhone?: string;
  navigation: Array<{ label: LocalizedText; href: string }>;
  homepageBlocks: Array<{
    title: LocalizedText;
    description: LocalizedText;
    href?: string;
  }>;
  footer: LocalizedText;
  termsUrl?: string;
  seoTitle: LocalizedText;
  seoDescription: LocalizedText;
}

export function validateSiteBrandConfig(value: unknown): { ok: true; value: SiteBrandConfig } | { ok: false; errors: string[] } {
  const errors: string[] = [];
  if (!value || typeof value !== 'object') return { ok: false, errors: ['brand config must be an object'] };
  const config = value as Partial<SiteBrandConfig>;
  if (!config.revision || !/^[A-Za-z0-9._-]{1,64}$/u.test(config.revision)) errors.push('revision is required');
  if (!config.siteName?.zhCN || !config.siteName?.enUS) errors.push('siteName.zhCN and siteName.enUS are required');
  if (!/^#[0-9a-f]{6}$/iu.test(config.primaryColor ?? '')) errors.push('primaryColor must be a hex color');
  if (!Array.isArray(config.navigation) || config.navigation.length > 20) errors.push('navigation must contain at most 20 items');
  for (const item of config.navigation ?? []) {
    if (!item?.label?.zhCN || !item?.label?.enUS || typeof item.href !== 'string' || !item.href.startsWith('/') || /^\/admin|^\/api|^\/x(?:\/|$)|^\/account|^\/auth|^\/cart|^\/checkout/u.test(item.href)) errors.push(`invalid navigation item: ${String(item?.href)}`);
  }
  if (!Array.isArray(config.homepageBlocks) || config.homepageBlocks.length > 12) errors.push('homepageBlocks must contain at most 12 items');
  for (const block of config.homepageBlocks ?? []) {
    if (!block?.title?.zhCN || !block?.title?.enUS || !block?.description?.zhCN || !block?.description?.enUS || (block.href && (typeof block.href !== 'string' || !block.href.startsWith('/') || /^\/admin|^\/api|^\/x(?:\/|$)|^\/x-assets|^\/account|^\/auth|^\/cart|^\/checkout/u.test(block.href)))) errors.push(`invalid homepage block: ${String(block?.href ?? '')}`);
  }
  if (!config.footer?.zhCN || !config.footer?.enUS) errors.push('footer translations are required');
  if (!config.seoTitle?.zhCN || !config.seoTitle?.enUS || !config.seoDescription?.zhCN || !config.seoDescription?.enUS) errors.push('SEO title and description translations are required');
  const serialized = JSON.stringify(value);
  if (/<script|javascript:|<iframe|onerror\s*=/iu.test(serialized)) errors.push('code snippets are not allowed in brand config');
  if (errors.length > 0) return { ok: false, errors };
  return { ok: true, value: value as SiteBrandConfig };
}

export interface SiteContentProjection {
  bodyHtml: string;
  description: LocalizedText;
  publishedAt: number;
  revision: string;
  slug: string;
  title: LocalizedText;
}

export interface SiteCartHandoff {
  customerUrl: string;
  expiresAt: number;
  reference: string;
}

/**
 * Validate the signed, pre-built website manifest before installation.
 * This is deliberately pure and dependency-free so the admin can run it in
 * the upload path and third-party packers can run the same checks locally.
 */
export function validateSitePackageManifest(
  manifest: unknown,
): { ok: true; value: SitePackageManifest } | { ok: false; errors: string[] } {
  const errors: string[] = [];
  if (!manifest || typeof manifest !== 'object') {
    return { ok: false, errors: ['manifest must be an object'] };
  }
  const value = manifest as Partial<SitePackageManifest>;
  if (value.schemaVersion !== 1) errors.push('schemaVersion must be 1');
  if (value.compatibility?.siteContract !== SITE_CONTRACT_VERSION) {
    errors.push(`compatibility.siteContract must be ${SITE_CONTRACT_VERSION}`);
  }
  if (!value.packageId || !/^[a-z0-9][a-z0-9-]{1,63}$/u.test(value.packageId)) {
    errors.push('packageId must be a safe kebab-case identifier');
  }
  if (!value.version || !/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/u.test(value.version)) {
    errors.push('version must be semver-like');
  }
  if (!Array.isArray(value.locales) || value.locales.length === 0) {
    errors.push('locales must contain at least one locale');
  }
  const routes = value.routes?.public;
  if (!Array.isArray(routes) || routes.length === 0) {
    errors.push('routes.public must contain at least one route');
  } else {
    const forbidden = /^(?:\/admin|\/api|\/x(?:\/|$)|\/x-assets|\/auth|\/account|\/cart|\/checkout)/u;
    for (const route of routes) {
      if (typeof route !== 'string' || !route.startsWith('/') || forbidden.test(route)) {
        errors.push(`route is not allowed: ${String(route)}`);
      }
    }
  }
  if (!value.runtime?.entry || !value.runtime?.healthPath || !value.runtime?.publicAssets) {
    errors.push('runtime entry, healthPath and publicAssets are required');
  }
  if (!Array.isArray(value.files) || value.files.length === 0) {
    errors.push('files must contain the packaged file list');
  } else {
    const seen = new Set<string>();
    for (const file of value.files) {
      const path = file?.path;
      if (typeof path !== 'string' || !path || path.startsWith('/') || path.includes('..') || path.includes('\\')) {
        errors.push(`unsafe file path: ${String(path)}`);
      }
      if (typeof path === 'string' && seen.has(path)) errors.push(`duplicate file path: ${path}`);
      if (typeof path === 'string') seen.add(path);
      if (!/^[a-f0-9]{64}$/u.test(String(file?.sha256))) errors.push(`invalid sha256 for ${String(path)}`);
      if (!Number.isSafeInteger(file?.size) || (file?.size ?? -1) < 0) errors.push(`invalid size for ${String(path)}`);
    }
  }
  if (errors.length > 0) return { ok: false, errors };
  return { ok: true, value: manifest as SitePackageManifest };
}
