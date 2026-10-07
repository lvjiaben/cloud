import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "staff-access";

export interface StaffAccessGrant {
  capabilityCode: string;
  createdAt: number;
  grantNo: string;
  nickname: string;
  scopeKey: string;
  scopeType: "department" | "global";
  status: number;
  updatedAt: number;
  userId: number;
  username: string;
  validFrom?: null | number;
  validUntil?: null | number;
  version: number;
}

export interface StaffCandidate {
  nickname: string;
  userId: number;
  username: string;
}

export interface StaffAccessGrantInput {
  capabilityCode: string;
  enabled: boolean;
  expectedVersion: number;
  reason: string;
  scopeKey: string;
  scopeType: "department" | "global";
  userId: null | number;
  validFrom: null | number;
  validUntil: null | number;
}

export function staffAccessGrantList(params: {
  capabilityCode?: string;
  keyword?: string;
  page: number;
  pageSize: number;
  status?: number;
}) {
  return requestClient.get<{ items: StaffAccessGrant[]; total: number }>(
    pluginApiPath(pluginID, "admin/staff-access/grants"),
    { params },
  );
}

export function staffCandidateList(params: {
  keyword?: string;
  page?: number;
  pageSize?: number;
}) {
  return requestClient.get<{ items: StaffCandidate[]; total: number }>(
    pluginApiPath(pluginID, "admin/staff-access/user-candidates"),
    { params },
  );
}

export function saveStaffAccessGrant(input: StaffAccessGrantInput) {
  return requestClient.put<{ item: StaffAccessGrant }>(
    pluginApiPath(pluginID, "admin/staff-access/grants"),
    input,
  );
}
