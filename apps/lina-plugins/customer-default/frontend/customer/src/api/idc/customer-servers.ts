import { pluginApiPath, requestClient } from '#/api/request';

const pluginID = 'idc-core';

export type ServerAction =
  | 'backup_create'
  | 'backup_delete'
  | 'backup_restore'
  | 'console'
  | 'ip_change'
  | 'nat_create'
  | 'nat_delete'
  | 'reboot'
  | 'reinstall'
  | 'reset_password'
  | 'resize'
  | 'security_group_attach'
  | 'security_group_detach'
  | 'snapshot_create'
  | 'snapshot_delete'
  | 'snapshot_restore'
  | 'start'
  | 'stop'
  | 'traffic_add'
  | 'virtual_media_mount'
  | 'virtual_media_unmount';

export interface ServerAccessory {
  kind: string;
  name: string;
  objectNo: string;
  projection: Record<string, unknown>;
  state: string;
}

export interface ServerOperationSummary {
  action: ServerAction;
  createdAt: number;
  operationNo: string;
  status: string;
}

export interface CustomerServer {
  activeOperation?: ServerOperationSummary;
  activeRefundNo: string;
  autoRenewEnabled: boolean;
  availableActions: ServerAction[];
  capabilityObservedAt?: number;
  customerAlias: string;
  customerRemark: string;
  entitlementExpiresAt?: number;
  entitlementStartedAt?: number;
  entitlementStatus: string;
  periodCount: number;
  periodUnit: string;
  productNameEnUS: string;
  productNameZhCN: string;
  productType: string;
  providerTaskStatus: string;
  renewalCycles: number;
  runtimeStatus: string;
  serviceNo: string;
  updatedAt: number;
  version: number;
}

export interface ReinstallImage {
  arch: string;
  bootMode: string;
  group: string;
  name: string;
  osType: string;
  reference: string;
  selectable: boolean;
}

export interface ServerConfigurationItem {
  componentType: string;
  groupNameEnUS: string;
  groupNameZhCN: string;
  normalizedValue: unknown;
  optionNameEnUS: string;
  optionNameZhCN: string;
  quantity: number;
}

export interface ServerOperationHistory extends ServerOperationSummary {
  completedAt?: number;
  confidentialOutputAvailable: boolean;
  failureCode: string;
  updatedAt: number;
}

export interface CustomerServerDetail {
  accessories: ServerAccessory[];
  accessoriesExpiresAt?: number;
  accessoriesObservedAt?: number;
  configuration: ServerConfigurationItem[];
  operations: ServerOperationHistory[];
  reinstallImages: ReinstallImage[];
  reinstallImagesObservedAt?: number;
  server: CustomerServer;
}

export interface ServerAccessoryQuote {
  trafficGb: number;
  packageNameZhCN: string;
  packageNameEnUS: string;
  action: ServerAction;
  chargedQuantity: number;
  currencyCode: string;
  expiresAt: number;
  freeCountPerServer: number;
  kind: string;
  periodCount: number;
  periodUnitCode: string;
  quoteNo: string;
  requestedQuantity: number;
  totalAmountMinor: number;
}

export interface ServerAccessoryCheckout {
  amountMinor: number;
  currencyCode: string;
  invoiceNo: string;
  operationNo: string;
  paymentDueAt: number;
  paymentRequired: boolean;
  serviceNo: string;
  status: string;
}

export interface ServerIPChangeQuote {
  amountMinor: number;
  currencyCode: string;
  expiresAt: number;
  freeChange: boolean;
  ipQuantity: number;
  quoteNo: string;
  serviceNo: string;
}

export interface ServerIPChangeCheckout extends ServerAccessoryCheckout {}

export interface IncomingServerTransfer {
  expiresAt: number;
  serviceNo: string;
  transferNo: string;
}

export interface ServerResizeSelection {
  optionNo: string;
  quantity: number;
}
export interface ServerResizeOption {
  componentType: string;
  defaultQuantity: number;
  maximumQuantity: number;
  minimumQuantity: number;
  nameEnUS: string;
  nameZhCN: string;
  optionNo: string;
}
export interface ServerResizeGroup {
  groupNo: string;
  maximumSelected: number;
  minimumSelected: number;
  nameEnUS: string;
  nameZhCN: string;
  options: ServerResizeOption[];
  selectionMode: string;
}
export interface ServerResizeOptions {
  currencyCode: string;
  currentSelections: ServerResizeSelection[];
  groups: ServerResizeGroup[];
  publicSlug: string;
  revision: string;
  rules: Array<Record<string, unknown>>;
}
export interface ServerResizeQuote {
  chargeAmountMinor: number;
  currencyCode: string;
  currentRecurringAmountMinor: number;
  direction: 'downgrade' | 'upgrade';
  effectiveAt: number;
  expiresAt: number;
  issuedAt: number;
  quoteNo: string;
  serviceNo: string;
  targetRecurringAmountMinor: number;
}
export interface ServerResizeCheckout {
  chargeAmountMinor: number;
  currencyCode: string;
  direction: 'downgrade' | 'upgrade';
  effectiveAt: number;
  invoiceNo: string;
  operationNo: string;
  orderNo: string;
  serviceNo: string;
  status: string;
}

