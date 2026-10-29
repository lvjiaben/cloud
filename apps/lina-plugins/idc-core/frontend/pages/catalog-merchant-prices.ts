// Draft option prices stay local until the configuration owner atomically
// submits their exact currency, billing rule, choice and schedule together.
import type {
  CatalogBillingRule,
  CatalogMerchantPriceRow,
  CatalogMerchantPriceTarget,
  CatalogPrice,
} from "./catalog-client";
import type { CatalogConfigurationCandidate } from "./catalog-configuration-state";
import { majorToMinorAmount, minorToMajorAmount } from "./catalog-money";

export interface MerchantPriceTierDraft {
  minimumQuantity: number;
  maximumQuantity: number | null;
  amount: string;
  setup: string;
  renewal: string;
}
export interface MerchantPriceDraft {
  fieldKey: string;
  candidateValue: string;
  quantity: boolean;
  pricingMode: "fixed" | "per_unit" | "band_total" | "graduated";
  billingRuleNo: string;
  currencyCode: string;
  previousPriceNo: string;
  active: boolean;
  tiers: MerchantPriceTierDraft[];
}
export interface MerchantOptionPricingContext {
  fieldKey: string;
  quantity: boolean;
  options: CatalogConfigurationCandidate[];
  billingRules: CatalogBillingRule[];
  currencies: string[];
  prices: CatalogPrice[];
  targets: CatalogMerchantPriceTarget[];
}
export class MerchantPriceDraftError extends Error {
  constructor(
    public reason: "amount" | "range" | "limit" | "permission" | "setup",
  ) {
    super(reason);
  }
}
export function merchantPriceDraftKey(
  value: Pick<
    MerchantPriceDraft,
    | "fieldKey"
    | "candidateValue"
    | "quantity"
    | "billingRuleNo"
    | "currencyCode"
  >,
) {
  return JSON.stringify([
    value.fieldKey,
    value.quantity,
    value.candidateValue,
    value.billingRuleNo,
    value.currencyCode,
  ]);
}
export function initialMerchantPriceDraft(
  context: MerchantOptionPricingContext,
  value: string,
  rule: CatalogBillingRule,
  currency: string,
): MerchantPriceDraft {
  const target = context.targets.find(
    (target) =>
      target.fieldKey === context.fieldKey &&
      target.quantity === context.quantity &&
      (context.quantity || target.candidateValue === value),
  );
  const existing = target?.dimensionNo
    ? context.prices
        .filter(
          (price) =>
            price.configurationPriceDimensionNo === target.dimensionNo &&
            price.billingRuleNo === rule.billingRuleNo &&
            price.currencyCode === currency,
        )
        .sort((a, b) => a.minimumQuantity - b.minimumQuantity)
    : [];
  return {
    fieldKey: context.fieldKey,
    candidateValue: context.quantity ? "" : value,
    quantity: context.quantity,
    pricingMode:
      target?.pricingMode || (context.quantity ? "per_unit" : "fixed"),
    billingRuleNo: rule.billingRuleNo,
    currencyCode: currency,
    previousPriceNo: existing[0]?.priceNo || "",
    active: existing[0]?.status !== "inactive",
    tiers: existing.length
      ? existing.map((price) => ({
          minimumQuantity: price.minimumQuantity,
          maximumQuantity: price.maximumQuantity,
          amount: minorToMajorAmount(price.amountMinor, currency),
          setup: minorToMajorAmount(price.setupFeeMinor, currency),
          renewal:
            price.renewalAmountMinor === null
              ? ""
              : minorToMajorAmount(price.renewalAmountMinor, currency),
        }))
      : [
          {
            minimumQuantity: 0,
            maximumQuantity: null,
            amount: "",
            setup: "",
            renewal: "",
          },
        ],
  };
}
export function merchantPriceDraftInput(
  draft: MerchantPriceDraft,
  rule: CatalogBillingRule,
): CatalogMerchantPriceRow {
  if (!draft.tiers.length || draft.tiers.length > 256)
    throw new MerchantPriceDraftError("limit");
  const tiers = draft.tiers.map((tier, index) => {
    if (
      !Number.isSafeInteger(tier.minimumQuantity) ||
      tier.minimumQuantity < 0 ||
      (index === 0 && tier.minimumQuantity !== 0) ||
      (tier.maximumQuantity !== null &&
        (!Number.isSafeInteger(tier.maximumQuantity) ||
          tier.maximumQuantity <= tier.minimumQuantity)) ||
      (index < draft.tiers.length - 1 &&
        tier.maximumQuantity !== draft.tiers[index + 1]?.minimumQuantity) ||
      (index === draft.tiers.length - 1 && tier.maximumQuantity !== null)
    )
      throw new MerchantPriceDraftError("range");
    try {
      return {
        minimumQuantity: tier.minimumQuantity,
        maximumQuantity: tier.maximumQuantity,
        amountMinor: majorToMinorAmount(
          rule.pricePhase === "renewal" && !tier.amount.trim()
            ? "0"
            : tier.amount,
          draft.currencyCode,
        ),
        setupFeeMinor: majorToMinorAmount(
          tier.setup || "0",
          draft.currencyCode,
        ),
        ...(["both", "renewal"].includes(rule.pricePhase)
          ? {
              renewalAmountMinor: majorToMinorAmount(
                tier.renewal,
                draft.currencyCode,
              ),
            }
          : {}),
      };
    } catch {
      throw new MerchantPriceDraftError("amount");
    }
  });
  if (
    draft.pricingMode === "graduated" &&
    tiers.some((tier, index) => index > 0 && tier.setupFeeMinor !== "0")
  )
    throw new MerchantPriceDraftError("setup");
  return {
    fieldKey: draft.fieldKey,
    candidateValue: draft.candidateValue,
    quantity: draft.quantity,
    pricingMode: draft.pricingMode,
    active: draft.active,
    previousPriceNo: draft.previousPriceNo,
    billingRuleNo: draft.billingRuleNo,
    currencyCode: draft.currencyCode,
    tiers,
  };
}
