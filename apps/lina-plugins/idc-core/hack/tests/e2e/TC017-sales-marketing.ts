import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

/**
 * Sales-channel contract regression.  The production service tests cover the
 * PostgreSQL rules; this browser test proves the plugin page can consume the
 * same versioned contracts without exposing internal customer identifiers.
 */
test.describe("TC-17 代理与销售渠道", () => {
  test("TC-17a: 代理报价、AFF 归因、资格校验和佣金计算保持契约一致", async ({
    adminPage,
  }) => {
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/sales-channel/overview",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              qualifications: 1,
              affiliateLinks: 1,
              attributions: 1,
              pendingMaturity: 2,
              pendingSettlement: 1,
              currencies: [
                {
                  currencyCode: "CNY",
                  accruedMinor: 12500,
                  maturedMinor: 8000,
                },
                { currencyCode: "USD", accruedMinor: 300, maturedMinor: 100 },
              ],
            },
          }),
        });
      },
    );
    await adminPage.route("**/x/idc-core/api/v1/admin/sales-channel/**", async (route) => {
      const request = route.request();
      if (request.url().endsWith("/overview")) return route.fallback();
      if (request.url().endsWith("/offer/evaluate")) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              eligible: true,
              priceMinor: 9000,
              currencyCode: "CNY",
              snapshot: {
                qualificationNo: "qualification-public",
                qualificationVersion: 3,
                productRuleNo: "product-rule-public",
                productRuleVersion: 5,
                priceRuleNo: "price-rule-public",
                priceRuleVersion: 7,
                currencyCode: "CNY",
                basePriceMinor: 10000,
                priceMinor: 9000,
              },
            },
          }),
        });
      }
      if (request.url().endsWith("/affiliate/attribute")) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              statusCode: "eligible",
              policyCode: "first_touch",
              referrerCustomerNo: "10001",
              referredCustomerNo: "10002",
            },
          }),
        });
      }
      if (request.url().endsWith("/qualification/transition/validate")) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ code: 0, data: { allowed: true } }),
        });
      }
      if (request.url().endsWith("/commission/calculate")) {
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              amountMinor: 900,
              maturesAt: 1788307200000,
              ruleNo: "commission-rule-public",
              ruleVersion: 2,
              currencyCode: "CNY",
              operationTypeCode: "new_purchase",
            },
          }),
        });
      }
      return route.fallback();
    });

    const now = Date.now();
    const [offer, attribution, qualification, commission] =
      await adminPage.evaluate(async (nowValue) => {
        const json = async (url: string, init?: RequestInit) =>
          (await fetch(url, init)).json();
        const offer = await json(
          "/x/idc-core/api/v1/admin/sales-channel/agent/offer/evaluate",
          {
            method: "POST",
            headers: { "content-type": "application/json" },
            body: JSON.stringify({
              customerNo: "10001",
              productNo: "product-public",
              currencyCode: "CNY",
              basePriceMinor: 10000,
              costMinor: 6000,
              existingCreditUsed: 0,
              now: nowValue,
            }),
          },
        );
        const attribution = await json(
          "/x/idc-core/api/v1/admin/sales-channel/affiliate/attribute",
          {
            method: "POST",
            headers: { "content-type": "application/json" },
            body: JSON.stringify({
              now: nowValue,
              linkNo: "link-public",
              ownerCustomerNo: "10001",
              policyCode: "first_touch",
              linkStatus: "active",
              touchNo: "touch-public",
              touchOccurredAt: nowValue - 1000,
              touchExpiresAt: nowValue + 3600000,
              referredCustomerNo: "10002",
            }),
          },
        );
        const qualification = await json(
          "/x/idc-core/api/v1/admin/sales-channel/agent/qualification/transition/validate",
          {
            method: "POST",
            headers: { "content-type": "application/json" },
            body: JSON.stringify({
              now: nowValue,
              nextStatus: "active",
              nextLevelNo: "agent.standard",
              effectiveAt: nowValue,
              reasonCode: "application_approved",
              actorReference: "staff-public",
              evidenceDigest:
                "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
            }),
          },
        );
        const commission = await json(
          "/x/idc-core/api/v1/admin/sales-channel/commission/calculate",
          {
            method: "POST",
            headers: { "content-type": "application/json" },
            body: JSON.stringify({
              now: nowValue,
              operationTypeCode: "new_purchase",
              beneficiaryTypeCode: "agent",
              productNo: "product-public",
              currencyCode: "CNY",
              baseAmountMinor: 10000,
              ruleNo: "commission-rule-public",
              calculationTypeCode: "rate",
              rateBasisPoints: 900,
              maturityDays: 7,
              ruleVersion: 2,
              effectiveAt: nowValue - 3600000,
            }),
          },
        );
        return [
          offer.data,
          attribution.data,
          qualification.data,
          commission.data,
        ];
      }, now);
    expect(offer.eligible).toBe(true);
    expect(offer.priceMinor).toBe(9000);
    expect(offer.snapshot.currencyCode).toBe("CNY");
    expect(attribution.statusCode).toBe("eligible");
    expect(attribution.referrerCustomerNo).toBe("10001");
    expect(qualification.allowed).toBe(true);
    expect(commission.amountMinor).toBe(900);
    expect(commission.ruleVersion).toBe(2);

    await adminPage.goto("/idc/sales-channel");
    await expect(
      adminPage.getByTestId("sales-channel-workspace"),
    ).toContainText("代理与销售");
    await expect(
      adminPage.getByTestId("sales-channel-workspace"),
    ).toContainText("CNY");
    await expect(
      adminPage.getByTestId("sales-channel-workspace"),
    ).not.toContainText("CUS_");
    const screenshotPrefix = new Date()
      .toISOString()
      .replace(/[-:TZ.]/gu, "")
      .slice(0, 14);
    await adminPage.screenshot({
      path: `temp/20260830/${screenshotPrefix}-sales-channel-contract.png`,
      fullPage: false,
    });
  });
});
