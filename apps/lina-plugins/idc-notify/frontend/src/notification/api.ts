import type { PluginHostMountContext } from '../host-context';
import { createPluginRequest } from '../runtime/request';

const notificationAdminBase = '/x/idc-notify/api/v1/admin/notifications';

export type ChannelConfiguration = {
  smtp?: { host: string; port: number; security: 'implicit_tls' | 'starttls'; username: string; fromAddress: string; fromName: string };
  aliyunSms?: { signName: string };
  smsBao?: { signName: string; productId: string };
  tencentSms?: { region: string; smsSdkAppId: string; signName: string };
};
export type SMSTemplateMapping = { providerCode: string; templateId: string; variableOrder: string[]; dataMapping: Record<string, string> };
export type Channel = { code: string; kind: string; providerCode: string; displayName: string; status: string; balanceState: string; configured: boolean; credentialMaskedTail: string; configurationGeneration: number; pendingQueryCount: number; version: number; updatedAt: number; configuration: ChannelConfiguration };
export type EventItem = { code: string; topic: string; objectType: string; requiredVariables: string[]; optionalVariables: string[]; securityRequired: boolean; version: number };
export type EventPolicy = { eventCode: string; topic: string; recipientKind: 'customer_contact' | 'staff_user' | 'verified_destination'; channelCode: string; enabled: boolean; version: number; updatedAt: number };
export type TemplateItem = { templateNo: string; version: number; eventCode: string; channelCode: string; locale: string; subject: string; body: string; status: string; createdBy: number; createdAt: number; sms?: SMSTemplateMapping };
export type Delivery = { taskNo: string; chainNo: string; previousTaskNo: string; eventCode: string; topic: string; channelCode: string; locale: string; recipientMask: string; businessObjectType: string; businessObjectNo: string; status: string; resultClass: string; retrySequence: number; nextAttemptAt?: number; queryDeadline?: number; queryDeadlineSource: string; manualResendCount: number; isChainTail: boolean; version: number; queuedAt: number; completedAt?: number };
export type Eligibility = { taskNo: string; taskVersion: number; allowed: boolean; reasons: string[]; evaluatedAt: number; nextAllowedAt?: number; remainingCount: number; replacedByTaskNo: string };
export type ChannelTestResult = { operationNo: string; channelCode: string; targetMask: string; resultClass: string; reused: boolean; completedAt: number };

