import { pluginApiPath, requestClient } from '#/api/request';
import { requestWithHttpStatus } from './request-with-http-status';

export type CustomerTicketStage =
  | 'closed'
  | 'in_progress'
  | 'new'
  | 'resolved'
  | 'waiting_customer';

export interface CustomerTicketActions {
  canClose: boolean;
  canRate: boolean;
  canReopen: boolean;
  canReply: boolean;
}

export interface CustomerTicketCategory {
  categoryNo: string;
  nameEnUS: string;
  nameZhCN: string;
  priority?: string;
}

export interface CustomerTicketAttachment {
  attachmentNo: string;
  fileName: string;
  mediaType: string;
  sizeBytes: number;
}

export interface CustomerTicketMessage {
  attachments: CustomerTicketAttachment[];
  author: string;
  automated: boolean;
  body: string;
  createdAt: number;
  messageNo: string;
}

export type SupportCustomFieldType =
  | 'boolean'
  | 'date'
  | 'multi_line'
  | 'multi_select'
  | 'number'
  | 'single_line'
  | 'single_select';

export interface CustomerSupportCustomFieldOption {
  nameEnUS: string;
  nameZhCN: string;
  optionNo: string;
  sortOrder: number;
}

export interface CustomerSupportCustomFieldDefinition {
  descriptionEnUS?: string;
  descriptionZhCN?: string;
  fieldNo: string;
  fieldType: SupportCustomFieldType;
  nameEnUS: string;
  nameZhCN: string;
  options: CustomerSupportCustomFieldOption[];
  required: boolean;
  sortOrder: number;
}

export interface CustomerTicketCustomFieldSubmission {
  booleanValue?: boolean;
  dateValue?: string;
  fieldNo: string;
  numberValue?: string;
  optionNos?: string[];
  textValue?: string;
}

export type CustomerTicketCustomFieldValue = Omit<
  CustomerSupportCustomFieldDefinition,
  'options'
> & {
  normalizedValue: string;
  selectedOptions: CustomerSupportCustomFieldOption[];
};

export interface CustomerTicketRelatedEntitlement {
  associationReference: string;
  currentAvailable: boolean;
  currentNameEnUS?: string;
  currentNameZhCN?: string;
  currentStatus?: string;
  currentVersion?: number;
  snapshotNameEnUS: string;
  snapshotNameZhCN: string;
  snapshotStatus: string;
  snapshotVersion: number;
}

export interface CustomerSupportEntitlementCandidate {
  associationReference: string;
  nameEnUS: string;
  nameZhCN: string;
  status: string;
}

export interface CustomerTicketSummary {
  category?: CustomerTicketCategory;
  createdAt: number;
  currentRound: number;
  lastActivityAt: number;
  priority: string;
  relatedEntitlement?: CustomerTicketRelatedEntitlement;
  stage: CustomerTicketStage;
  subject: string;
  ticketNo: string;
  updatedAt: number;
  version: number;
}

export interface CustomerTicketDetail extends CustomerTicketSummary {
  actions: CustomerTicketActions;
  closedAt?: number;
  customFields: CustomerTicketCustomFieldValue[];
  messages: CustomerTicketMessage[];
  rating?: {
    feedback?: string;
    roundNo: number;
    stars: number;
    submittedAt: number;
  };
  reopenDeadline?: number;
  resolvedAt?: number;
}

export interface CustomerTicketPage {
  items: CustomerTicketSummary[];
  nextCursor: string;
  total: number;
}

export interface CustomerTicketFilters {
  cursor?: string;
  limit?: number;
  page?: number;
  query?: string;
}

export interface CreateCustomerTicketInput {
  associationReference?: string;
  body: string;
  categoryNo: string;
  customFields?: CustomerTicketCustomFieldSubmission[];
  subject: string;
  uploadNos?: string[];
}

interface TicketItemDTO {
  category?: CustomerTicketCategory;
  closedAt?: number;
  createdAt: number;
  currentRound: number;
  lastActivityAt: number;
  priority: string;
  relatedEntitlement?: CustomerTicketRelatedEntitlement;
  resolvedAt?: number;
  status: CustomerTicketStage;
  subject: string;
  ticketNo: string;
  updatedAt: number;
  version: number;
}

interface TicketAttachmentDTO {
  attachmentNo: string;
  fileName: string;
  mediaType: string;
  sizeBytes: number;
}

