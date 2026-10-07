import type { Page, Route } from "@host-tests/support/playwright";

import * as XLSX from "xlsx";

function apiEnvelope(data: unknown) {
  // These arrays are complete test fixture collections, not production totals.
  if (data && typeof data === "object") {
    const value = data as Record<string, unknown>;
    const rows = [
      "segments",
      "campaigns",
      "assets",
      "accounts",
      "grants",
      "adjustments",
      "rows",
      "cases",
      "items",
    ]
      .map((key) => value[key])
      .find(Array.isArray);
    if (rows && value.total === undefined) value.total = rows.length;
  }
  return { code: 0, data, message: "success" };
}

export interface MarketingMockState {
  actionRequests: Array<{
    body: Record<string, unknown>;
    method: string;
    url: URL;
  }>;
  couponBatchRequests: Array<Record<string, unknown>>;
  directoryRequests: URL[];
  overviewRequests: URL[];
  requests: URL[];
  insightMutations: Array<{
    body: Record<string, unknown>;
    url: URL;
  }>;
}

export async function installMarketingOperationsMock(
  page: Page,
  options: {
    enableCouponBatch?: boolean;
    enableFinancialActions?: boolean;
    enableInsights?: boolean;
    directoryRows?: number;
  } = {},
): Promise<MarketingMockState> {
  const state: MarketingMockState = {
    actionRequests: [],
    couponBatchRequests: [],
    directoryRequests: [],
    overviewRequests: [],
    requests: [],
    insightMutations: [],
  };
  function directoryEnvelope(data: Record<string, unknown>, route: Route) {
    if (options.directoryRows === undefined) return apiEnvelope(data);
    const keys: Record<string, string[]> = {
      segments: ["segmentNo", "segmentCode"],
      campaigns: ["campaignNo", "campaignCode"],
      assets: ["assetNo"],
      accounts: ["customerNo"],
      grants: ["grantNo"],
      adjustments: ["adjustmentNo"],
      rows: ["campaignNo"],
      cases: ["caseNo"],
      items: ["orderItemNo"],
    };
    const key = Object.keys(keys).find((key) => Array.isArray(data[key]));
    if (!key) throw new Error("Missing directory fixture rows");
    const examples = data[key] as Record<string, unknown>[];
    const collection = Array.from(
      { length: options.directoryRows },
      (_, index) => {
        const item = { ...examples[index % examples.length] };
        if (index >= examples.length)
          for (const field of keys[key]!)
            item[field] = `${item[field]}-page-${index}`;
        return item;
      },
    );
    const url = new URL(route.request().url());
    const size = Number(url.searchParams.get("limit") || 10);
    const pageNumber = Number(url.searchParams.get("page") || 1);
    return apiEnvelope({
      ...data,
      [key]: collection.slice((pageNumber - 1) * size, pageNumber * size),
      total: collection.length,
      nextCursor: "",
    });
  }
  const financialActionPermissions = [
    "idc:admin:marketing:grant:post",
    "idc:admin:marketing:grant:reverse",
    "idc:admin:marketing:grant:review",
  ];
  const extraPermissions = [
    ...(options.enableFinancialActions ? financialActionPermissions : []),
    ...(options.enableCouponBatch
      ? ["idc:admin:marketing:coupon-code:generate"]
      : []),
    ...(options.enableInsights
      ? [
          "idc:admin:marketing:performance:list",
          "idc:admin:marketing:abuse:list",
          "idc:admin:marketing:abuse:review",
          "idc:admin:marketing:cost:missing:list",
          "idc:admin:marketing:cost:correct",
        ]
      : []),
  ];
  if (extraPermissions.length > 0) {
    await page.route("**/api/v1/user/info", async (route: Route) => {
      const response = await route.fetch();
      const body = (await response.json()) as {
        data?: { permissions?: string[] };
        permissions?: string[];
      };
      const userInfo = body.data ?? body;
      userInfo.permissions = [
        ...new Set([...(userInfo.permissions ?? []), ...extraPermissions]),
      ];
      await route.fulfill({ json: body, response });
    });
    await page.evaluate((permissions) => {
      const accessStorageKey = "lina-web-antd-5.6.0-dev-core-access";
      const rawAccessState = localStorage.getItem(accessStorageKey);
      const accessState = rawAccessState ? JSON.parse(rawAccessState) : {};
      localStorage.setItem(
        accessStorageKey,
        JSON.stringify({
          ...accessState,
          accessCodes: [
            ...new Set([...(accessState.accessCodes ?? []), ...permissions]),
          ],
        }),
      );
    }, extraPermissions);
  }
  if (options.enableCouponBatch) {
    await page.route(
      "**/x/idc-core/api/v1/admin/marketing/coupon-codes/batches",
      async (route: Route) => {
        state.couponBatchRequests.push(
          route.request().postDataJSON() as Record<string, unknown>,
        );
        const workbook = XLSX.utils.book_new();
        XLSX.utils.book_append_sheet(
          workbook,
          XLSX.utils.aoa_to_sheet([
            ["Batch number", "Coupon code"],
            ["COUPON_BATCH_0001", "SUMMER2026CODE01"],
            ["COUPON_BATCH_0001", "SUMMER2026CODE02"],
          ]),
          "Coupon codes",
        );
        await route.fulfill({
          body: Buffer.from(
            XLSX.write(workbook, { bookType: "xlsx", type: "buffer" }),
          ),
          contentType:
            "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
          headers: {
            "Content-Disposition":
              'attachment; filename="coupon-codes-COUPON_BATCH_0001.xlsx"',
          },
          status: 200,
        });
      },
    );
  }
  if (options.enableInsights) {
    await page.route(
      "**/x/idc-core/api/v1/admin/marketing/performance?**",
      async (route: Route) => {
        await route.fulfill({
          json: directoryEnvelope(
            {
              endedAt: 1_789_017_600_000,
              rows: [
                {
                  campaignNo: "campaign-summer-2026",
                  claim: 42,
                  costComplete: true,
                  costMinor: "7000",
                  currencyCode: "CNY",
                  exposure: 160,
                  nameEnUs: "Summer campaign",
                  nameZhCn: "夏季活动",
                  netMinor: "81000",
                  order: 12,
                  receiptMinor: "100000",
                  refundMinor: "12000",
                  use: 15,
                },
                {
                  campaignNo: "campaign-points-2026",
                  claim: 9,
                  costComplete: false,
                  costMinor: "0",
                  currencyCode: "",
                  exposure: 40,
                  nameEnUs: "Points campaign",
                  nameZhCn: "积分活动",
                  netMinor: "0",
                  order: 4,
                  receiptMinor: "0",
                  refundMinor: "0",
                  use: 7,
                },
              ],
              startedAt: 1_786_339_200_000,
            },
            route,
          ),
        });
      },
    );
    await page.route(
      "**/x/idc-core/api/v1/admin/marketing/abuse-cases?**",
      async (route: Route) => {
        await route.fulfill({
          json: directoryEnvelope(
            {
              cases: [
                {
                  blockedUntil: null,
                  campaignNo: "campaign-summer-2026",
                  caseNo: "MKT_CASE_0001",
                  createdAt: 1_786_339_200_000,
                  customerNo: "CUS_masked_0001",
                  evidenceSummaryCode: "refund_ratio_threshold",
                  observedCount: 4,
                  relatedAccountCount: 2,
                  reviewReason: "",
                  reviewedAt: null,
                  reviewedBy: 0,
                  signalCode: "refund_arbitrage",
                  statusCode: "pending_review",
                  updatedAt: 1_786_339_200_000,
                  version: 1,
                  windowEndedAt: 1_789_017_600_000,
                  windowStartedAt: 1_786_339_200_000,
                },
              ],
            },
            route,
          ),
        });
      },
    );
    await page.route(
      "**/x/idc-core/api/v1/admin/marketing/abuse-cases/*/reviews",
      async (route: Route) => {
        state.insightMutations.push({
          body: route.request().postDataJSON() as Record<string, unknown>,
          url: new URL(route.request().url()),
        });
        await route.fulfill({
          json: apiEnvelope({ case: { caseNo: "MKT_CASE_0001" } }),
        });
      },
    );
    await page.route(
      "**/x/idc-core/api/v1/admin/marketing/missing-costs?**",
      async (route: Route) => {
        await route.fulfill({
          json: directoryEnvelope(
            {
              items: [
                {
                  bindingNo: "POOL_BINDING_0001",
                  correctionReason: "",
                  createdAt: 1_786_339_200_000,
                  currencyCode: "CNY",
                  orderItemNo: "ORDER_ITEM_0001",
                  orderNo: "ORDER_0001",
                  periodCount: 1,
                  periodUnitCode: "month",
                  productNameEnUs: "Cloud server",
                  productNameZhCn: "云服务器",
                  purchaseCycles: 1,
                  serverQuantity: 2,
                  statusCode: "missing",
                  totalCostMinor: null,
                  unitCostMinor: null,
                  updatedAt: 1_786_339_200_000,
                  version: 1,
                },
              ],
            },
            route,
          ),
        });
      },
    );
    await page.route(
      "**/x/idc-core/api/v1/admin/marketing/missing-costs/*/corrections",
      async (route: Route) => {
        state.insightMutations.push({
          body: route.request().postDataJSON() as Record<string, unknown>,
          url: new URL(route.request().url()),
        });
        await route.fulfill({
          json: apiEnvelope({ item: { orderItemNo: "ORDER_ITEM_0001" } }),
        });
      },
    );
  }
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/overview",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.overviewRequests.push(url);
      await route.fulfill({
        json: apiEnvelope({
          currencies: [
            {
              availableMinor: 8_800,
              consumedMinor: 1_200,
              currencyCode: "CNY",
              expiredMinor: 0,
              frozenMinor: 500,
              grantCount: 3,
              pendingMinor: 2_000,
              reversedMinor: 300,
            },
            {
              availableMinor: 3_400,
              consumedMinor: 900,
              currencyCode: "USD",
              expiredMinor: 100,
              frozenMinor: 0,
              grantCount: 2,
              pendingMinor: 0,
              reversedMinor: 600,
            },
          ],
          generatedAt: 1_786_339_200_000 + state.requests.length * 1_000,
          recentCampaigns: [
            {
              campaignNo: "campaign-first-recharge",
              endsAt: 1_789_017_600_000,
              globalConsumedCount: 187,
              globalQuotaLimit: 1_000,
              globalReservedCount: 12,
              nameEnUs: "First recharge bonus",
              nameZhCn: "首充赠送",
              priority: 20,
              startsAt: 1_786_339_200_000,
              status: "active",
              targetSegmentNo: "segment-new-customer",
              targetSnapshotNo: "snapshot-new-customer-v3",
              type: "recharge_bonus",
              updatedAt: 1_786_339_260_000,
            },
          ],
          recentGrants: [
            {
              availableMinor: 8_800,
              campaignNo: "campaign-first-recharge",
              consumedMinor: 1_200,
              createdAt: 1_786_339_200_000,
              currencyCode: "CNY",
              customerNo: "CUS_marketing_customer_0001",
              expiredMinor: 0,
              expiresAt: 1_789_017_600_000,
              frozenMinor: 0,
              grantNo: "grant-first-recharge-0001",
              grantType: "recharge_bonus",
              originalMinor: 10_000,
              pendingMinor: 0,
              reversedMinor: 0,
              status: "available",
              updatedAt: 1_786_339_260_000,
              withdrawable: false,
            },
          ],
          summary: {
            activeSegmentMembers: 12_480,
            assetActive: 328,
            assetConsumedUses: 611,
            assetIssued: 84,
            assetReservedUses: 19,
            assetTotal: 1_023,
            campaignActive: 4,
            campaignConsumedUnits: 972,
            campaignPaused: 1,
            campaignReservedUnits: 31,
            campaignScheduled: 2,
            campaignTotal: 17,
            consentOptedIn: 8_204,
            consentOptedOut: 719,
            pointsAccountCount: 3_741,
            pointsAvailable: 928_400,
            pointsDebt: 120,
            pointsFrozen: 16_800,
            segmentActive: 6,
            segmentDynamic: 4,
            segmentTotal: 9,
          },
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/segments?**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.directoryRequests.push(url);
      const isNext =
        options.directoryRows === undefined &&
        Number(url.searchParams.get("page") || 1) > 1;
      await route.fulfill({
        json: directoryEnvelope(
          {
            nextCursor: isNext ? "" : "opaque-segment-next",
            total: 11,
            segments: [
              {
                activeMemberCount: isNext ? 42 : 12_480,
                conditionCount: isNext ? 1 : 3,
                createdAt: 1_786_300_000_000,
                latestSnapshotAt: 1_786_339_100_000,
                latestSnapshotMembers: isNext ? 40 : 12_451,
                latestSnapshotNo: isNext
                  ? "snapshot-dormant-v1"
                  : "snapshot-loyal-v3",
                latestSnapshotStatus: "complete",
                mode: isNext ? "static" : "dynamic",
                nameEnUs: isNext ? "Dormant customers" : "Loyal customers",
                nameZhCn: isNext ? "沉睡客户" : "忠诚客户",
                refreshIntervalMinutes: isNext ? null : 60,
                segmentCode: isNext ? "dormant_customers" : "loyal_customers",
                segmentNo: isNext ? "segment-dormant" : "segment-loyal",
                status: "active",
                updatedAt: 1_786_339_200_000,
                version: isNext ? 1 : 3,
              },
            ].flatMap((row) =>
              isNext
                ? [row]
                : Array.from({ length: 10 }, (_, index) =>
                    index === 0
                      ? row
                      : {
                          ...row,
                          segmentNo: `segment-page-${index}`,
                          segmentCode: `page_segment_${index}`,
                          nameZhCn: `分页分群 ${index}`,
                        },
                  ),
            ),
          },
          route,
        ),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/segments/*",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.directoryRequests.push(url);
      await route.fulfill({
        json: apiEnvelope({
          conditions: [
            {
              conditionNo: "condition-loyal-age",
              currencyCode: "",
              fact: "registered_age_days",
              operator: "gte",
              orGroupNo: 1,
              sortOrder: 10,
              valueBoolean: null,
              valueIntegerLower: 365,
              valueIntegerUpper: null,
              valueKind: "integer",
              valueText: "",
              values: [],
            },
          ],
          description: {
            enUs: "Customers registered for at least one year.",
            zhCn: "注册满一年的长期客户。",
          },
          segment: {
            activeMemberCount: 12_480,
            conditionCount: 1,
            createdAt: 1_786_300_000_000,
            latestSnapshotAt: 1_786_339_100_000,
            latestSnapshotMembers: 12_451,
            latestSnapshotNo: "snapshot-loyal-v3",
            latestSnapshotStatus: "complete",
            mode: "dynamic",
            nameEnUs: "Loyal customers",
            nameZhCn: "忠诚客户",
            refreshIntervalMinutes: 60,
            segmentCode: "loyal_customers",
            segmentNo: "segment-loyal",
            status: "active",
            updatedAt: 1_786_339_200_000,
            version: 3,
          },
          snapshots: [
            {
              completedAt: 1_786_339_160_000,
              memberCount: 12_451,
              segmentVersion: 3,
              snapshotNo: "snapshot-loyal-v3",
              startedAt: 1_786_339_100_000,
              status: "complete",
            },
          ],
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/campaigns?**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.directoryRequests.push(url);
      await route.fulfill({
        json: directoryEnvelope(
          {
            campaigns: [
              {
                benefitCount: 1,
                campaignCode: "summer_cashback",
                campaignNo: "campaign-summer-cashback",
                createdAt: 1_786_300_000_000,
                endsAt: 1_789_017_600_000,
                globalConsumedCount: 145,
                globalQuotaLimit: 1_000,
                globalReservedCount: 27,
                nameEnUs: "Summer cashback",
                nameZhCn: "暑期返现",
                perCustomerQuotaLimit: 1,
                priority: 100,
                reservationCount: 172,
                scopeCount: 1,
                stackPolicy: "same_group_exclusive",
                startsAt: 1_786_339_200_000,
                status: "active",
                targetSegmentNo: "segment-loyal",
                targetSnapshotNo: "snapshot-loyal-v3",
                type: "cashback",
                updatedAt: 1_786_339_260_000,
              },
            ],
            nextCursor: "",
          },
          route,
        ),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/campaigns/*",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.directoryRequests.push(url);
      await route.fulfill({
        json: apiEnvelope({
          benefits: [
            {
              amountMinor: 1_000,
              benefitNo: "benefit-summer-cashback",
              currencyCode: "CNY",
              cycleCount: null,
              cycleScope: "first",
              maximumDiscountMinor: null,
              minimumSpendMinor: 10_000,
              pointsQuantity: null,
              purchasePhase: "both",
              rateBasisPoints: null,
              status: "active",
              type: "cashback",
              version: 1,
            },
          ],
          campaign: {
            benefitCount: 1,
            campaignCode: "summer_cashback",
            campaignNo: "campaign-summer-cashback",
            createdAt: 1_786_300_000_000,
            endsAt: 1_789_017_600_000,
            globalConsumedCount: 145,
            globalQuotaLimit: 1_000,
            globalReservedCount: 27,
            nameEnUs: "Summer cashback",
            nameZhCn: "暑期返现",
            perCustomerQuotaLimit: 1,
            priority: 100,
            reservationCount: 172,
            scopeCount: 1,
            stackPolicy: "same_group_exclusive",
            startsAt: 1_786_339_200_000,
            status: "active",
            targetSegmentNo: "segment-loyal",
            targetSnapshotNo: "snapshot-loyal-v3",
            type: "cashback",
            updatedAt: 1_786_339_260_000,
          },
          description: {
            enUs: "Cashback for loyal server customers.",
            zhCn: "面向忠诚服务器客户的返现活动。",
          },
          scopes: [
            {
              kind: "product_type",
              matchMode: "include",
              referenceCode: "cloud_server",
            },
          ],
          stackGroup: "summer",
          version: 2,
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/assets?**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.directoryRequests.push(url);
      await route.fulfill({
        json: directoryEnvelope(
          {
            assets: [
              {
                assetNo: "asset-coupon-001",
                benefitNo: "benefit-coupon-001",
                benefitType: "fixed_discount",
                campaignNo: "campaign-coupon-2026",
                claimedAt: 1_786_339_210_000,
                consumedUses: 0,
                createdAt: 1_786_300_000_000,
                currencyCode: "CNY",
                customerNo: "CUS_marketing_customer_0001",
                faceAmountMinor: 500,
                rateBasisPoints: null,
                reservedUses: 1,
                status: "active",
                type: "coupon",
                updatedAt: 1_786_339_260_000,
                usageLimit: 1,
                validFrom: 1_786_300_000_000,
                validUntil: 1_789_017_600_000,
              },
              {
                assetNo: "asset-revocable-002",
                benefitNo: "benefit-coupon-001",
                benefitType: "fixed_discount",
                campaignNo: "campaign-coupon-2026",
                claimedAt: 1_786_339_210_000,
                consumedUses: 0,
                createdAt: 1_786_300_000_000,
                currencyCode: "CNY",
                customerNo: "CUS_marketing_customer_0002",
                faceAmountMinor: 300,
                rateBasisPoints: null,
                reservedUses: 0,
                status: "active",
                type: "coupon",
                updatedAt: 1_786_339_261_000,
                usageLimit: 1,
                validFrom: 1_786_300_000_000,
                validUntil: 1_789_017_600_000,
              },
            ],
            nextCursor: "",
          },
          route,
        ),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/assets/*",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.directoryRequests.push(url);
      await route.fulfill({
        json: apiEnvelope({
          asset: {
            assetNo: "asset-coupon-001",
            benefitNo: "benefit-coupon-001",
            benefitType: "fixed_discount",
            campaignNo: "campaign-coupon-2026",
            claimedAt: 1_786_339_210_000,
            consumedUses: 0,
            createdAt: 1_786_300_000_000,
            currencyCode: "CNY",
            customerNo: "CUS_marketing_customer_0001",
            faceAmountMinor: 500,
            rateBasisPoints: null,
            reservedUses: 1,
            status: "active",
            type: "coupon",
            updatedAt: 1_786_339_260_000,
            usageLimit: 1,
            validFrom: 1_786_300_000_000,
            validUntil: 1_789_017_600_000,
          },
          reservations: [
            {
              createdAt: 1_786_339_220_000,
              discountAmountMinor: 500,
              expiresAt: 1_786_339_520_000,
              reservationNo: "asset-reservation-001",
              reservedUses: 1,
              status: "active",
            },
          ],
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/points/accounts?**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.directoryRequests.push(url);
      await route.fulfill({
        json: directoryEnvelope(
          {
            accounts: [
              {
                availablePoints: 1_200,
                createdAt: 1_786_300_000_000,
                customerNo: "CUS_marketing_customer_0001",
                debtPoints: 20,
                entryCount: 4,
                frozenPoints: 100,
                lotCount: 2,
                updatedAt: 1_786_339_260_000,
                version: 7,
              },
            ],
            nextCursor: "",
          },
          route,
        ),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/points/accounts/*",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.directoryRequests.push(url);
      await route.fulfill({
        json: apiEnvelope({
          account: {
            availablePoints: 1_200,
            createdAt: 1_786_300_000_000,
            customerNo: "CUS_marketing_customer_0001",
            debtPoints: 20,
            entryCount: 4,
            frozenPoints: 100,
            lotCount: 2,
            updatedAt: 1_786_339_260_000,
            version: 7,
          },
          entries: [
            {
              availableDelta: 1_500,
              createdAt: 1_786_300_000_000,
              debtDelta: 0,
              entryNo: "points-entry-001",
              frozenDelta: 0,
              lotNo: "points-lot-001",
              reversalOfEntryNo: "",
              sourceReference: "order-001",
              sourceType: "order",
              type: "earn",
            },
          ],
          lots: [
            {
              availablePoints: 1_200,
              campaignNo: "campaign-points-2026",
              consumedPoints: 200,
              createdAt: 1_786_300_000_000,
              expiredPoints: 0,
              expiresAt: 1_789_017_600_000,
              frozenPoints: 100,
              lotNo: "points-lot-001",
              originalPoints: 1_500,
              reversalShortfallPoints: 0,
              reversedPoints: 0,
              sourceReference: "order-001",
              sourceType: "order",
              status: "available",
            },
          ],
        }),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/grants?**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.directoryRequests.push(url);
      await route.fulfill({
        json: directoryEnvelope(
          {
            grants: [
              {
                availableAmountMinor: 8_800,
                benefitNo: "benefit-recharge-001",
                campaignNo: "campaign-first-recharge",
                consumedAmountMinor: 1_200,
                createdAt: 1_786_300_000_000,
                currencyCode: "CNY",
                customerNo: "CUS_marketing_customer_0001",
                customerQuotaReserved: false,
                expiredAmountMinor: 0,
                expiresAt: 1_789_017_600_000,
                financialPostingReference: "ledger-entry-001",
                frozenAmountMinor: 0,
                globalQuotaReserved: false,
                grantNo: "grant-ledger-001",
                originalAmountMinor: 10_000,
                pendingAmountMinor: 0,
                reversedAmountMinor: 0,
                sourceReference: "payment-001",
                status: "available",
                type: "recharge_bonus",
                updatedAt: 1_786_339_260_000,
                withdrawable: false,
              },
              {
                availableAmountMinor: 0,
                benefitNo: "benefit-cashback-002",
                campaignNo: "campaign-summer-cashback",
                consumedAmountMinor: 0,
                createdAt: 1_786_339_280_000,
                currencyCode: "CNY",
                customerNo: "CUS_marketing_customer_0002",
                customerQuotaReserved: true,
                expiredAmountMinor: 0,
                expiresAt: 1_789_017_600_000,
                financialPostingReference: "",
                frozenAmountMinor: 0,
                globalQuotaReserved: true,
                grantNo: "grant-pending-002",
                originalAmountMinor: 2_000,
                pendingAmountMinor: 2_000,
                reversedAmountMinor: 0,
                sourceReference: "order-pending-002",
                status: "pending",
                type: "cashback",
                updatedAt: 1_786_339_280_000,
                withdrawable: false,
              },
            ],
            nextCursor: "",
          },
          route,
        ),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/grant-adjustments?**",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.directoryRequests.push(url);
      await route.fulfill({
        json: directoryEnvelope(
          {
            adjustments: [
              {
                adjustmentNo: "bar-review-001",
                adjustmentType: "marketing_grant_post",
                amountMinor: "2000",
                createdAt: 1_786_339_290_000,
                currencyCode: "CNY",
                customerNo: "CUS_marketing_customer_0002",
                grantNo: "grant-pending-002",
                ledgerTransactionNo: "",
                requestedBy: 17,
                reused: false,
                status: "pending_review",
              },
            ],
            nextCursor: "",
          },
          route,
        ),
      });
    },
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/grants/*",
    async (route: Route) => {
      const url = new URL(route.request().url());
      state.requests.push(url);
      state.directoryRequests.push(url);
      await route.fulfill({
        json: apiEnvelope({
          entries: [
            {
              availableDeltaMinor: -1_200,
              consumedDeltaMinor: 1_200,
              createdAt: 1_786_339_240_000,
              entryNo: "value-entry-001",
              expiredDeltaMinor: 0,
              frozenDeltaMinor: 0,
              pendingDeltaMinor: 0,
              reversalOfEntryNo: "",
              reversedDeltaMinor: 0,
              sourceReference: "order-001",
              sourceType: "order",
              type: "consume",
            },
          ],
          grant: {
            availableAmountMinor: 8_800,
            benefitNo: "benefit-recharge-001",
            campaignNo: "campaign-first-recharge",
            consumedAmountMinor: 1_200,
            createdAt: 1_786_300_000_000,
            currencyCode: "CNY",
            customerNo: "CUS_marketing_customer_0001",
            customerQuotaReserved: false,
            expiredAmountMinor: 0,
            expiresAt: 1_789_017_600_000,
            financialPostingReference: "ledger-entry-001",
            frozenAmountMinor: 0,
            globalQuotaReserved: false,
            grantNo: "grant-ledger-001",
            originalAmountMinor: 10_000,
            pendingAmountMinor: 0,
            reversedAmountMinor: 0,
            sourceReference: "payment-001",
            status: "available",
            type: "recharge_bonus",
            updatedAt: 1_786_339_260_000,
            withdrawable: false,
          },
        }),
      });
    },
  );
  const fulfillAction = async (
    route: Route,
    mutation: Record<string, unknown>,
  ) => {
    const url = new URL(route.request().url());
    const body = (route.request().postDataJSON() ?? {}) as Record<
      string,
      unknown
    >;
    state.requests.push(url);
    state.actionRequests.push({ body, method: route.request().method(), url });
    await route.fulfill({ json: apiEnvelope({ mutation }) });
  };
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/assets/issue",
    (route: Route) =>
      fulfillAction(route, {
        assetNo: "mast-issued-001",
        customerNo: "CUS_marketing_customer_0003",
        reused: false,
        status: "active",
        updatedAt: 1_786_339_300_000,
      }),
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/assets/*/revoke",
    (route: Route) =>
      fulfillAction(route, {
        assetNo: "asset-revocable-002",
        customerNo: "CUS_marketing_customer_0002",
        reused: false,
        status: "revoked",
        updatedAt: 1_786_339_301_000,
      }),
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/points/accounts/*/credits",
    (route: Route) =>
      fulfillAction(route, {
        availablePoints: 1_300,
        customerNo: "CUS_marketing_customer_0001",
        debtPoints: 20,
        frozenPoints: 100,
        lotNo: "points-lot-manual-002",
        reused: false,
      }),
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/points/accounts/*/reversals",
    (route: Route) =>
      fulfillAction(route, {
        availablePoints: 700,
        customerNo: "CUS_marketing_customer_0001",
        debtPoints: 20,
        frozenPoints: 100,
        lotNo: "points-lot-001",
        reused: false,
      }),
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/grants/*/posting-requests",
    (route: Route) =>
      fulfillAction(route, {
        adjustmentNo: "bar-post-002",
        adjustmentType: "marketing_grant_post",
        amountMinor: "2000",
        currencyCode: "CNY",
        customerNo: "CUS_marketing_customer_0002",
        grantNo: "grant-pending-002",
        ledgerTransactionNo: "",
        reused: false,
        status: "pending_review",
      }),
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/grants/*/reversal-requests",
    (route: Route) =>
      fulfillAction(route, {
        adjustmentNo: "bar-reverse-001",
        adjustmentType: "marketing_grant_reverse",
        amountMinor: "250",
        currencyCode: "CNY",
        customerNo: "CUS_marketing_customer_0001",
        grantNo: "grant-ledger-001",
        ledgerTransactionNo: "",
        reused: false,
        status: "pending_review",
      }),
  );
  await page.route(
    "**/x/idc-core/api/v1/admin/marketing/grant-adjustments/*/reviews",
    (route: Route) =>
      fulfillAction(route, {
        adjustmentNo: "bar-review-001",
        adjustmentType: "marketing_grant_post",
        amountMinor: "2000",
        currencyCode: "CNY",
        customerNo: "CUS_marketing_customer_0002",
        grantNo: "grant-pending-002",
        ledgerTransactionNo: "btx-post-002",
        reused: false,
        status: "posted",
      }),
  );
  return state;
}
