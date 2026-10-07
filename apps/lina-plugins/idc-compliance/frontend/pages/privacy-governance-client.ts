import { pluginApiPath, requestClient } from "#/api/request";

const base = "admin/compliance/privacy";
const pluginID = "idc-compliance";
const path = (suffix: string) => pluginApiPath(pluginID, `${base}/${suffix}`);

export interface AssessmentItem {
  assessmentNo: string;
  assessmentTypeCode: string;
  version: number;
  statusCode: string;
  purpose: string;
  necessity: string;
  impact: string;
  recipients: string;
  storageLocation: string;
  retentionDescription: string;
  rightsEntry: string;
  merchantRole: string;
  thirdPartyRole: string;
  processingInstruction: string;
  subcontractingBoundary: string;
  auditEvidenceDescription: string;
  aiAllowedFields: string[];
  crossBorderTransfer: false;
  rowVersion: number;
  updatedAt: number;
}
export interface RetentionItem {
  policyNo: string;
  dataTypeCode: string;
  retentionDays: number;
  version: number;
  createdAt: number;
}
export interface DeletionItem {
  taskNo: string;
  dataTypeCode: string;
  targetTypeCode: string;
  targetNo: string;
  statusCode: string;
  dueAt: number;
  holdReasonCode: string;
  version: number;
}
export interface IncidentEvent {
  eventNo: string;
  stageCode: string;
  safeNote: string;
  occurredAt: number;
}
export interface IncidentItem {
  incidentNo: string;
  statusCode: string;
  severityCode: string;
  sourceCode: string;
  safeSummary: string;
  exportBlocked: boolean;
  externalNotificationConfirmed: boolean;
  version: number;
  createdAt: number;
  timeline: IncidentEvent[];
}

export const listAssessments = () =>
  requestClient.get<{ items: AssessmentItem[] }>(path("assessments"), {
    params: { limit: 100 },
  });
export const saveAssessmentDraft = (
  input: Omit<
    AssessmentItem,
    | "assessmentNo"
    | "crossBorderTransfer"
    | "statusCode"
    | "updatedAt"
    | "version"
  >,
) =>
  requestClient.put<{ item: AssessmentItem }>(path("assessments/draft"), input);
export const publishAssessment = (input: {
  assessmentNo: string;
  expectedRowVersion: number;
  confirmed: boolean;
  idempotencyKey: string;
}) =>
  requestClient.post<{ item: AssessmentItem }>(
    path("assessments/publish"),
    input,
  );
export const listRetention = () =>
  requestClient.get<{ items: RetentionItem[] }>(path("retention-policies"));
export const setRetention = (input: {
  dataTypeCode: string;
  retentionDays: number;
  idempotencyKey: string;
}) =>
  requestClient.put<{ item: RetentionItem }>(path("retention-policies"), input);
export const listDeletion = () =>
  requestClient.get<{ items: DeletionItem[] }>(path("deletion-tasks"), {
    params: { limit: 200 },
  });
export const scanDeletion = () =>
  requestClient.post<{ created: number }>(path("deletion-tasks/scans"), {
    limit: 100,
  });
export const confirmDeletion = (item: DeletionItem, key: string) =>
  requestClient.post<{ item: DeletionItem }>(
    path(`deletion-tasks/${item.taskNo}/confirmations`),
    { confirmed: true, expectedVersion: item.version, idempotencyKey: key },
  );
export const listIncidents = () =>
  requestClient.get<{ items: IncidentItem[] }>(path("incidents"), {
    params: { limit: 100 },
  });
export const createIncident = (input: {
  severityCode: string;
  sourceCode: string;
  safeSummary: string;
  idempotencyKey: string;
}) => requestClient.post<{ item: IncidentItem }>(path("incidents"), input);
export const advanceIncident = (
  item: IncidentItem,
  nextStageCode: string,
  safeNote: string,
  key: string,
) =>
  requestClient.post<{ item: IncidentItem }>(
    path(`incidents/${item.incidentNo}/advances`),
    {
      confirmExternalNotification: nextStageCode === "notification_confirmed",
      expectedVersion: item.version,
      idempotencyKey: key,
      nextStageCode,
      safeNote,
    },
  );
export const generateSupportExport = (key: string) =>
  requestClient.post<{
    content: string;
    contentSha256: string;
    exportNo: string;
    fileName: string;
  }>(path("support-exports"), {
    idempotencyKey: key,
    supportAgreementConfirmed: true,
  });