export function createNotificationAPI(currentContext: () => PluginHostMountContext) {
  const request = createPluginRequest(currentContext);
  return {
    channels: () => request<{ items: Channel[] }>(`${notificationAdminBase}/channels`),
    configureSMTP: (item: Channel, value: { host: string; port: number; security: string; username: string; password: string; fromAddress: string; fromName: string }) => request<{ item: Channel }>(`${notificationAdminBase}/channels/email/smtp-configuration`, { method: 'PUT', body: { ...value, expectedVersion: item.version, idempotencyKey: `smtp-${crypto.randomUUID()}` } }),
    configureAliyunSMS: (item: Channel, value: { signName: string; accessKeyId: string; accessKeySecret: string }) => request<{ item: Channel }>(`${notificationAdminBase}/channels/sms/aliyun-configuration`, { method: 'PUT', body: { ...value, expectedVersion: item.version, idempotencyKey: `aliyun-sms-${crypto.randomUUID()}` } }),
    configureSMSBao: (item: Channel, value: { signName: string; productId: string; username: string; apiKey: string }) => request<{ item: Channel }>(`${notificationAdminBase}/channels/sms/smsbao-configuration`, { method: 'PUT', body: { ...value, expectedVersion: item.version, idempotencyKey: `smsbao-${crypto.randomUUID()}` } }),
    configureTencentSMS: (item: Channel, value: { region: string; smsSdkAppId: string; signName: string; secretId: string; secretKey: string }) => request<{ item: Channel }>(`${notificationAdminBase}/channels/sms/tencent-cloud-configuration`, { method: 'PUT', body: { ...value, expectedVersion: item.version, idempotencyKey: `sms-${crypto.randomUUID()}` } }),
    testChannel: (item: Channel, value: { target: string; templateNo: string; templateVersion: number; variables: Record<string, string> }) => request<{ item: ChannelTestResult }>(`${notificationAdminBase}/channels/${item.code}/tests`, { method: 'POST', body: { ...value, expectedVersion: item.version, idempotencyKey: `channel-test-${crypto.randomUUID()}`, confirmed: true } }),
    disableChannel: (item: Channel) => request<{ item: Channel }>(`${notificationAdminBase}/channels/${item.code}/disablement`, { method: 'POST', body: { expectedVersion: item.version, idempotencyKey: `disable-${crypto.randomUUID()}`, confirmed: true } }),
    enableChannel: (item: Channel) => request<{ item: Channel }>(`${notificationAdminBase}/channels/${item.code}/enablement`, { method: 'POST', body: { expectedVersion: item.version, idempotencyKey: `enable-${crypto.randomUUID()}`, confirmed: true } }),
    clearChannel: (item: Channel) => request<{ item: Channel }>(`${notificationAdminBase}/channels/${item.code}/configuration-clearance`, { method: 'POST', body: { expectedVersion: item.version, idempotencyKey: `clear-${crypto.randomUUID()}`, confirmed: true } }),
    events: () => request<{ items: EventItem[] }>(`${notificationAdminBase}/events`),
    eventPolicies: () => request<{ items: EventPolicy[] }>(`${notificationAdminBase}/event-policies?limit=100`),
    updateEventPolicy: (item: EventPolicy, enabled: boolean) => request<{ item: EventPolicy }>(`${notificationAdminBase}/event-policies/${encodeURIComponent(item.eventCode)}/${item.recipientKind}/${item.channelCode}`, { method: 'PUT', body: { enabled, expectedVersion: item.version, idempotencyKey: `event-policy-${crypto.randomUUID()}` } }),
    templates: () => request<{ items: TemplateItem[] }>(`${notificationAdminBase}/templates?limit=100`),
    saveTemplate: (item: Partial<TemplateItem> & { eventCode: string; channelCode: string; locale: string; subject: string; body: string; status: string; version: number }) => request<{ item: TemplateItem }>(`${notificationAdminBase}/templates`, { method: 'PUT', body: { templateNo: item.templateNo ?? '', eventCode: item.eventCode, channelCode: item.channelCode, locale: item.locale, subject: item.subject, body: item.body, status: item.status, expectedVersion: item.version, idempotencyKey: `template-${crypto.randomUUID()}`, sms: item.channelCode === 'sms' ? item.sms : undefined } }),
    previewTemplate: (item: TemplateItem, variables: Record<string, string>) => request<{ subject: string; body: string }>(`${notificationAdminBase}/templates/${item.templateNo}/preview`, { method: 'POST', body: { version: item.version, variables } }),
    deliveries: (cursor = '') => request<{ items: Delivery[]; nextCursor: string }>(`${notificationAdminBase}/deliveries?limit=30${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`),
    delivery: (taskNo: string) => request<{ item: Delivery }>(`${notificationAdminBase}/deliveries/${taskNo}`),
    eligibility: (taskNo: string) => request<{ item: Eligibility }>(`${notificationAdminBase}/deliveries/${taskNo}/resend-eligibility`),
    resend: (item: Delivery, reason: string) => request<{ item: Delivery }>(`${notificationAdminBase}/deliveries/${item.taskNo}/resends`, { method: 'POST', body: { expectedVersion: item.version, operationNo: `resend-${crypto.randomUUID()}`, reason, confirmed: true } }),
    closeUnknown: (item: Delivery, reason: string) => request<{ item: Delivery }>(`${notificationAdminBase}/deliveries/${item.taskNo}/unknown-closures`, { method: 'POST', body: { expectedVersion: item.version, operationNo: `close-${crypto.randomUUID()}`, reason, confirmed: true } }),
  };
}
