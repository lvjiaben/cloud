import type { PluginHostMountContext } from '../host-context';

import { createPluginRequest } from '../runtime/request';
import type { PluginRequestOptions } from '../runtime/request';

const supportBase = '/x/idc-core/api/v1/admin/support';

export type TicketSummary = {
  assigneeUserId?: number;
  baseStage: string;
  categoryNameEnUS?: string;
  categoryNameZhCN?: string;
  categoryNo?: string;
  createdAt: number;
  currentRound: number;
  customerDisplayLabel?: string;
  customerNo?: string;
  customerStage: string;
  customStatusNo?: string;
  departmentNo?: string;
  followUp: boolean;
  lastActivityAt: number;
  priority: string;
  queue: string;
  relatedEntitlement?: TicketRelatedEntitlement;
  slaRisk: 'at_risk' | 'breached' | 'none';
  slaConfigured: boolean;
  subject: string;
  ticketNo: string;
  type: string;
  updatedAt: number;
  version: number;
  visibility: string;
};

export type TicketAttachment = {
  attachmentNo: string;
  createdAt: number;
  fileName: string;
  hidden: boolean;
  mediaType: string;
  messageNo?: string;
  sizeBytes: number;
  visibility: string;
};

export type TicketMessage = {
  attachments: TicketAttachment[];
  authorType: string;
  authorUserId?: number;
  automated: boolean;
  body: string;
  createdAt: number;
  hidden: boolean;
  messageNo: string;
  type: string;
  visibility: string;
};

export type TicketSLARound = {
  calendarMode?: string;
  calendarNo?: string;
  firstResponseBreached: boolean;
  firstResponseDueAt?: number;
  firstRespondedAt?: number;
  pausedSeconds: number;
  resolutionBreached: boolean;
  resolutionDueAt?: number;
  resolvedAt?: number;
  round: number;
  ruleNo?: string;
  stoppedAt?: number;
};

export type TicketAssignment = {
  action: string;
  fromDepartmentNo?: string;
  occurredAt: number;
  reason?: string;
  toDepartmentNo?: string;
};

export type TicketRelation = {
  createdAt: number;
  reason?: string;
  sourceTicketNo: string;
  targetTicketNo: string;
  type: string;
};

export type TicketEscalation = {
  createdAt: number;
  reason: string;
};

export type TicketStatusHistory = {
  customStatusNo?: string;
  fromStage?: string;
  occurredAt: number;
  reason?: string;
  reasonCode?: string;
  source: string;
  toStage: string;
};

export type TicketStatusTransitionOption = {
  baseStage: string;
  nameEnUS: string;
  nameZhCN: string;
  requiresReason: boolean;
  statusNo: string;
  version: number;
};

export type MergeCandidate = {
  stage: string;
  subject: string;
  ticketNo: string;
  version: number;
};

export type TicketDetail = {
  assignmentHistory: TicketAssignment[];
  attachments: TicketAttachment[];
  availableActions: string[];
  availableStatusTransitions: TicketStatusTransitionOption[];
  escalations: TicketEscalation[];
  internalNotes: TicketMessage[];
  publicMessages: TicketMessage[];
  relations: TicketRelation[];
  slaRounds: TicketSLARound[];
  statusHistory: TicketStatusHistory[];
  ticket: TicketSummary;
  customFields: TicketCustomFieldValue[];
};

export type TicketListQuery = {
  assigneeUserId?: number;
  cursor?: string;
  departmentNo?: string;
  followUp?: boolean;
  limit?: number;
  priority?: string;
  query?: string;
  stage?: string;
  status?: string;
  type?: string;
};

export type TicketListResult = {
  nextCursor: string;
  pageActions: string[];
  tickets: TicketSummary[];
};

export type DirectoryItem = {
  assignmentMode?: string;
  baseStage?: string;
  body?: string;
  categoryNo?: string;
  departmentNo?: string;
  locale?: string;
  nameEnUS?: string;
  nameZhCN?: string;
  parentDepartmentNo?: string;
  presetNo?: string;
  priority?: string;
  status?: string;
  statusNo?: string;
  title?: string;
  version?: number;
};