export interface OperationIntent {
  action: ServerAction;
  parameters: Record<string, boolean | number | string>;
  requestNo: string;
}

function path(pathName = '') {
  return pluginApiPath(pluginID, `account/servers${pathName}`);
}

export function listCustomerServers(
  params: { cursor?: string; limit?: number } = {},
) {
  return requestClient.get<{ items: CustomerServer[]; nextCursor: string }>(
    path(),
    {
      params: { limit: 20, ...params },
    },
  );
}

export function getCustomerServer(serviceNo: string) {
  return requestClient.get<{ detail: CustomerServerDetail }>(
    path(`/${encodeURIComponent(serviceNo)}`),
  );
}

export function updateCustomerServerMetadata(
  serviceNo: string,
  input: { alias: string; expectedVersion: number; remark: string },
) {
  return requestClient.put<{
    metadata: { alias: string; remark: string; version: number };
  }>(path(`/${encodeURIComponent(serviceNo)}/metadata`), input);
}

export function requestCustomerServerTransfer(
  serviceNo: string,
  recipientEmail: string,
  idempotencyKey: string,
) {
  return requestClient.post<{ accepted: boolean; reused: boolean }>(
    path(`/${encodeURIComponent(serviceNo)}/transfers`),
    { idempotencyKey, recipientEmail },
  );
}

export function listIncomingServerTransfers() {
  return requestClient.get<{ items: IncomingServerTransfer[] }>(
    pluginApiPath(pluginID, 'account/server-transfers/incoming'),
  );
}

export function confirmIncomingServerTransfer(
  transferNo: string,
  idempotencyKey: string,
) {
  return requestClient.post<{
    reused: boolean;
    serviceNo: string;
    status: string;
    transferNo: string;
  }>(
    pluginApiPath(
      pluginID,
      `account/server-transfers/${encodeURIComponent(transferNo)}/confirmations`,
    ),
    { idempotencyKey },
  );
}

export function getServerResizeOptions(serviceNo: string) {
  return requestClient.get<ServerResizeOptions>(
    path(`/${encodeURIComponent(serviceNo)}/resize-options`),
  );
}

export function createServerResizeQuote(
  serviceNo: string,
  input: {
    currencyCode: string;
    idempotencyKey: string;
    revision: string;
    selections: ServerResizeSelection[];
  },
) {
  return requestClient.post<{ quote: ServerResizeQuote }>(
    path(`/${encodeURIComponent(serviceNo)}/resize-quotes`),
    input,
  );
}

export function checkoutServerResizeQuote(
  quoteNo: string,
  idempotencyKey: string,
) {
  return requestClient.post<{ checkout: ServerResizeCheckout }>(
    pluginApiPath(
      pluginID,
      `account/server-resize-quotes/${encodeURIComponent(quoteNo)}/checkout`,
    ),
    { idempotencyKey },
  );
}

export function authorizeServerOperation(
  serviceNo: string,
  intent: OperationIntent,
) {
  return requestClient.post<{
    expiresAt?: number;
    proofToken: string;
    required: boolean;
  }>(
    path(`/${encodeURIComponent(serviceNo)}/operation-authorizations`),
    intent,
  );
}

export function createServerOperation(
  serviceNo: string,
  intent: OperationIntent,
  options: { password?: string; proofToken?: string } = {},
) {
  return requestClient.post<{ operation: ServerOperationSummary }>(
    path(`/${encodeURIComponent(serviceNo)}/operations`),
    { ...intent, ...options },
  );
}

export function createServerAccessoryQuote(
  serviceNo: string,
  action: ServerAction,
  parameters: Record<string, boolean | number | string>,
  idempotencyKey: string,
  packageNo?: string,
) {
  return requestClient.post<{ quote: ServerAccessoryQuote }>(
    path(`/${encodeURIComponent(serviceNo)}/accessory-quotes`),
    { action, idempotencyKey, parameters, packageNo },
  );
}

export function checkoutServerAccessoryQuote(
  quoteNo: string,
  idempotencyKey: string,
) {
  return requestClient.post<{ checkout: ServerAccessoryCheckout }>(
    pluginApiPath(
      pluginID,
      `account/server-accessory-quotes/${encodeURIComponent(quoteNo)}/checkout`,
    ),
    { idempotencyKey },
  );
}

export function createServerIPChangeQuote(
  serviceNo: string,
  idempotencyKey: string,
) {
  return requestClient.post<{ quote: ServerIPChangeQuote }>(
    path(`/${encodeURIComponent(serviceNo)}/ip-change-quotes`),
    { idempotencyKey },
  );
}

export function checkoutServerIPChangeQuote(
  quoteNo: string,
  idempotencyKey: string,
  proofToken = '',
) {
  return requestClient.post<{ checkout: ServerIPChangeCheckout }>(
    pluginApiPath(
      pluginID,
      `account/server-ip-change-quotes/${encodeURIComponent(quoteNo)}/checkout`,
    ),
    { idempotencyKey, proofToken },
  );
}

export function redeemServerOperationOutput(
  serviceNo: string,
  operationNo: string,
) {
  return requestClient.post<{ payload: Record<string, unknown> }>(
    path(
      `/${encodeURIComponent(serviceNo)}/operations/${encodeURIComponent(operationNo)}/confidential-output`,
    ),
    { confirmed: true },
  );
}