interface TicketMessageDTO {
  attachments?: TicketAttachmentDTO[];
  author: string;
  automated: boolean;
  body: string;
  createdAt: number;
  messageNo: string;
}

export interface CustomerTicketDetailDTO {
  allowedActions?: string[];
  messages?: TicketMessageDTO[];
  customFields?: CustomerTicketCustomFieldValue[];
  rating?: {
    feedback?: string;
    roundNo: number;
    stars: number;
    submittedAt: number;
  };
  reopenDeadline?: number;
  ticket: TicketItemDTO;
}

interface TicketDetailEnvelope {
  item: CustomerTicketDetailDTO;
}

interface AttachmentUploadReservation {
  expiresAt: number;
  nextOffset: number;
  uploadNo: string;
  version: number;
}

interface AttachmentUploadResult {
  fileName: string;
  mediaType: string;
  ready: boolean;
  sizeBytes: number;
  uploadNo: string;
  version: number;
}

const base = pluginApiPath('idc-core', 'account/support/tickets');
const categoryBase = pluginApiPath('idc-core', 'account/support/categories');
const uploadBase = pluginApiPath(
  'idc-core',
  'account/support/ticket-attachment-uploads',
);
const initialUploadBase = pluginApiPath(
  'idc-core',
  'account/support/initial-attachment-uploads',
);
const attachmentBase = pluginApiPath('idc-core', 'account/support/attachments');
const customFieldBase = pluginApiPath(
  'idc-core',
  'account/support/custom-fields',
);
const entitlementCandidateBase = pluginApiPath(
  'idc-core',
  'account/support/entitlement-candidates',
);

function mapTicketSummary(ticket: TicketItemDTO): CustomerTicketSummary {
  return {
    category: ticket.category,
    createdAt: ticket.createdAt,
    currentRound: ticket.currentRound,
    lastActivityAt: ticket.lastActivityAt,
    priority: ticket.priority,
    relatedEntitlement: ticket.relatedEntitlement,
    stage: ticket.status,
    subject: ticket.subject,
    ticketNo: ticket.ticketNo,
    updatedAt: ticket.updatedAt,
    version: ticket.version,
  };
}

export function mapCustomerTicketDetail(
  detail: CustomerTicketDetailDTO,
): CustomerTicketDetail {
  const actions = new Set(detail.allowedActions || []);
  return {
    ...mapTicketSummary(detail.ticket),
    actions: {
      canClose: actions.has('close'),
      canRate: actions.has('rate') && detail.ticket.currentRound > 0,
      canReopen: actions.has('reopen'),
      canReply: actions.has('reply'),
    },
    closedAt: detail.ticket.closedAt,
    customFields: detail.customFields || [],
    messages: (detail.messages || []).map((message) => ({
      attachments: (message.attachments || []).map((attachment) => ({
        attachmentNo: attachment.attachmentNo,
        fileName: attachment.fileName,
        mediaType: attachment.mediaType,
        sizeBytes: attachment.sizeBytes,
      })),
      author: message.author,
      automated: message.automated,
      body: message.body,
      createdAt: message.createdAt,
      messageNo: message.messageNo,
    })),
    rating: detail.rating,
    reopenDeadline: detail.reopenDeadline,
    resolvedAt: detail.ticket.resolvedAt,
  };
}

export function listCustomerTicketCategories(
  params: { cursor?: string; keyword?: string; limit?: number } = {},
) {
  return requestClient.get<{
    items: CustomerTicketCategory[];
    nextCursor: string;
  }>(categoryBase, { params });
}

export function listCustomerSupportCustomFields() {
  return requestClient.get<{ items: CustomerSupportCustomFieldDefinition[] }>(
    customFieldBase,
  );
}

export function listCustomerSupportEntitlementCandidates(
  params: {
    associationReference?: string;
    cursor?: string;
    keyword?: string;
    limit?: number;
  } = {},
) {
  return requestClient.get<{
    items: CustomerSupportEntitlementCandidate[];
    nextCursor: string;
  }>(entitlementCandidateBase, { params });
}

export async function listCustomerTickets(params: CustomerTicketFilters = {}) {
  const result = await requestWithHttpStatus<{
    nextCursor: string;
    tickets: TicketItemDTO[];
    total: number;
  }>(requestClient, base, { method: 'GET', params });
  return {
    items: (result.tickets || []).map(mapTicketSummary),
    nextCursor: result.nextCursor || '',
    total: result.total,
  } satisfies CustomerTicketPage;
}

