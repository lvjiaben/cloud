import { pluginApiPath, requestClient } from '#/api/request';

export type NotificationTopic =
  | 'account'
  | 'billing'
  | 'marketing'
  | 'security'
  | 'service'
  | 'support';

export type NotificationChannel =
  | 'email'
  | 'in_app'
  | 'sms'
  | 'wechat_official';

export type NotificationDeliveryStatus =
  | 'cancelled'
  | 'delivered'
  | 'failed'
  | 'manual_review'
  | 'manual_unknown_closed'
  | 'preference_blocked'
  | 'queued'
  | 'result_unknown'
  | 'retry_wait'
  | 'sending'
  | 'submitted'
  | 'succeeded'
  | 'template_error';

export interface CustomerNotificationDelivery {
  channelCode: NotificationChannel;
  completedAt?: number;
  queuedAt: number;
  status: NotificationDeliveryStatus;
  taskNo: string;
  topic: NotificationTopic;
}

export interface CustomerNotificationDeliveryPage {
  items: CustomerNotificationDelivery[];
  nextCursor: string;
}

export interface CustomerNotificationContactPreference {
  channelCode: NotificationChannel;
  contactNo: string;
  enabled: boolean;
  topic: NotificationTopic;
  version: number;
}

export interface CustomerNotificationMarketingConsent {
  channelCode: NotificationChannel;
  contactNo: string;
  locale: 'en-US' | 'zh-CN';
  policyReference: string;
  policyVersion: string;
  status: 'opted_in' | 'opted_out';
  version: number;
}

export interface CustomerNotificationQuietHours {
  crossesMidnight: boolean;
  enabled: boolean;
  endMinute: number;
  startMinute: number;
  timezoneName: string;
  version: number;
}

export interface CustomerNotificationPreferences {
  customerNo: string;
  marketingConsents: CustomerNotificationMarketingConsent[];
  preferences: CustomerNotificationContactPreference[];
  preferredLocale: 'en-US' | 'zh-CN';
  quietHours?: CustomerNotificationQuietHours;
  version: number;
}

export interface QuietHoursInput {
  enabled: boolean;
  endMinute: number;
  startMinute: number;
  timezoneName: string;
}

export interface UpdateCustomerNotificationPreferenceInput {
  channelCode?: NotificationChannel;
  contactNo?: string;
  enabled?: boolean;
  expectedVersion: number;
  idempotencyKey: string;
  locale?: 'en-US' | 'zh-CN';
  marketingConsentAction?: 'granted' | 'withdrawn';
  policyReference?: string;
  policyVersion?: string;
  quietHours?: QuietHoursInput;
  topic?: NotificationTopic;
}

export interface UpdateCustomerNotificationPreferenceResult {
  appliedAt: number;
  evidenceDigest: string;
  reused: boolean;
  version: number;
}

const base = pluginApiPath('idc-notify', 'account/notifications');

export function listCustomerNotificationDeliveries(
  params: { cursor?: string; limit?: number } = {},
) {
  return requestClient.get<CustomerNotificationDeliveryPage>(
    `${base}/deliveries`,
    { params },
  );
}

export function getCustomerNotificationPreferences() {
  return requestClient.get<CustomerNotificationPreferences>(
    `${base}/preferences`,
  );
}

export function updateCustomerNotificationPreference(
  input: UpdateCustomerNotificationPreferenceInput,
) {
  return requestClient.put<UpdateCustomerNotificationPreferenceResult>(
    `${base}/preferences`,
    input,
  );
}
