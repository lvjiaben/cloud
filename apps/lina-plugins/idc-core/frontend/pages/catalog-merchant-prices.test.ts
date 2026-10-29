import { describe, expect, it } from "vitest";
import type { CatalogBillingRule, CatalogPrice } from "./catalog-client";
import {
  initialMerchantPriceDraft,
  merchantPriceDraftInput,
  merchantPriceDraftKey,
  type MerchantOptionPricingContext,
  type MerchantPriceDraft,
} from "./catalog-merchant-prices";

const rule = {
  billingRuleNo: "monthly",
  pricePhase: "both",
} as CatalogBillingRule;
function context(): MerchantOptionPricingContext {
  return {
    fieldKey: "validation_method",
    quantity: false,
    options: [{ value: "dns", label: { zhCN: "验证", enUS: "Validation" } }],
    billingRules: [rule],
    currencies: ["CNY"],
    targets: [],
    prices: [
      {
        priceNo: "base",
        configurationPriceDimensionNo: "",
        billingRuleNo: "monthly",
        currencyCode: "CNY",
        amountMinor: "9900",
        setupFeeMinor: "1000",
        renewalAmountMinor: "9900",
        minimumQuantity: 0,
        maximumQuantity: null,
        status: "active",
      } as CatalogPrice,
    ],
  };
}
describe("atomic merchant price drafts", () => {
  it("does not silently use base prices or turn unspecified extra charges into zero", () => {
    const draft = initialMerchantPriceDraft(context(), "dns", rule, "CNY");
    expect(draft.previousPriceNo).toBe("");
    expect(draft.tiers[0]?.amount).toBe("");
    expect(() => merchantPriceDraftInput(draft, rule)).toThrow("amount");
    draft.tiers[0]!.amount = "0";
    draft.tiers[0]!.renewal = "0";
    expect(merchantPriceDraftInput(draft, rule).tiers).toEqual([
      {
        minimumQuantity: 0,
        maximumQuantity: null,
        amountMinor: "0",
        setupFeeMinor: "0",
        renewalAmountMinor: "0",
      },
    ]);
  });
  it("joins current fields by identity and exact currency, including identical labels", () => {
    const data = context();
    data.targets = [
      {
        fieldKey: "validation_method",
        candidateValue: "dns",
        quantity: false,
        dimensionNo: "correct",
        pricingMode: "fixed",
      },
      {
        fieldKey: "other",
        candidateValue: "dns",
        quantity: false,
        dimensionNo: "other",
        pricingMode: "fixed",
      },
    ];
    data.prices.push(
      {
        ...data.prices[0]!,
        priceNo: "option",
        configurationPriceDimensionNo: "correct",
        amountMinor: "123",
        renewalAmountMinor: "456",
        status: "inactive",
      },
      {
        ...data.prices[0]!,
        priceNo: "other",
        configurationPriceDimensionNo: "other",
        amountMinor: "900",
      },
    );
    const draft = initialMerchantPriceDraft(data, "dns", rule, "CNY");
    expect(draft).toMatchObject({
      previousPriceNo: "option",
      active: false,
      tiers: [{ amount: "1.23", setup: "10.00", renewal: "4.56" }],
    });
    expect(
      initialMerchantPriceDraft(data, "dns", rule, "JPY").previousPriceNo,
    ).toBe("");
    expect(merchantPriceDraftKey(draft)).not.toBe(
      merchantPriceDraftKey({ ...draft, fieldKey: "other" }),
    );
  });
  it("keeps capacity bands and exact large money, rejects gaps and excessive decimals", () => {
    const draft: MerchantPriceDraft = {
      fieldKey: "capacity",
      candidateValue: "",
      quantity: true,
      pricingMode: "per_unit",
      billingRuleNo: "monthly",
      currencyCode: "CNY",
      previousPriceNo: "",
      active: true,
      tiers: [
        {
          minimumQuantity: 0,
          maximumQuantity: 30,
          amount: "0",
          setup: "0",
          renewal: "0",
        },
        {
          minimumQuantity: 30,
          maximumQuantity: null,
          amount: "92233720368547758.07",
          setup: "0",
          renewal: "0.01",
        },
      ],
    };
    expect(merchantPriceDraftInput(draft, rule).tiers[1]?.amountMinor).toBe(
      "9223372036854775807",
    );
    draft.tiers[1]!.minimumQuantity = 31;
    expect(() => merchantPriceDraftInput(draft, rule)).toThrow("range");
    draft.tiers[1]!.minimumQuantity = 30;
    draft.tiers[1]!.amount = "1.001";
    expect(() => merchantPriceDraftInput(draft, rule)).toThrow("amount");
  });
});

describe("graduated quantity drafts", () => {
  it("preserves exact marginal band rates and rejects a repeated setup fee", () => {
    const draft: MerchantPriceDraft = {
      fieldKey: "local_transfer",
      candidateValue: "",
      quantity: true,
      pricingMode: "graduated",
      billingRuleNo: "monthly",
      currencyCode: "CNY",
      previousPriceNo: "",
      active: true,
      tiers: [
        {
          minimumQuantity: 0,
          maximumQuantity: 10,
          amount: "0",
          renewal: "0",
          setup: "0.05",
        },
        {
          minimumQuantity: 10,
          maximumQuantity: null,
          amount: "2",
          renewal: "3",
          setup: "0",
        },
      ],
    };
    const saved = merchantPriceDraftInput(draft, rule);
    expect(saved.pricingMode).toBe("graduated");
    expect(
      saved.tiers.map((tier) => [
        tier.minimumQuantity,
        tier.maximumQuantity,
        tier.amountMinor,
        tier.renewalAmountMinor,
        tier.setupFeeMinor,
      ]),
    ).toEqual([
      [0, 10, "0", "0", "5"],
      [10, null, "200", "300", "0"],
    ]);
    draft.tiers[1]!.setup = "0.01";
    expect(() => merchantPriceDraftInput(draft, rule)).toThrow("setup");
    expect(draft.tiers[1]!.setup).toBe("0.01");
  });
});
