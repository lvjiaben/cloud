// Resource-owned native server management; no supplier identifiers or secrets are persisted here.
import { pluginApiPath, requestClient } from '#/api/request';

export interface NativeServerItem {
  serverNo: string;
  productNameZhCn: string;
  productNameEnUs: string;
  state: string;
}
export interface NativeOperation {
  waitingUntil?: number;
  operationNo: string;
  action: string;
  status: string;
  createdAt: number;
  updatedAt: number;
}
export interface NativeServerDetail extends Omit<NativeServerItem, 'state'> {
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
  availableActions: string[];
  operations: NativeOperation[];
  credentialAvailable: boolean;
}
const path = (suffix = '') => pluginApiPath('idc-resource', `account/native-servers${suffix}`);
export function listNativeServers(page = 1, limit = 20) {
  return requestClient.get<{ items: NativeServerItem[]; total: number }>(path(), { params: { page, limit }, silentErrorMessage: true });
}
export function getNativeServer(serverNo: string) {
  return requestClient.get<NativeServerDetail>(path(`/${encodeURIComponent(serverNo)}`), { silentErrorMessage: true });
}
export function resolveNativeServer(entitlementNo: string) {
  return requestClient.get<{ serverNo: string }>(path(`/resolve/${encodeURIComponent(entitlementNo)}`), { silentErrorMessage: true });
}
export function createNativeOperation(serverNo: string, action: string, idempotencyKey: string, confirmed: boolean) {
  return requestClient.post<{ operation: NativeOperation }>(path(`/${encodeURIComponent(serverNo)}/operations`), { action, idempotencyKey, confirmed }, { silentErrorMessage: true });
}
// Deliberately one explicit request: never poll, retry or persist this payload.
export function revealInitialCredential(serverNo: string) {
  return requestClient.post<{ password: string }>(path(`/${encodeURIComponent(serverNo)}/initial-credential/reveal`), { confirmed: true }, { silentErrorMessage: true });
}
