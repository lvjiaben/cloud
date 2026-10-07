import type { PluginHostMountContext } from '../host-context';

import { createPluginRequest } from '../runtime/request';

const contentBase = '/x/idc-core/api/v1/admin/content';

export type ContentSummary = {
  availableActions?: string[];
  categoryNameEnUs?: string;
  categoryNameZhCn?: string;
  categoryNo?: string;
  contentNo: string;
  createdAt: number;
  currentFileName?: string;
  fileGeneration?: number;
  publicPath?: string;
  status: string;
  summary?: string;
  title: string;
  type: string;
  updatedAt: number;
  version?: number;
  visibility: string;
};

export type ContentFile = {
  canCancel?: boolean;
  canEnable?: boolean;
  canRescan?: boolean;
  contentKind?: string;
  fileNo?: string;
  fileName?: string;
  gateExpiresAt?: number;
  gateStatus?: string;
  mediaType?: string;
  scanStatus?: string;
  sha256?: string;
  sizeBytes?: number;
  version?: number;
};

export type ContentUploadSession = {
  canCancel?: boolean;
  canResume?: boolean;
  chunkSizeBytes: number;
  confirmedBytes: number;
  expiresAt: number;
  fileName: string;
  sizeBytes: number;
  uploadNo: string;
  version: number;
};

export type ContentUploadChunk = {
  complete: boolean;
  confirmedBytes: number;
  nextOffset: number;
  uploadNo: string;
  version: number;
};

export type ContentUploadPolicy = {
  inspectionRequiredAvailable: boolean;
  inspectionRequiredExtensions: string[];
  inspectionRequiredMaxSizeBytes: number;
  ordinaryExtensions: string[];
  ordinaryMaxSizeBytes: number;
};

export type ContentDetail = ContentSummary & {
  availableActions: string[];
  activeUpload?: ContentUploadSession;
  body: string;
  candidateFile?: ContentFile;
  currentFile?: ContentFile;
  seoDescription?: string;
  seoTitle?: string;
  targetCustomerNos?: string[];
  targetCustomerGroupNos?: string[];
  targetAudiences?: ContentAudienceTarget[];
  targetServiceNos?: string[];
  version: number;
};

export type ContentAudienceCandidate = {
  customerNo?: string;
  groupNo?: string;
  label: string;
  serviceNo?: string;
};

export type ContentAudienceTarget = {
  label: string;
  number: string;
  type: 'customer' | 'customer_group' | 'service';
};

type ContentAudienceTargetWire = Omit<ContentAudienceTarget, 'type'> & {
  kind?: ContentAudienceTarget['type'];
  type?: ContentAudienceTarget['type'];
};

type ContentDetailWire = Omit<ContentDetail, 'targetAudiences'> & {
  targetAudiences?: ContentAudienceTargetWire[];
};

export type ContentCategory = {
  categoryNo: string;
  name: string;
  synthetic?: boolean;
  type: string;
  version: number;
};

export type ContentListResult = {
  items: ContentSummary[];
  nextCursor: string;
};

export type ContentWriteInput = {
  body: string;
  categoryNo?: string;
  seoDescription?: string;
  seoTitle?: string;
  status: 'draft' | 'published';
  summary?: string;
  targetCustomerGroupNos?: string[];
  targetCustomerNos?: string[];
  targetServiceNos?: string[];
  title: string;
  type: string;
  version?: number;
  visibility: string;
};

function queryString(query: Record<string, number | string | undefined>) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== '') {
      params.set(key, String(value));
    }
  }
  const encoded = params.toString();
  return encoded ? `?${encoded}` : '';
}

function normalizeDetail(item: ContentDetailWire): ContentDetail {
  const targetAudiences = (item.targetAudiences ?? []).flatMap((target) => {
    const type = target.type ?? target.kind;
    return type
      ? [{ label: target.label, number: target.number, type }]
      : [];
  });
  return { ...item, targetAudiences };
}

