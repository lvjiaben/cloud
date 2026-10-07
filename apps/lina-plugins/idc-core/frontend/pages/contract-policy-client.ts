import { pluginApiPath, requestClient } from "#/api/request";

const pluginID = "idc-core";

export type ContractPolicyCode = "inherit" | "none" | "required";
export type ContractPolicyLevel = "enterprise" | "order" | "product";

export interface ContractPolicyTemplate {
  languageCode: "en-US" | "zh-CN";
  name: string;
  scopeCode: "all_customers" | "enterprise" | "personal";
  signingOrderCode: "customer_first" | "merchant_first" | "parallel";
  templateNo: string;
  versionNo: number;
}

export interface ContractPolicyRule {
  policyCode: ContractPolicyCode;
  templateNo: string;
  version: number;
}

export interface ContractPolicyOrderRule extends ContractPolicyRule {
  decisionSource: "default" | "enterprise" | "order" | "product";
  requiresContract: boolean;
  template?: ContractPolicyTemplate;
}

export function contractPolicyTemplates(
  level: ContractPolicyLevel,
  targetNo = "",
  customerNo = "",
) {
  return requestClient.get<{ templates: ContractPolicyTemplate[] }>(
    pluginApiPath(pluginID, "admin/contract-policy/templates"),
    {
      params: {
        customerNo: customerNo || undefined,
        level,
        targetNo: targetNo || undefined,
      },
    },
  );
}

export function customerContractPolicyUpdate(
  customerNo: string,
  input: {
    expectedVersion: number;
    policyCode: Exclude<ContractPolicyCode, "inherit">;
    templateNo: string;
  },
) {
  return requestClient.put<{ rule: ContractPolicyRule }>(
    pluginApiPath(
      pluginID,
      `admin/customers/${encodeURIComponent(customerNo)}/contract-policy`,
    ),
    input,
  );
}

export function catalogContractPolicyUpdate(
  versionNo: string,
  input: {
    expectedVersion: number;
    policyCode: ContractPolicyCode;
    templateNo: string;
  },
) {
  return requestClient.put<{ rule: ContractPolicyRule }>(
    pluginApiPath(
      pluginID,
      `admin/catalog/product-versions/${encodeURIComponent(versionNo)}/contract-policy`,
    ),
    input,
  );
}

export function orderContractPolicyUpdate(
  orderNo: string,
  input: {
    expectedVersion: number;
    policyCode: ContractPolicyCode;
    templateNo: string;
  },
) {
  return requestClient.put<{ rule: ContractPolicyOrderRule }>(
    pluginApiPath(
      pluginID,
      `admin/commerce/orders/${encodeURIComponent(orderNo)}/contract-policy`,
    ),
    input,
  );
}