export type AttachmentUpload = {
  expiresAt?: number;
  fileName?: string;
  mediaType?: string;
  nextOffset?: number;
  ready?: boolean;
  sizeBytes?: number;
  uploadNo: string;
  version: number;
};

export type AutomationPolicy = {
  autoCloseDays: number;
  reopenDays: number;
  version: number;
};

export type SupportSettingStatus = 'active' | 'disabled';

export type SupportCustomFieldType =
  | 'boolean'
  | 'date'
  | 'multi_line'
  | 'multi_select'
  | 'number'
  | 'single_line'
  | 'single_select';

export type SupportCustomFieldVisibility =
  | 'customer_visible'
  | 'internal_only';

export type SupportCustomFieldOption = {
  nameEnUS: string;
  nameZhCN: string;
  optionNo: string;
  sortOrder: number;
  status: SupportSettingStatus;
  version: number;
};

export type SupportCustomFieldDefinition = {
  appliesCustomer: boolean;
  appliesInternal: boolean;
  appliesPresales: boolean;
  descriptionEnUS?: string;
  descriptionZhCN?: string;
  fieldNo: string;
  fieldType: SupportCustomFieldType;
  nameEnUS: string;
  nameZhCN: string;
  options: SupportCustomFieldOption[];
  required: boolean;
  sortOrder: number;
  status: SupportSettingStatus;
  version: number;
  visibility: SupportCustomFieldVisibility;
};

export type SupportCustomFieldOptionInput = {
  expectedVersion: number;
  nameEnUS: string;
  nameZhCN: string;
  optionNo: string;
  sortOrder: number;
  status: SupportSettingStatus;
};

export type SupportCustomFieldDefinitionInput = Omit<
  SupportCustomFieldDefinition,
  'fieldNo' | 'options' | 'version'
> & {
  options: SupportCustomFieldOptionInput[];
};

export type TicketCustomFieldSubmission = {
  booleanValue?: boolean;
  dateValue?: string;
  fieldNo: string;
  numberValue?: string;
  optionNos?: string[];
  textValue?: string;
};

export type TicketCustomFieldValue = {
  definitionVersion: number;
  descriptionEnUS?: string;
  descriptionZhCN?: string;
  fieldNo: string;
  fieldType: SupportCustomFieldType;
  nameEnUS: string;
  nameZhCN: string;
  normalizedValue: string;
  required: boolean;
  selectedOptions: SupportCustomFieldOption[];
  sortOrder: number;
  visibility: SupportCustomFieldVisibility;
};

export type TicketRelatedEntitlement = {
  currentAvailable: boolean;
  currentNameEnUS?: string;
  currentNameZhCN?: string;
  currentStatus?: string;
  currentVersion?: number;
  entitlementNo: string;
  snapshotNameEnUS: string;
  snapshotNameZhCN: string;
  snapshotStatus: string;
  snapshotVersion: number;
};

export type SupportDepartmentSetting = {
  assignmentMode: 'least_unresolved' | 'manual_claim' | 'round_robin';
  departmentNo: string;
  nameEnUS: string;
  nameZhCN: string;
  parentDepartmentNo?: string;
  status: SupportSettingStatus;
  version: number;
};

export type SupportIssueCategorySetting = {
  categoryNo: string;
  departmentNo?: string;
  nameEnUS: string;
  nameZhCN: string;
  priority: 'high' | 'low' | 'normal' | 'urgent';
  sortOrder: number;
  status: SupportSettingStatus;
  version: number;
};

export type SupportCustomStatusSetting = {
  baseStage:
    | 'closed'
    | 'in_progress'
    | 'new'
    | 'resolved'
    | 'waiting_customer'
    | 'waiting_upstream';
  nameEnUS: string;
  nameZhCN: string;
  sortOrder: number;
  status: SupportSettingStatus;
  statusNo: string;
  version: number;
};

