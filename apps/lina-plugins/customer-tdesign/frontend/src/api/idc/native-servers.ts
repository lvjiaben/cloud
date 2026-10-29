// Resource-owned native server management; no supplier identifiers or secrets are persisted here.
import { pluginApiPath, requestClient } from "#/api/request";

export interface NativeServerItem {
  serverNo: string;
  productNameZhCn: string;
  productNameEnUs: string;
  state: string;
}
export interface NativeOperation {
  failureCode?: string;
  availableOutputs?: {
    name: "generatedCredential" | "consoleTicket";
    policy: string;
    expiresAt?: number;
  }[];
  waitingUntil?: number;
  operationNo: string;
  action: string;
  status: string;
  createdAt: number;
  updatedAt: number;
}
export interface NativeServerDetail extends Omit<NativeServerItem, "state"> {
  entitlementNo: string;
  addresses: string[];
  addressesConfirmed: boolean;
  entitlementStatus: string;
  runtimeState: string;
  orderNo: string;
  periodUnit: string;
  periodCount: number;
  startedAt: number;
  expiresAt?: number;
  observedAt?: number;
  information?: {
    hostname: string;
    osName: string;
    loginUsername: string;
    port?: number;
    cpuCores?: number;
    memoryMiB?: number;
    networkType: string;
    bootOrder: string;
    rescue?: boolean;
  };
  commercialCapabilities?: {
    action: string;
    supported: boolean;
    canQuote: boolean;
    reasonCode: string;
  }[];
  capabilities?: {
    action: string;
    supported: boolean;
    available: boolean;
    reasonCode: string;
  }[];
  availableActions: string[];
  operations: NativeOperation[];
  credentialAvailable: boolean;
}
const path = (suffix = "") =>
  pluginApiPath("idc-resource", `account/native-servers${suffix}`);
export function listNativeServers(page = 1, limit = 20) {
  return requestClient.get<{ items: NativeServerItem[]; total: number }>(
    path(),
    { params: { page, limit }, silentErrorMessage: true },
  );
}
export function getNativeServer(serverNo: string) {
  return requestClient.get<NativeServerDetail>(
    path(`/${encodeURIComponent(serverNo)}`),
    { silentErrorMessage: true },
  );
}
export function resolveNativeServer(entitlementNo: string) {
  return requestClient.get<{ serverNo: string }>(
    path(`/resolve/${encodeURIComponent(entitlementNo)}`),
    { silentErrorMessage: true },
  );
}
export function createNativeOperation(
  serverNo: string,
  action: string,
  idempotencyKey: string,
  confirmed: boolean,
  parameters: NativeParameters = {},
  proofToken = "",
  password?: string,
) {
  return requestClient.post<{ operation: NativeOperation }>(
    path(`/${encodeURIComponent(serverNo)}/operations`),
    {
      action,
      idempotencyKey,
      confirmed,
      parameters,
      proofToken,
      ...(password === undefined ? {} : { password }),
    },
    { silentErrorMessage: true },
  );
}
// Deliberately one explicit request: never poll, retry or persist this payload.
export function revealInitialCredential(serverNo: string) {
  return requestClient.post<{ password: string }>(
    path(`/${encodeURIComponent(serverNo)}/initial-credential/reveal`),
    { confirmed: true },
    { silentErrorMessage: true },
  );
}

// Bounded customer-owned local facts; this endpoint never polls Providers per row.
export type NativeServerFact = {
  entitlementNo: string;
  serverNo: string;
  hostname: string;
  addresses: string[];
  addressesConfirmed: boolean;
  osName: string;
  lastObservedState: string;
  stale: boolean;
  observedAt?: number;
};
export function listNativeServerFacts(entitlementNos: string[]) {
  return requestClient.get<{ items: NativeServerFact[] }>(
    pluginApiPath("idc-resource", "account/native-server-facts"),
    { params: { entitlementNos }, silentErrorMessage: true },
  );
}

