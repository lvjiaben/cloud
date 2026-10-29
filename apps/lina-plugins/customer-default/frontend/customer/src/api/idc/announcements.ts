import { pluginApiPath, requestClient } from '#/api/request';

export interface CustomerAnnouncement {
  id: number;
  title: string;
  content: string;
  createdAt: number;
  updatedAt: number;
  sortOrder: number;
  read: boolean;
}

const base = pluginApiPath('content-notice', 'notice/announcements');

export function listCustomerAnnouncements(pageNum = 1, pageSize = 10) {
  return requestClient.get<{ list: CustomerAnnouncement[]; total: number }>(
    base, { params: { pageNum, pageSize }, silentErrorMessage: true },
  );
}

export function getCustomerAnnouncement(id: number) {
  return requestClient.get<CustomerAnnouncement>(`${base}/${id}`, { silentErrorMessage: true });
}

export function readCustomerAnnouncement(id: number) {
  return requestClient.put(`${base}/${id}/read`, undefined, { silentErrorMessage: true });
}