export type SupportReplyPresetSetting = {
  body: string;
  departmentNo: string;
  locale: 'en-US' | 'zh-CN';
  presetNo: string;
  sortOrder: number;
  status: SupportSettingStatus;
  title: string;
  version: number;
};

export type SupportSLACalendarSetting = {
  calendarNo: string;
  mode: 'business_hours' | 'continuous';
  name: string;
  status: SupportSettingStatus;
  timezone?: string;
  version: number;
  workdays: number[];
  workEndMinute?: number;
  workStartMinute?: number;
};

export type SupportSLAHolidaySetting = {
  calendarNo: string;
  date: string;
  name: string;
};

export type SupportSLARuleSetting = {
  calendarNo: string;
  categoryNo?: string;
  departmentNo: string;
  firstResponseMinutes?: number;
  nearDueMinutes: number[];
  priority: 'high' | 'low' | 'normal' | 'urgent';
  resolutionMinutes?: number;
  ruleNo: string;
  status: SupportSettingStatus;
  version: number;
};

export type SupportDepartmentSettingInput = Omit<
  SupportDepartmentSetting,
  'departmentNo' | 'version'
>;
export type SupportIssueCategorySettingInput = Omit<
  SupportIssueCategorySetting,
  'categoryNo' | 'version'
>;
export type SupportCustomStatusSettingInput = Omit<
  SupportCustomStatusSetting,
  'statusNo' | 'version'
>;
export type SupportReplyPresetSettingInput = Omit<
  SupportReplyPresetSetting,
  'presetNo' | 'version'
>;
export type SupportSLACalendarSettingInput = Omit<
  SupportSLACalendarSetting,
  'calendarNo' | 'version'
>;
export type SupportSLARuleSettingInput = Omit<
  SupportSLARuleSetting,
  'ruleNo' | 'version'
>;

export type InternalTicketCreateInput = {
  body: string;
  categoryNo: string;
  customFields: TicketCustomFieldSubmission[];
  departmentNo: string;
  priority: string;
  subject: string;
  uploadNos: string[];
};

export type SupportUpstreamChannel = {
  authMode: 'resource_login' | 'zjmf_api_login';
  baseUrl: string;
  configured: boolean;
  credentialMaskedTail: string;
  status: 'disabled' | 'enabled';
  updatedAt: number;
  version: number;
};
export type SupportUpstreamDepartmentMap = {
  departmentNo: string;
  status: 'active' | 'disabled';
  upstreamDepartmentId: number;
  upstreamDepartmentName: string;
  version: number;
};
export type SupportUpstreamEntitlementMap = {
  entitlementNo: string;
  status: 'active' | 'disabled';
  upstreamReference: string;
  version: number;
};

export type SupportUpstreamPreview = {
  attachmentNos: string[];
  content: string;
  departmentId: number;
  forwardNo: string;
  messageNos: string[];
  previewDigest: string;
  selectionDigest: string;
  entitlementUpstreamRef: string;
  ticketNo: string;
  title: string;
  version: number;
};

export type SupportUpstreamForward = {
  attemptCount: number;
  evidenceCode: string;
  forwardNo: string;
  providerTicketReference: string;
  previewDigest: string;
  resultClassification: 'accepted' | 'definitive_not_received' | 'result_unknown';
  retryEligible: boolean;
  status: string;
  ticketNo: string;
  updatedAt: number;
  version: number;
};

function queryString(query: Record<string, boolean | number | string | undefined>) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== '') {
      params.set(key, String(value));
    }
  }
  const encoded = params.toString();
  return encoded ? `?${encoded}` : '';
}