export function createContentAPI(
  currentContext: () => PluginHostMountContext,
) {
  const request = createPluginRequest(currentContext);
  return {
    activateCandidate(contentNo: string, fileNo: string, body: unknown) {
      return request<{ item: ContentDetailWire }>(
        `${contentBase}/items/${encodeURIComponent(contentNo)}/file-candidates/${encodeURIComponent(fileNo)}/enablements`,
        { body, method: 'POST' },
      ).then(({ item }) => normalizeDetail(item));
    },
    audienceCustomerGroups(keyword: string) {
      return request<{ items: ContentAudienceCandidate[]; nextCursor: string }>(
        `${contentBase}/audience-customer-groups${queryString({ keyword, limit: 50 })}`,
      );
    },
    audienceCustomers(keyword: string) {
      return request<{ items: ContentAudienceCandidate[]; nextCursor: string }>(
        `${contentBase}/audience-customers${queryString({ keyword, limit: 50 })}`,
      );
    },
    audienceServices(keyword: string) {
      return request<{ items: ContentAudienceCandidate[]; nextCursor: string }>(
        `${contentBase}/audience-services${queryString({ keyword, limit: 50 })}`,
      );
    },
    cancelCandidate(contentNo: string, fileNo: string, body: unknown) {
      return request<{ item: ContentDetailWire }>(
        `${contentBase}/items/${encodeURIComponent(contentNo)}/file-candidates/${encodeURIComponent(fileNo)}/cancellation`,
        { body, method: 'POST' },
      ).then(({ item }) => normalizeDetail(item));
    },
    cancelUpload(uploadNo: string, body: unknown) {
      return request<Record<string, never>>(
        `${contentBase}/upload-sessions/${encodeURIComponent(uploadNo)}/cancellation`,
        { body, method: 'POST' },
      );
    },
    categories(type = '') {
      return request<{ items: ContentCategory[]; nextCursor: string }>(
        `${contentBase}/categories${queryString({ type })}`,
      );
    },
    create(input: ContentWriteInput) {
      return request<{ item: ContentDetailWire }>(`${contentBase}/items`, {
        body: input,
        method: 'POST',
      }).then(({ item }) => normalizeDetail(item));
    },
    createCategory(body: {
      name: string;
      sortOrder: number;
      type: string;
    }) {
      return request<{ item: ContentCategory }>(`${contentBase}/categories`, {
        body,
        method: 'POST',
      }).then(({ item }) => item);
    },
    createUploadSession(
      contentNo: string,
      body: { contentVersion: number; fileName: string; sizeBytes: number },
    ) {
      return request<{ item: ContentUploadSession }>(
        `${contentBase}/items/${encodeURIComponent(contentNo)}/upload-sessions`,
        { body, method: 'POST' },
      ).then(({ item }) => item);
    },
    delete(contentNo: string, version: number) {
      return request<{ item: ContentDetailWire }>(
        `${contentBase}/items/${encodeURIComponent(contentNo)}`,
        {
          body: { confirmed: true, contentNo, version },
          method: 'DELETE',
        },
      ).then(({ item }) => normalizeDetail(item));
    },
    deleteCategory(categoryNo: string, version: number) {
      return request<Record<string, never>>(
        `${contentBase}/categories/${encodeURIComponent(categoryNo)}`,
        {
          body: { categoryNo, confirmed: true, version },
          method: 'DELETE',
        },
      );
    },
    detail(contentNo: string) {
      return request<{ item: ContentDetailWire }>(
        `${contentBase}/items/${encodeURIComponent(contentNo)}`,
      ).then(({ item }) => normalizeDetail(item));
    },
    list(query: {
      categoryNo?: string;
      cursor?: string;
      query?: string;
      limit?: number;
      status?: string;
      type?: string;
      visibility?: string;
    }) {
      return request<ContentListResult>(
        `${contentBase}/items${queryString(query)}`,
      );
    },
    lifecycle(contentNo: string, action: string, body: unknown) {
      return request<{ item: ContentDetailWire }>(
        `${contentBase}/items/${encodeURIComponent(contentNo)}/${action}`,
        { body, method: 'POST' },
      ).then(({ item }) => normalizeDetail(item));
    },
    completeUpload(
      uploadNo: string,
      body: { expectedSizeBytes: number; version: number },
    ) {
      return request<{ item: ContentFile }>(
        `${contentBase}/upload-sessions/${encodeURIComponent(uploadNo)}/completion`,
        { body, method: 'POST', timeoutMs: 120_000 },
      ).then(({ item }) => item);
    },
    rescanCandidate(contentNo: string, fileNo: string, body: unknown) {
      return request<{ item: ContentDetailWire }>(
        `${contentBase}/items/${encodeURIComponent(contentNo)}/file-candidates/${encodeURIComponent(fileNo)}/scan-retries`,
        { body, method: 'POST' },
      ).then(({ item }) => normalizeDetail(item));
    },
    update(contentNo: string, input: ContentWriteInput) {
      return request<{ item: ContentDetailWire }>(
        `${contentBase}/items/${encodeURIComponent(contentNo)}`,
        { body: input, method: 'PUT' },
      ).then(({ item }) => normalizeDetail(item));
    },
    uploadChunk(uploadNo: string, offset: number, chunk: Blob) {
      return request<ContentUploadChunk>(
        `${contentBase}/upload-sessions/${encodeURIComponent(uploadNo)}/chunks/${offset}`,
        {
          headers: { 'Content-Type': 'application/octet-stream' },
          method: 'PUT',
          rawBody: chunk,
          timeoutMs: 120_000,
        },
      );
    },
    uploadPolicy() {
      return request<ContentUploadPolicy>(`${contentBase}/upload-policy`);
    },
    uploadSession(uploadNo: string) {
      return request<{ item: ContentUploadSession }>(
        `${contentBase}/upload-sessions/${encodeURIComponent(uploadNo)}`,
      ).then(({ item }) => item);
    },
  };
}
