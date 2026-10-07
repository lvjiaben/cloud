import type {
  CustomerTicketCategory,
  CustomerTicketAttachment,
  CustomerTicketDetail,
  CustomerTicketDetailDTO,
  CustomerSupportCustomFieldDefinition,
  CustomerTicketCustomFieldSubmission,
} from './support';

import { pluginApiPath, requestClient } from '#/api/request';

import { mapCustomerTicketDetail } from './support';

const base = pluginApiPath('idc-core', 'public/support/presales');
const verificationBase = pluginApiPath(
  'idc-notify',
  'public/support/presales',
);

export type PresalesDestinationKind = 'email' | 'sms';
export type PresalesOperation = 'create' | 'renew';
export type PresalesPresentationKind = 'accessible' | 'image';

interface PresalesVerificationBinding {
  browserNonce: string;
  destination: string;
  destinationKind: PresalesDestinationKind;
  locale: 'en-US' | 'zh-CN';
  operation: PresalesOperation;
  ticketNo?: string;
}

export interface PresalesHumanChallengeRequest extends PresalesVerificationBinding {
  presentationKind: PresalesPresentationKind;
}

export interface PresalesHumanChallenge {
  accessiblePrompt?: string;
  challengeNo: string;
  expiresAt: number;
  imageDataUrl?: string;
  kind: string;
  providerAction?: string;
  providerCode: string;
  siteKey?: string;
}

export interface PresalesHumanChallengeCompletionRequest extends PresalesVerificationBinding {
  answer?: string;
  challengeNo: string;
  providerToken?: string;
}

export interface PresalesHumanChallengeProof {
  expiresAt: number;
  proofToken: string;
}

export interface PresalesVerificationCodeRequest extends PresalesVerificationBinding {
  idempotencyKey: string;
  proofToken: string;
}

export interface PresalesVerificationCode {
  deliveryState: string;
  expiresAt: number;
  maskedDestination: string;
  retryAfterAt: number;
  verificationNo: string;
}

interface PresalesVerifiedOperationRequest {
  browserNonce: string;
  code: string;
  destination: string;
  destinationKind: PresalesDestinationKind;
  locale: 'en-US' | 'zh-CN';
  operationNo: string;
  verificationNo: string;
}

export interface PresalesTicketCreateRequest extends PresalesVerifiedOperationRequest {
  body: string;
  categoryNo: string;
  customFields: CustomerTicketCustomFieldSubmission[];
  subject: string;
}

export interface PresalesAccessRenewalRequest extends PresalesVerifiedOperationRequest {
  ticketNo: string;
}

export interface PresalesAccessResult {
  accessToken: string;
  expiresAt: number;
  linkDeliveryState: string;
  ticketNo: string;
}

interface PresalesDetailEnvelope {
  expiresAt: number;
  item: CustomerTicketDetailDTO;
}

export interface PresalesTicketDetail extends CustomerTicketDetail {
  accessExpiresAt: number;
}

export interface PresalesAttachmentUploadReservation {
  expiresAt: number;
  nextOffset: number;
  uploadNo: string;
  version: number;
}

export interface PresalesAttachmentUploadResult {
  fileName: string;
  mediaType: string;
  ready: boolean;
  sizeBytes: number;
  uploadNo: string;
  version: number;
}

function mapPresalesDetail(item: CustomerTicketDetailDTO, expiresAt: number) {
  const detail = mapCustomerTicketDetail(item);
  return {
    ...detail,
    accessExpiresAt: expiresAt,
  } satisfies PresalesTicketDetail;
}

export function listPresalesCategories(
  params: { cursor?: string; keyword?: string; limit?: number } = {},
) {
  return requestClient.get<{
    items: CustomerTicketCategory[];
    nextCursor: string;
  }>(pluginApiPath('idc-core', 'public/support/categories'), {
    params,
  });
}

export function listPresalesCustomFields() {
  return requestClient.get<{ items: CustomerSupportCustomFieldDefinition[] }>(
    `${base}/custom-fields`,
  );
}

export function requestPresalesHumanChallenge(
  input: PresalesHumanChallengeRequest,
) {
  return requestClient.post<PresalesHumanChallenge>(
    `${verificationBase}/human-challenges`,
    input,
    { silentErrorMessage: true },
  );
}

