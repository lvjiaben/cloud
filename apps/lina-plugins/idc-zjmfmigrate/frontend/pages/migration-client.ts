import { pluginApiPath, requestClient } from '#/api/request';
import { requestPreservingHttpStatus } from './migration-request';

const pluginID = 'idc-zjmfmigrate';
const basePath = 'admin/migration';

function api(pathName: string) {
  return pluginApiPath(pluginID, `${basePath}/${pathName}`);
}

export interface MigrationPreflightInput {
  charset: string;
  currencies: string[];
  sourceSystem: string;
  sourceVersion: string;
  timezone: string;
}

export interface MigrationPreflightResult {
  blockers: string[];
  capabilities: string[];
  sourceSystem: string;
  sourceVersion: string;
  supported: boolean;
  warnings: string[];
}

export interface MigrationBatch {
  batchNo: string;
  id: number;
  status: string;
}

export interface MigrationRunResult {
  blockers: string[];
  id: number;
  mapped: number;
  status: string;
}

export interface MigrationConflict {
  decision: string;
  id: number;
  objectType: string;
  sourceId: string;
  summary: string;
}

export interface MigrationReport {
  blockers: string[];
  byType: Record<string, string>;
  id: number;
  status: string;
  warnings: string[];
}

export function runPreflight(data: MigrationPreflightInput) {
  return requestClient.post<MigrationPreflightResult>(api('preflight'), data);
}

export function createBatch(data: {
  batchNo: string;
  mode: 'dry_run' | 'import';
  sourceSystem: string;
  sourceVersion: string;
}) {
  return requestClient.post<MigrationBatch>(api('batches'), data);
}

export function runBatch(id: number, watermark = '') {
  return requestClient.post<MigrationRunResult>(api(`batches/${id}/run`), {
    watermark,
  });
}

export function listConflicts(id: number, page = 1, pageSize = 10) {
  return requestPreservingHttpStatus<{ list: MigrationConflict[]; total: number }>(
    requestClient, api(`batches/${id}/conflicts`),
    { method: 'GET', params: { page, pageSize } },
  );
}

export function decideConflict(
  id: number,
  decision: 'keep_source' | 'merge' | 'skip',
  reason: string,
) {
  return requestClient.put(api(`conflicts/${id}/decision`), {
    decision,
    reason,
  });
}

export function reconcileBatch(id: number) {
  return requestClient.post<{
    differences: number;
    id: number;
    status: string;
  }>(api(`batches/${id}/reconcile`));
}

export function checkCutover(id: number) {
  return requestClient.post<{ allowed: boolean; checks: string[]; id: number }>(
    api(`batches/${id}/cutover`),
  );
}

export function rollbackBatch(id: number) {
  return requestClient.post<{ id: number; status: string }>(
    api(`batches/${id}/rollback`),
  );
}

export function getReport(id: number) {
  return requestClient.get<MigrationReport>(api(`batches/${id}/report`));
}