export function createSupportAPI(
  currentContext: () => PluginHostMountContext,
) {
  const request = createPluginRequest(currentContext);
  async function settingItems<T>(resource: string) {
    const result = await request<{ items: T[] }>(
      `${supportBase}/settings/${resource}`,
    );
    return result.items;
  }
  async function settingItem<T>(
    resource: string,
    options: PluginRequestOptions,
  ) {
    const result = await request<{ item: T }>(
      `${supportBase}/settings/${resource}`,
      options,
    );
    return result.item;
  }
  return {
    async action(ticketNo: string, action: string, body: unknown) {
      const result = await request<{ item: TicketDetail }>(
        `${supportBase}/tickets/${encodeURIComponent(ticketNo)}/${action}`,
        { body, method: 'POST' },
      );
      return result.item;
    },
    async upstreamChannel() {
      const result = await request<{ item: SupportUpstreamChannel }>(
        `${supportBase}/settings/upstream-channel`,
      );
      return result.item;
    },
    async configureUpstreamChannel(body: {
      authMode: SupportUpstreamChannel['authMode'];
      baseUrl: string;
      expectedVersion: number;
      operationNo: string;
      password: string;
      reason: string;
      status: SupportUpstreamChannel['status'];
      username: string;
    }) {
      const result = await request<{ item: SupportUpstreamChannel }>(
        `${supportBase}/settings/upstream-channel`,
        { body, method: 'PUT' },
      );
      return result.item;
    },
    async upstreamDepartmentMap(departmentNo: string) {
      const result = await request<{ item: null | SupportUpstreamDepartmentMap }>(
        `${supportBase}/settings/upstream-departments/${encodeURIComponent(departmentNo)}`,
      );
      return result.item;
    },
    async saveUpstreamDepartmentMap(
      departmentNo: string,
      body: {
        expectedVersion: number;
        status: 'active' | 'disabled';
        upstreamDepartmentId: number;
        upstreamDepartmentName: string;
      },
    ) {
      const result = await request<{ item: SupportUpstreamDepartmentMap }>(
        `${supportBase}/settings/upstream-departments/${encodeURIComponent(departmentNo)}`,
        { body, method: 'PUT' },
      );
      return result.item;
    },
    async upstreamEntitlementMap(entitlementNo: string) {
      const result = await request<{
        item: null | SupportUpstreamEntitlementMap;
      }>(
        `${supportBase}/settings/upstream-entitlements/${encodeURIComponent(entitlementNo)}`,
      );
      return result.item;
    },
    async saveUpstreamEntitlementMap(
      entitlementNo: string,
      body: {
        expectedVersion: number;
        status: 'active' | 'disabled';
        upstreamReference: string;
      },
    ) {
      const result = await request<{ item: SupportUpstreamEntitlementMap }>(
        `${supportBase}/settings/upstream-entitlements/${encodeURIComponent(entitlementNo)}`,
        { body, method: 'PUT' },
      );
      return result.item;
    },
    async prepareUpstream(
      ticketNo: string,
      body: {
        attachmentNos: string[];
        messageNos: string[];
        operationNo: string;
        ticketVersion: number;
      },
    ) {
      const result = await request<{ item: SupportUpstreamPreview }>(
        `${supportBase}/tickets/${encodeURIComponent(ticketNo)}/upstream-previews`,
        { body, method: 'POST' },
      );
      return result.item;
    },
    async confirmUpstream(
      forwardNo: string,
      body: {
        expectedVersion: number;
        operationNo: string;
        previewDigest: string;
        reason: string;
      },
    ) {
      const result = await request<{ item: SupportUpstreamForward }>(
        `${supportBase}/upstream-forwards/${encodeURIComponent(forwardNo)}/confirmations`,
        { body, method: 'POST' },
      );
      return result.item;
    },
    async retryUpstream(
      forwardNo: string,
      body: {
        expectedVersion: number;
        operationNo: string;
        previewDigest: string;
        reason: string;
      },
    ) {
      const result = await request<{ item: SupportUpstreamForward }>(
        `${supportBase}/upstream-forwards/${encodeURIComponent(forwardNo)}/retries`,
        { body, method: 'POST' },
      );
      return result.item;
    },
    async upstreamForward(forwardNo: string) {
      const result = await request<{ item: SupportUpstreamForward }>(
        `${supportBase}/upstream-forwards/${encodeURIComponent(forwardNo)}`,
      );
      return result.item;
    },
    async ticketUpstreamForwards(ticketNo: string) {
      const result = await request<{ items: SupportUpstreamForward[] }>(
        `${supportBase}/tickets/${encodeURIComponent(ticketNo)}/upstream-forwards`,
      );
      return result.items;
    },
    syncUpstreamReplies(forwardNo: string, expectedVersion: number) {
      return request<{ added: number }>(
        `${supportBase}/upstream-forwards/${encodeURIComponent(forwardNo)}/reply-syncs`,
        { body: { expectedVersion }, method: 'POST' },
      );
    },
    async publishUpstreamReply(
      ticketNo: string,
      localMessageNo: string,
      body: { reason: string; ticketVersion: number },
    ) {
      const result = await request<{ item: TicketDetail }>(
        `${supportBase}/tickets/${encodeURIComponent(ticketNo)}/upstream-replies/${encodeURIComponent(localMessageNo)}/publications`,
        { body, method: 'POST' },
      );
      return result.item;
    },
    createAttachmentUpload(
      ticketNo: string,
      body: {
        context: 'internal_note' | 'public_reply';
        fileName: string;
        sizeBytes: number;
      },
    ) {
      return request<AttachmentUpload>(
        `${supportBase}/tickets/${encodeURIComponent(ticketNo)}/${body.context === 'internal_note' ? 'internal-note-attachment-uploads' : 'attachment-uploads'}`,
        { body, method: 'POST' },
      );
    },
    createInitialAttachmentUpload(body: {
      fileName: string;
      sizeBytes: number;
    }) {
      return request<AttachmentUpload>(
        `${supportBase}/internal-ticket-attachment-uploads`,
        { body, method: 'POST' },
      );
    },
    async createTicket(body: InternalTicketCreateInput) {
      const result = await request<{ item: TicketDetail }>(
        `${supportBase}/tickets`,
        { body, method: 'POST' },
      );
      return result.item;
    },
    async automationPolicy() {
      const result = await request<{ policy: AutomationPolicy }>(
        `${supportBase}/settings/automation-policy`,
      );
      return result.policy;
    },
    createCustomStatus(body: SupportCustomStatusSettingInput) {
      return settingItem<SupportCustomStatusSetting>('statuses', {
        body,
        method: 'POST',
      });
    },
    createCustomField(body: SupportCustomFieldDefinitionInput) {
      return settingItem<SupportCustomFieldDefinition>('custom-fields', {
        body,
        method: 'POST',
      });
    },
    createDepartment(body: SupportDepartmentSettingInput) {
      return settingItem<SupportDepartmentSetting>('departments', {
        body,
        method: 'POST',
      });
    },
    createIssueCategory(body: SupportIssueCategorySettingInput) {
      return settingItem<SupportIssueCategorySetting>('issue-categories', {
        body,
        method: 'POST',
      });
    },
    createReplyPreset(body: SupportReplyPresetSettingInput) {
      return settingItem<SupportReplyPresetSetting>('reply-presets', {
        body,
        method: 'POST',
      });
    },
    createSLACalendar(body: SupportSLACalendarSettingInput) {
      return settingItem<SupportSLACalendarSetting>('sla-calendars', {
        body,
        method: 'POST',
      });
    },
    createSLARule(body: SupportSLARuleSettingInput) {
      return settingItem<SupportSLARuleSetting>('sla-rules', {
        body,
        method: 'POST',
      });
    },
    departments() {
      return request<{ departments: DirectoryItem[] }>(
        `${supportBase}/departments`,
      );
    },
    async downloadAttachment(attachmentNo: string) {
      const context = currentContext();
      const token = context.accessToken.trim();
      const response = await fetch(
        new URL(
          `${supportBase}/attachments/${encodeURIComponent(attachmentNo)}/content`,
          globalThis.location.origin,
        ),
        {
          credentials: 'same-origin',
          headers: token ? { Authorization: `Bearer ${token}` } : {},
        },
      );
      if (!response.ok) throw new Error('attachment_download_failed');
      return response.blob();
    },
    issueCategories() {
      return request<{ issueCategories: DirectoryItem[] }>(
        `${supportBase}/issue-categories`,
      );
    },
    customStatusSettings() {
      return settingItems<SupportCustomStatusSetting>('statuses');
    },
    customFieldSettings() {
      return settingItems<SupportCustomFieldDefinition>('custom-fields');
    },
    departmentSettings() {
      return settingItems<SupportDepartmentSetting>('departments');
    },
    disableCustomStatus(statusNo: string, expectedVersion: number) {
      return settingItem<SupportCustomStatusSetting>(
        `statuses/${encodeURIComponent(statusNo)}`,
        { body: { expectedVersion }, method: 'DELETE' },
      );
    },
    disableCustomField(fieldNo: string, expectedVersion: number) {
      return settingItem<SupportCustomFieldDefinition>(
        `custom-fields/${encodeURIComponent(fieldNo)}`,
        { body: { expectedVersion }, method: 'DELETE' },
      );
    },
    disableDepartment(departmentNo: string, expectedVersion: number) {
      return settingItem<SupportDepartmentSetting>(
        `departments/${encodeURIComponent(departmentNo)}`,
        { body: { expectedVersion }, method: 'DELETE' },
      );
    },
    disableIssueCategory(categoryNo: string, expectedVersion: number) {
      return settingItem<SupportIssueCategorySetting>(
        `issue-categories/${encodeURIComponent(categoryNo)}`,
        { body: { expectedVersion }, method: 'DELETE' },
      );
    },
    disableReplyPreset(presetNo: string, expectedVersion: number) {
      return settingItem<SupportReplyPresetSetting>(
        `reply-presets/${encodeURIComponent(presetNo)}`,
        { body: { expectedVersion }, method: 'DELETE' },
      );
    },
    disableSLACalendar(calendarNo: string, expectedVersion: number) {
      return settingItem<SupportSLACalendarSetting>(
        `sla-calendars/${encodeURIComponent(calendarNo)}`,
        { body: { expectedVersion }, method: 'DELETE' },
      );
    },
    disableSLARule(ruleNo: string, expectedVersion: number) {
      return settingItem<SupportSLARuleSetting>(
        `sla-rules/${encodeURIComponent(ruleNo)}`,
        { body: { expectedVersion }, method: 'DELETE' },
      );
    },
    issueCategorySettings() {
      return settingItems<SupportIssueCategorySetting>('issue-categories');
    },
    internalCustomFields() {
      return request<{ items: SupportCustomFieldDefinition[] }>(
        `${supportBase}/custom-fields/internal`,
      );
    },
    list(query: TicketListQuery) {
      return request<TicketListResult>(
        `${supportBase}/tickets${queryString(query)}`,
      );
    },
    mergeCandidates(ticketNo: string, cursor = '', global = false) {
      return request<{ items: MergeCandidate[]; nextCursor: string }>(
        `${supportBase}/tickets/${encodeURIComponent(ticketNo)}/${global ? 'global-merge-candidates' : 'merge-candidates'}${queryString({ cursor, limit: 50 })}`,
      );
    },
    replyPresetSettings() {
      return settingItems<SupportReplyPresetSetting>('reply-presets');
    },
    replyPresets() {
      return request<{ replyPresets: DirectoryItem[] }>(
        `${supportBase}/reply-presets${queryString({ locale: currentContext().locale })}`,
      );
    },
    statuses() {
      return request<{ statuses: DirectoryItem[] }>(
        `${supportBase}/statuses`,
      );
    },
    slaCalendarSettings() {
      return settingItems<SupportSLACalendarSetting>('sla-calendars');
    },
    slaHolidays(calendarNo: string) {
      return settingItems<SupportSLAHolidaySetting>(
        `sla-calendars/${encodeURIComponent(calendarNo)}/holidays`,
      );
    },
    slaRuleSettings() {
      return settingItems<SupportSLARuleSetting>('sla-rules');
    },
    async deleteSLAHoliday(calendarNo: string, date: string) {
      await request<Record<string, never>>(
        `${supportBase}/settings/sla-calendars/${encodeURIComponent(calendarNo)}/holidays/${encodeURIComponent(date)}`,
        { method: 'DELETE' },
      );
    },
    putSLAHoliday(calendarNo: string, date: string, name: string) {
      return settingItem<SupportSLAHolidaySetting>(
        `sla-calendars/${encodeURIComponent(calendarNo)}/holidays/${encodeURIComponent(date)}`,
        { body: { name }, method: 'PUT' },
      );
    },
    async updateAutomationPolicy(body: {
      autoCloseDays: number;
      expectedVersion: number;
      reopenDays: number;
    }) {
      const result = await request<{ policy: AutomationPolicy }>(
        `${supportBase}/settings/automation-policy`,
        { body, method: 'PUT' },
      );
      return result.policy;
    },
    async ticket(ticketNo: string) {
      const result = await request<{ item: TicketDetail }>(
        `${supportBase}/tickets/${encodeURIComponent(ticketNo)}`,
      );
      return result.item;
    },
    async transitionStatus(
      ticketNo: string,
      body: {
        customStatusNo: string;
        reason: string;
        ticketVersion: number;
      },
    ) {
      const result = await request<{ item: TicketDetail }>(
        `${supportBase}/tickets/${encodeURIComponent(ticketNo)}/status-transitions`,
        { body, method: 'POST' },
      );
      return result.item;
    },
    updateCustomStatus(
      statusNo: string,
      body: SupportCustomStatusSettingInput & { expectedVersion: number },
    ) {
      return settingItem<SupportCustomStatusSetting>(
        `statuses/${encodeURIComponent(statusNo)}`,
        { body, method: 'PUT' },
      );
    },
    updateCustomField(
      fieldNo: string,
      body: SupportCustomFieldDefinitionInput & { expectedVersion: number },
    ) {
      return settingItem<SupportCustomFieldDefinition>(
        `custom-fields/${encodeURIComponent(fieldNo)}`,
        { body, method: 'PUT' },
      );
    },
    updateDepartment(
      departmentNo: string,
      body: SupportDepartmentSettingInput & { expectedVersion: number },
    ) {
      return settingItem<SupportDepartmentSetting>(
        `departments/${encodeURIComponent(departmentNo)}`,
        { body, method: 'PUT' },
      );
    },
    updateIssueCategory(
      categoryNo: string,
      body: SupportIssueCategorySettingInput & { expectedVersion: number },
    ) {
      return settingItem<SupportIssueCategorySetting>(
        `issue-categories/${encodeURIComponent(categoryNo)}`,
        { body, method: 'PUT' },
      );
    },
    updateReplyPreset(
      presetNo: string,
      body: SupportReplyPresetSettingInput & { expectedVersion: number },
    ) {
      return settingItem<SupportReplyPresetSetting>(
        `reply-presets/${encodeURIComponent(presetNo)}`,
        { body, method: 'PUT' },
      );
    },
    updateSLACalendar(
      calendarNo: string,
      body: SupportSLACalendarSettingInput & { expectedVersion: number },
    ) {
      return settingItem<SupportSLACalendarSetting>(
        `sla-calendars/${encodeURIComponent(calendarNo)}`,
        { body, method: 'PUT' },
      );
    },
    updateSLARule(
      ruleNo: string,
      body: SupportSLARuleSettingInput & { expectedVersion: number },
    ) {
      return settingItem<SupportSLARuleSetting>(
        `sla-rules/${encodeURIComponent(ruleNo)}`,
        { body, method: 'PUT' },
      );
    },
    uploadAttachmentContent(
      uploadNo: string,
      file: File,
      context: 'internal_initial' | 'internal_note' | 'public_reply',
    ) {
      const collection =
        context === 'internal_initial'
          ? 'internal-ticket-attachment-uploads'
          : context === 'internal_note'
            ? 'internal-note-attachment-uploads'
            : 'ticket-attachment-uploads';
      return request<AttachmentUpload>(
        `${supportBase}/${collection}/${encodeURIComponent(uploadNo)}/content`,
        {
          headers: { 'Content-Type': file.type || 'application/octet-stream' },
          method: 'PUT',
          rawBody: file,
          timeoutMs: 120_000,
        },
      );
    },
  };
}
