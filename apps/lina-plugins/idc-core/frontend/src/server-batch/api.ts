import type { PluginHostMountContext } from '../host-context';

import { createPluginRequest } from '../runtime/request';

const base = '/x/idc-core/api/v1/admin/resource/server-batches';
const classificationBase = '/x/idc-core/api/v1/admin/resource/server-classifications';

export type ServerBatchAction =
  | 'ip_change'
  | 'classification_set'
  | 'note_append'
  | 'reboot'
  | 'reinstall'
  | 'renewal_reminder'
  | 'suspend'
  | 'tag_add'
  | 'tag_remove'
  | 'terminate';

export type ServerBatchItem = {
  actionAmountMinor: number;
  currencyCode: string;
  customerNo: string;
  entitlementExpiresAt?: number;
  entitlementStatus: string;
  executedAt?: number;
  executionStatus: string;
  failureCode: string;
  irreversible: boolean;
  lineNumber: number;
  notificationEventNo: string;
  operationNo: string;
  previewStatus: string;
  providerStatus: string;
  resourceSummary: string;
  runtimeStatus: string;
  serviceNo: string;
  serviceVersion: number;
  unpaidAmountMinor: number;
};

export type ServerBatch = {
  action: ServerBatchAction;
  batchNo: string;
  createdAt: number;
  dangerous: boolean;
  items: ServerBatchItem[];
  previewDigest: string;
  status: string;
};

export type ServerBatchPreviewItem = {
  parameters: Record<string, number | string>;
  serviceNo: string;
};

export type ServerClassification = {
  classificationNo: string;
  nameEnUS: string;
  nameZhCN: string;
  status: string;
  version: number;
};

export function createServerBatchAPI(
  currentContext: () => PluginHostMountContext,
) {
  const request = createPluginRequest(currentContext);
  return {
    classifications() {
      return request<{ items: ServerClassification[] }>(classificationBase).then(({ items }) => items);
    },
    saveClassification(input: { classificationNo?: string; expectedVersion?: number; nameEnUS: string; nameZhCN: string; requestNo: string }) {
      const path = input.classificationNo ? `${classificationBase}/${encodeURIComponent(input.classificationNo)}` : classificationBase;
      return request<{ item: ServerClassification }>(path, { body: input, method: input.classificationNo ? 'PUT' : 'POST' }).then(({ item }) => item);
    },
    disableClassification(item: ServerClassification) {
      return request<{ item: ServerClassification }>(`${classificationBase}/${encodeURIComponent(item.classificationNo)}`, {
        body: { expectedVersion: item.version }, method: 'DELETE',
      }).then(({ item: result }) => result);
    },
    confirm(batch: ServerBatch) {
      return request<{ expiresAt: number; proofToken: string }>(
        `${base}/${encodeURIComponent(batch.batchNo)}/confirmation`,
        {
          body: { previewDigest: batch.previewDigest },
          method: 'POST',
        },
      );
    },
    execute(batch: ServerBatch, proofToken = '') {
      return request<{ batch: ServerBatch }>(
        `${base}/${encodeURIComponent(batch.batchNo)}/${batch.dangerous ? 'dangerous' : 'safe'}-executions`,
        {
          body: { previewDigest: batch.previewDigest, proofToken },
          method: 'POST',
        },
      ).then(({ batch: result }) => result);
    },
    preview(input: {
      action: ServerBatchAction;
      items: ServerBatchPreviewItem[];
      requestNo: string;
    }) {
      const risk = ['ip_change', 'reboot', 'reinstall', 'suspend', 'terminate'].includes(input.action)
        ? 'dangerous'
        : 'safe';
      return request<{ batch: ServerBatch }>(`${base}/${risk}/previews`, {
        body: input,
        method: 'POST',
      }).then(({ batch }) => batch);
    },
  };
}
