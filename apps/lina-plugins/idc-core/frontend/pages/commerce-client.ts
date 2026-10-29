import { pluginApiPath, requestClient } from "#/api/request";
import { requestPreservingHttpStatus } from "./commerce-request";


const pluginID = "idc-core";

export interface CommerceOrderSummary {
  assistedRequestNo?: string;
  canCancel: boolean;
  canPreparePayment: boolean;
  createdAt: number;
  currencyCode: string;
  customerNo: string;
  customerCode: string;
  customerDisplayName: string;
  customerPrimaryPhone: string;
  customerTypeCode?: "enterprise" | "individual";
  itemCount: number;
  ipAddress?: string;
  orderNo: string;
  paymentMethodCode?: string;
  paymentDueAt?: number;
  paymentTermCode: string;
  productSummary?: string;
  remark?: string;
  riskDecisionCode: string;
  statusCode: string;
  totalAmountMinor: string;
}

export interface CommerceOrderDetail {
  canApproveRisk: boolean;
  canRejectRisk: boolean;
  cancellation?: Record<string, number | string>;
  deliveries: CommerceDelivery[];
  invoice?: Record<string, number | string>;
  items: Array<Record<string, boolean | number | string>>;
  paymentPreparation?: Record<string, number | string>;
  reviews: Array<Record<string, number | string>>;
  serviceEntitlements: CommerceServiceEntitlement[];
  summary: CommerceOrderSummary;
}

export interface CommerceDelivery {
  completedAt?: number;
  ongoingService: boolean;
  productNameEnUS: string;
  productNameZhCN: string;
  quantity: number;
  startedAt?: number;
  statusCode:
    | "awaiting_payment"
    | "awaiting_review"
    | "cancelled"
    | "completed"
    | "manual_processing"
    | "not_completed"
    | "processing";
  updatedAt: number;
}

export interface CommerceServiceEntitlement {
  expiresAt?: number;
  productNameEnUS: string;
  productNameZhCN: string;
  startedAt: number;
  statusCode: "active" | "expired" | "suspended" | "terminated";
}

export interface CommerceOrderPage {
  total: number;
  nextCursor: string;
  orders: CommerceOrderSummary[];
}

export interface CommerceOrderDetailResult {
  order: CommerceOrderDetail;
}

export function commerceOrders(params: {
  page?: number;
  cursor?: string;
  customerNo?: string;
  limit?: number;
  riskDecisionCode?: string;
  statusCode?: string;
}) {
  return requestPreservingHttpStatus<CommerceOrderPage>(
    requestClient,
    pluginApiPath(pluginID, "admin/commerce/orders"),
    { method: "GET", params },
  );
}

export async function commerceOrderDetail(orderNo: string) {
  return requestClient.get<CommerceOrderDetailResult>(
    pluginApiPath(
      pluginID,
      `admin/commerce/orders/${encodeURIComponent(orderNo)}`,
    ),
  );
}