export function completePresalesHumanChallenge(
  input: PresalesHumanChallengeCompletionRequest,
) {
  return requestClient.post<PresalesHumanChallengeProof>(
    `${verificationBase}/human-challenge-completions`,
    input,
    { silentErrorMessage: true },
  );
}

export function requestPresalesVerificationCode(
  input: PresalesVerificationCodeRequest,
) {
  return requestClient.post<PresalesVerificationCode>(
    `${verificationBase}/verification-codes`,
    input,
    { silentErrorMessage: true },
  );
}

export function createPresalesTicket(input: PresalesTicketCreateRequest) {
  return requestClient.post<PresalesAccessResult>(
    `${verificationBase}/tickets`,
    input,
    { silentErrorMessage: true },
  );
}

export function renewPresalesAccess(input: PresalesAccessRenewalRequest) {
  return requestClient.post<PresalesAccessResult>(
    `${verificationBase}/access-renewals`,
    input,
    { silentErrorMessage: true },
  );
}

export async function verifyPresalesAccess(accessToken: string) {
  const result = await requestClient.post<PresalesDetailEnvelope>(
    `${base}/access-verifications`,
    { accessToken },
    { silentErrorMessage: true },
  );
  return mapPresalesDetail(result.item, result.expiresAt);
}

export async function replyPresales(
  accessToken: string,
  body: string,
  ticketVersion: number,
  uploadNos: string[] = [],
) {
  const result = await requestClient.post<PresalesDetailEnvelope>(
    `${base}/replies`,
    { accessToken, body, ticketVersion, uploadNos },
    { silentErrorMessage: true },
  );
  return mapPresalesDetail(result.item, result.expiresAt);
}

export async function closePresales(
  accessToken: string,
  ticketVersion: number,
) {
  const result = await requestClient.post<PresalesDetailEnvelope>(
    `${base}/closures`,
    { accessToken, ticketVersion },
    { silentErrorMessage: true },
  );
  return mapPresalesDetail(result.item, result.expiresAt);
}

export async function reopenPresales(
  accessToken: string,
  ticketVersion: number,
) {
  const result = await requestClient.post<PresalesDetailEnvelope>(
    `${base}/reopenings`,
    { accessToken, ticketVersion },
    { silentErrorMessage: true },
  );
  return mapPresalesDetail(result.item, result.expiresAt);
}

export async function ratePresales(
  accessToken: string,
  ticketVersion: number,
  roundNo: number,
  stars: number,
  feedback: string,
) {
  const result = await requestClient.post<PresalesDetailEnvelope>(
    `${base}/ratings`,
    { accessToken, feedback, roundNo, stars, ticketVersion },
    { silentErrorMessage: true },
  );
  return mapPresalesDetail(result.item, result.expiresAt);
}

export async function uploadPresalesReplyAttachments(
  accessToken: string,
  files: File[],
) {
  const uploadNos: string[] = [];
  for (const file of files) {
    const reservation =
      await requestClient.post<PresalesAttachmentUploadReservation>(
        `${base}/attachment-uploads`,
        {
          accessToken,
          fileName: file.name,
          sizeBytes: file.size,
        },
        { silentErrorMessage: true },
      );
    if (reservation.nextOffset !== 0) {
      throw new Error('unexpected anonymous attachment offset');
    }
    const uploaded = await requestClient.put<PresalesAttachmentUploadResult>(
      `${base}/attachment-uploads/${encodeURIComponent(reservation.uploadNo)}/content?accessToken=${encodeURIComponent(accessToken)}`,
      file,
      {
        headers: {
          'Content-Type': file.type || 'application/octet-stream',
        },
        silentErrorMessage: true,
        timeout: 120_000,
      },
    );
    if (!uploaded.ready || uploaded.uploadNo !== reservation.uploadNo) {
      throw new Error('anonymous attachment is not ready');
    }
    uploadNos.push(uploaded.uploadNo);
  }
  return uploadNos;
}

export function downloadPresalesAttachment(
  accessToken: string,
  attachment: Pick<CustomerTicketAttachment, 'attachmentNo'>,
) {
  return requestClient.download<Blob>(
    `${base}/attachments/${encodeURIComponent(attachment.attachmentNo)}/content`,
    {
      data: { accessToken },
      method: 'POST',
      silentErrorMessage: true,
    },
  );
}