export async function getCustomerTicket(ticketNo: string) {
  const result = await requestClient.get<TicketDetailEnvelope>(
    `${base}/${encodeURIComponent(ticketNo)}`,
  );
  return mapCustomerTicketDetail(result.item);
}

export async function createCustomerTicket(input: CreateCustomerTicketInput) {
  const result = await requestClient.post<TicketDetailEnvelope>(
    base,
    {
      body: input.body,
      categoryNo: input.categoryNo,
      customFields: input.customFields || [],
      ...(input.associationReference
        ? { associationReference: input.associationReference }
        : {}),
      subject: input.subject,
      uploadNos: input.uploadNos || [],
    },
    { silentErrorMessage: true },
  );
  return mapCustomerTicketDetail(result.item);
}

export async function uploadInitialCustomerTicketAttachments(files: File[]) {
  const uploadNos: string[] = [];
  for (const file of files) {
    const reservation = await requestClient.post<AttachmentUploadReservation>(
      initialUploadBase,
      {
        fileName: file.name,
        sizeBytes: file.size,
      },
      { silentErrorMessage: true },
    );
    const uploaded = await requestClient.put<AttachmentUploadResult>(
      `${initialUploadBase}/${encodeURIComponent(reservation.uploadNo)}/content`,
      file,
      {
        headers: {
          'Content-Type': file.type || 'application/octet-stream',
        },
        silentErrorMessage: true,
        timeout: 120_000,
      },
    );
    if (!uploaded.ready) {
      throw new Error('Initial ticket attachment upload did not finish');
    }
    uploadNos.push(uploaded.uploadNo);
  }
  return uploadNos;
}

export function downloadCustomerTicketAttachment(attachmentNo: string) {
  return requestClient.download<Blob>(
    `${attachmentBase}/${encodeURIComponent(attachmentNo)}/content`,
    { silentErrorMessage: true },
  );
}

async function uploadCustomerTicketAttachments(
  ticketNo: string,
  files: File[],
) {
  const uploadNos: string[] = [];
  for (const file of files) {
    const reservation = await requestClient.post<AttachmentUploadReservation>(
      `${base}/${encodeURIComponent(ticketNo)}/attachment-uploads`,
      { fileName: file.name, sizeBytes: file.size },
    );
    const uploaded = await requestClient.put<AttachmentUploadResult>(
      `${uploadBase}/${encodeURIComponent(reservation.uploadNo)}/content`,
      file,
      {
        headers: {
          'Content-Type': file.type || 'application/octet-stream',
        },
        timeout: 120_000,
      },
    );
    if (!uploaded.ready) {
      throw new Error('Ticket attachment upload did not finish');
    }
    uploadNos.push(uploaded.uploadNo);
  }
  return uploadNos;
}

export async function replyCustomerTicket(
  ticketNo: string,
  body: string,
  ticketVersion: number,
  files: File[] = [],
) {
  const uploadNos = await uploadCustomerTicketAttachments(ticketNo, files);
  const result = await requestClient.post<TicketDetailEnvelope>(
    `${base}/${encodeURIComponent(ticketNo)}/replies`,
    { body, ticketVersion, uploadNos },
  );
  return mapCustomerTicketDetail(result.item);
}

export async function closeCustomerTicket(
  ticketNo: string,
  ticketVersion: number,
) {
  const result = await requestClient.post<TicketDetailEnvelope>(
    `${base}/${encodeURIComponent(ticketNo)}/closures`,
    { confirmed: true, ticketVersion },
  );
  return mapCustomerTicketDetail(result.item);
}

export async function reopenCustomerTicket(
  ticketNo: string,
  ticketVersion: number,
) {
  const result = await requestClient.post<TicketDetailEnvelope>(
    `${base}/${encodeURIComponent(ticketNo)}/reopenings`,
    { confirmed: true, ticketVersion },
  );
  return mapCustomerTicketDetail(result.item);
}

export async function rateCustomerTicket(
  ticketNo: string,
  stars: number,
  roundNo: number,
  ticketVersion: number,
  feedback = '',
) {
  const result = await requestClient.post<TicketDetailEnvelope>(
    `${base}/${encodeURIComponent(ticketNo)}/ratings`,
    { feedback, roundNo, stars, ticketVersion },
  );
  return mapCustomerTicketDetail(result.item);
}