export type NativeParameters = Record<string, string | number | boolean>;
export interface NativeImage {
  reference: string;
  name: string;
  group: string;
  osType: string;
  arch: string;
  bootMode: string;
  selectable: boolean;
}
export interface NativeAccessory {
  objectNo: string;
  kind: string;
  name: string;
  state: string;
  projection: Record<string, unknown>;
}
export interface NativeOptions {
  images: NativeImage[];
  imagesObservedAt?: number;
  accessories: NativeAccessory[];
  accessoriesObservedAt?: number;
  accessoriesExpiresAt?: number;
}
export interface NativeConfirmation {
  required: boolean;
  proofToken: string;
  expiresAt?: number;
}
export function getNativeOptions(serverNo: string) {
  return requestClient.get<NativeOptions>(
    path(`/${encodeURIComponent(serverNo)}/management-options`),
    { silentErrorMessage: true },
  );
}
export function prepareNativeOperation(
  serverNo: string,
  action: string,
  parameters: NativeParameters,
  idempotencyKey: string,
) {
  return requestClient.post<NativeConfirmation>(
    path(`/${encodeURIComponent(serverNo)}/operation-confirmations`),
    { action, parameters, idempotencyKey },
    { silentErrorMessage: true },
  );
}
export function getNativeOperation(serverNo: string, operationNo: string) {
  return requestClient
    .get<{
      operation: NativeOperation;
    }>(
      path(
        `/${encodeURIComponent(serverNo)}/operations/${encodeURIComponent(operationNo)}`,
      ),
      { silentErrorMessage: true },
    )
    .then((result) => {
      if (!result.operation || result.operation.operationNo !== operationNo)
        throw new Error("暂时无法读取操作结果，请刷新后重试");
      return result;
    });
}
export interface GeneratedCredential {
  username: string;
  password: string;
  port?: number;
}
export interface ConsoleTicket {
  vnc_url_https?: string;
  vnc_url_http?: string;
  vnc_url?: string;
  token?: string;
  vnc_password?: string;
}
// Explicit, single delivery only. The request layer never retries reveal endpoints.
export function revealNativeOutput<
  T extends GeneratedCredential | ConsoleTicket,
>(
  serverNo: string,
  operationNo: string,
  name: "generatedCredential" | "consoleTicket",
) {
  return requestClient.post<T>(
    path(
      `/${encodeURIComponent(serverNo)}/operations/${encodeURIComponent(operationNo)}/outputs/${name}/reveal`,
    ),
    { confirmed: true },
    { silentErrorMessage: true },
  );
}

export function listNativeOperations(
  serverNo: string,
  page = 1,
  action = "",
  limit = 20,
) {
  return requestClient.get<{ items: NativeOperation[]; total: number }>(
    path(`/${encodeURIComponent(serverNo)}/operations`),
    {
      params: { page, limit, ...(action ? { action } : {}) },
      silentErrorMessage: true,
    },
  );
}
export type CommercialAction =
  | "snapshot_create"
  | "backup_create"
  | "traffic_add"
  | "ip_change"
  | "resize";
export interface NativeCommercialQuote {
  direction?: string;
  effectiveAt?: number;
  currentRecurringMinor?: string;
  targetRecurringMinor?: string;
  quoteNo: string;
  action: CommercialAction;
  currencyCode: string;
  totalAmountMinor: string;
  taxAmountMinor: string;
  expiresAt: number;
  freeCountPerServer: number;
  existingObjectCount: number;
  chargedQuantity: number;
  trafficGb: number;
  periodUnit: string;
  periodCount: number;
}
export interface NativePurchase {
  operationNo: string;
  quoteNo: string;
  orderNo: string;
  invoiceNo: string;
  status: string;
  currencyCode: string;
  totalAmountMinor: string;
  lastErrorCode: string;
}
export interface NativeTrafficPackage {
  packageNo: string;
  nameZhCn: string;
  nameEnUs: string;
  currencyCode: string;
  trafficGb: number;
  amountMinor: string;
  remainingStock?: number;
  unlimitedStock: boolean;
}
export function listNativeTrafficPackages(serverNo: string) {
  return requestClient.get<{ items: NativeTrafficPackage[] }>(
    path(`/${encodeURIComponent(serverNo)}/traffic-packages`),
    { silentErrorMessage: true },
  );
}
export function quoteNativePurchase(
  serverNo: string,
  input: {
    action: CommercialAction;
    parameters: NativeParameters;
    packageNo?: string;
    targetQuoteNo?: string;
    idempotencyKey: string;
  },
) {
  return requestClient.post<NativeCommercialQuote>(
    path(`/${encodeURIComponent(serverNo)}/commercial-quotes`),
    input,
    { silentErrorMessage: true },
  );
}
export function prepareNativePurchase(
  serverNo: string,
  quoteNo: string,
  idempotencyKey: string,
) {
  return requestClient.post<NativeConfirmation>(
    path(`/${encodeURIComponent(serverNo)}/commercial-confirmations`),
    { quoteNo, idempotencyKey },
    { silentErrorMessage: true },
  );
}
export function createNativePurchase(
  serverNo: string,
  quoteNo: string,
  idempotencyKey: string,
  proofToken: string,
) {
  return requestClient.post<NativePurchase>(
    path(`/${encodeURIComponent(serverNo)}/commercial-purchases`),
    { quoteNo, idempotencyKey, proofToken, confirmed: true },
    { silentErrorMessage: true },
  );
}
