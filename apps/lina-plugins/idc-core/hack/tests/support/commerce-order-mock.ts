import type { Page, Route } from "@host-tests/support/playwright";

const orderNo = "ORD_20260811_ADMIN_0001";

function apiEnvelope(data: unknown) {
  return { code: 0, data, message: "success" };
}

function summary() {
  return {
    canCancel: true,
    canPreparePayment: true,
    createdAt: 1_786_406_400_000,
    currencyCode: "CNY",
    customerNo: "CUS_admin_customer_0001",
    customerCode: "10008",
    customerDisplayName: "订单测试客户",
    customerPrimaryPhone: "138****8008",
    customerTypeCode: "enterprise",
    itemCount: 2,
    orderNo,
    paymentTermCode: "extended_prepay",
    paymentDueAt: 1_786_410_000_000,
    riskDecisionCode: "manual_review",
    statusCode: "manual_review",
    totalAmountMinor: "900719925474099301",
  };
}

export interface CommerceOrderMockState {
  retiredContractRequests: URL[];
  detailRequests: URL[];
  listRequests: URL[];
  customerListRequests: URL[];
  failNextList: boolean;
  denyNextList: boolean;
}

export async function installCommerceOrderMock(
  page: Page,
  options: {
    emptyItems?: boolean;
    currencyCode?: string;
    deliveryStatus?: "processing" | "manual_processing";
    listSize?: number;
  } = {},
): Promise<CommerceOrderMockState> {
  const state: CommerceOrderMockState = {
    retiredContractRequests: [],
    detailRequests: [],
    listRequests: [],
    customerListRequests: [],
    failNextList: false,
    denyNextList: false,
  };
  await page.route("**/x/idc-core/api/v1/admin/customers**", async (route: Route) => {
    const url = new URL(route.request().url());
    state.customerListRequests.push(url);
    await route.fulfill({ json: apiEnvelope({ customers: [{ customerNo: "CUS_admin_customer_0001", customerCode: "10008", displayName: "订单测试客户", primaryPhone: "138****8008" }], nextCursor: "" }) });
  });
  await page.route("**/x/idc-compliance/api/v1/admin/compliance/electronic-contracts**", async route => {
    state.retiredContractRequests.push(new URL(route.request().url()));
    await route.fulfill({ status: 404, json: { code: 404, message: "Electronic contracts have been removed" } });
  });
  await page.route(
    "**/x/idc-core/api/v1/admin/commerce/orders**",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      if (url.pathname.endsWith(`/${orderNo}`)) {
        state.detailRequests.push(url);
        await route.fulfill({
          json: apiEnvelope({
            order: {
              canApproveRisk: true,
              canRejectRisk: true,
              deliveries: [
                {
                  ongoingService: true,
                  productNameEnUS: "Wildcard SSL certificate",
                  productNameZhCN: "通配符 SSL 证书",
                  quantity: 1,
                  statusCode: options.deliveryStatus ?? "processing",
                  updatedAt: 1_786_406_400_000,
                },
                {
                  completedAt: 1_786_406_400_000,
                  ongoingService: false,
                  productNameEnUS: "Business phone",
                  productNameZhCN: "商务手机",
                  quantity: 2,
                  statusCode: "completed",
                  updatedAt: 1_786_406_400_000,
                },
              ],
              invoice: {
                invoiceNo: "INV_ADMIN_0001",
                statusCode: "open",
                totalAmountMinor: "900719925474099301",
              },
              items: options.emptyItems ? [] : [
                {
                  orderItemNo: "OI_ADMIN_0001",
                  productNameEnUS: "Wildcard SSL certificate",
                  productNameZhCN: "通配符 SSL 证书",
                  purchaseCycles: 12,
                  quantity: 1,
                  totalAmountMinor: "900719925474099301",
                },
                {
                  orderItemNo: "OI_ADMIN_0002",
                  productNameEnUS: "Business phone",
                  productNameZhCN: "商务手机",
                  purchaseCycles: 1,
                  quantity: 2,
                  totalAmountMinor: "17600",
                },
              ],
              reviews: [
                {
                  reviewNo: "REV_ADMIN_0001",
                  reviewTypeCode: "risk",
                  statusCode: "pending",
                },
                {
                  reviewNo: "REV_ADMIN_0002",
                  reviewTypeCode: "future_review_type",
                  statusCode: "future_review_status",
                },
              ],
              serviceEntitlements: [
                {
                  productNameEnUS: "Wildcard SSL certificate",
                  productNameZhCN: "通配符 SSL 证书",
                  startedAt: 1_786_406_400_000,
                  statusCode: "active",
                },
              ],
              summary: {...summary(),currencyCode:options.currencyCode ?? "CNY"},
            },
          }),
        });
        return;
      }
      state.listRequests.push(url);
      if (state.denyNextList) {
        state.denyNextList = false;
        await route.fulfill({ status: 403, json: { code: 403, message: "无权查看订单" } });
        return;
      }
      if (state.failNextList) {
        state.failNextList = false;
        await route.fulfill({ status: 503, json: { code: 503, message: "Order list temporarily unavailable" } });
        return;
      }
      const listSize = options.listSize ?? 12;
      const limit = Number(url.searchParams.get("limit") || "10");
      const offset = (Number(url.searchParams.get("page") || "1") - 1) * limit;
      await route.fulfill({
        json: apiEnvelope({
          total: listSize,
          nextCursor: offset + limit < listSize ? "sealed-next-page" : "",
          orders: Array.from({ length: listSize }, (_, index) => ({
            ...summary(),
            currencyCode: options.currencyCode ?? "CNY",
            orderNo: index === 0 ? orderNo : `${orderNo}_${index + 1}`,
          })).slice(offset, offset + limit),
        }),
      });
    },
  );
  return state;
}

export const commerceAdminOrderNo = orderNo;
