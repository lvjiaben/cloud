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
    requiresContract: false,
    riskDecisionCode: "manual_review",
    statusCode: "manual_review",
    totalAmountMinor: "900719925474099301",
  };
}

export interface CommerceOrderMockState {
  contractCreateRequests: Array<Record<string, unknown>>;
  contractSignerCandidateRequests: URL[];
  contractSubmissionRequests: Array<Record<string, unknown>>;
  contractTemplateCandidateRequests: URL[];
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
    listSize?: number;
    submissionStatus?: "result_unknown" | "signing";
  } = {},
): Promise<CommerceOrderMockState> {
  const state: CommerceOrderMockState = {
    contractCreateRequests: [],
    contractSignerCandidateRequests: [],
    contractSubmissionRequests: [],
    contractTemplateCandidateRequests: [],
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
  await page.route("**/api/v1/user/info", async (route: Route) => {
    const response = await route.fetch();
    const payload = (await response.json()) as {
      data?: { permissions?: string[] };
    };
    if (!payload.data) throw new Error("user-info payload is unavailable");
    payload.data.permissions = Array.from(
      new Set([
        ...(payload.data.permissions ?? []),
        "idc:admin:compliance:contract:create",
      ]),
    );
    await route.fulfill({ json: payload, response });
  });
  await page.route(
    "**/x/idc-compliance/api/v1/admin/compliance/**",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      if (url.pathname.endsWith("/electronic-contracts/template-candidates")) {
        state.contractTemplateCandidateRequests.push(url);
        await route.fulfill({
          json: apiEnvelope({
            items: [
              {
                languageCode: "zh-CN",
                scopeCode: "enterprise",
                signingOrderCode: "customer_first",
                templateName: "企业服务器服务合同",
                templateNo: "ctpl_admin_enterprise_0001",
                templateVersion: 5,
              },
            ],
          }),
        });
        return;
      }
      if (url.pathname.endsWith("/contract-signer-candidates")) {
        state.contractSignerCandidateRequests.push(url);
        await route.fulfill({
          json: apiEnvelope({
            items: [
              {
                authorizationVersion: 1,
                identityVerificationVersion: 2,
                identityVerifiedAt: 1_786_665_660_000,
                maskedName: "张*",
                maskedPhone: "138****8000",
                phoneVerificationVersion: 3,
                phoneVerifiedAt: 1_786_665_720_000,
                signerNo: "csgn_admin_enterprise_0001",
                version: 4,
              },
            ],
          }),
        });
        return;
      }
      if (url.pathname.endsWith("/signing-submissions")) {
        state.contractSubmissionRequests.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        await route.fulfill({
          json: apiEnvelope({
            item: {
              contractNo: "CTR_ADMIN_0001",
              flowNo: "CFW_ADMIN_LOCAL_0001",
              reused: false,
              statusCode: options.submissionStatus ?? "signing",
              version: 2,
            },
          }),
        });
        return;
      }
      if (
        url.pathname.endsWith("/electronic-contracts") &&
        request.method() === "POST"
      ) {
        state.contractCreateRequests.push(
          request.postDataJSON() as Record<string, unknown>,
        );
        await route.fulfill({
          json: apiEnvelope({
            item: {
              contractNo: "CTR_ADMIN_0001",
              createdAt: 1_786_665_600_000,
              currencyCode: "CNY",
              customerName: "示例企业客户",
              customerType: "enterprise",
              languageCode: "zh-CN",
              lines: [
                {
                  amountMinor: "900719925474099301",
                  orderItemNo: "OI_ADMIN_0001",
                  periodCount: 1,
                  periodUnitCode: "month",
                  purchaseCycles: 12,
                  quantity: 2,
                  serviceName: "商务手机",
                },
              ],
              merchantName: "示例 IDC 商家",
              orderNo,
              requiredForFulfillment: false,
              reused: false,
              signerName: "张三",
              signerNo: "csgn_admin_enterprise_0001",
              signerPhoneMasked: "138****8000",
              signingDate: "",
              signingDateNoticeCode: "filled_after_authoritative_signature",
              signingExpiresAt: 1_787_270_400_000,
              signingOrderCode: "customer_first",
              statusCode: "pending_launch",
              templateName: "企业服务器服务合同",
              templateNo: "ctpl_admin_enterprise_0001",
              templateVersion: 5,
              totalAmountMinor: "900719925474099301",
              version: 1,
            },
          }),
        });
        return;
      }
      await route.fallback();
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/commerce/orders**",
    async (route: Route) => {
      const request = route.request();
      const url = new URL(request.url());
      if (url.pathname.endsWith(`/${orderNo}`)) {
        state.detailRequests.push(url);
        await route.fulfill({
          json: apiEnvelope({
            contractRule: {
              policyCode: "none",
              requiresContract: false,
              sourceLevel: "default",
              version: 1,
            },
            order: {
              canApproveRisk: true,
              canRejectRisk: true,
              deliveries: [
                {
                  ongoingService: true,
                  productNameEnUS: "Wildcard SSL certificate",
                  productNameZhCN: "通配符 SSL 证书",
                  quantity: 1,
                  statusCode: "processing",
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
              summary: summary(),
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
            orderNo: index === 0 ? orderNo : `${orderNo}_${index + 1}`,
          })).slice(offset, offset + limit),
        }),
      });
    },
  );
  return state;
}

export const commerceAdminOrderNo = orderNo;
