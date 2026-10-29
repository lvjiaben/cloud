// This file defines a generic imported-product procurement plan independently
// of server hardware. It contains only an owner-frozen purchase; provider
// correlation, callback secrets and execution authority remain resource-owned.
package resourcecap

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

// NativePurchasePlanVersion identifies the explicit imported purchase arm of
// provision.parameters.build. It never changes the existing server plan shape.
const NativePurchasePlanVersion = "native-purchase-v1"

// NativePurchasePlan transports one already-paid quantity unit with the source
// configuration and billing mappings frozen by the product owner. It cannot
// change the source account, identity, callback or execution idempotency key.
type NativePurchasePlan struct {
	Protocol productfulfillmentcap.SaleImportProtocol `json:"protocol"`
	Version  string                                   `json:"version"`
	Purchase channelcommercecap.Purchase              `json:"nativePurchase"`
}

// Validate bounds the generic purchase before connector invocation. Scalar and
// candidate semantics remain the exact frozen source contract's responsibility.
func (plan NativePurchasePlan) Validate() error {
	purchase := plan.Purchase
	input := channelcommercecap.ConfigurationInput{PublicSlug: purchase.PublicSlug, ConfigurationRevision: purchase.ConfigurationRevision, CurrencyCode: purchase.CurrencyCode, Values: purchase.Values}
	if plan.Version != NativePurchasePlanVersion || !plan.Protocol.NativePurchase() || input.Validate() != nil || len(purchase.CurrencyCode) != 3 || len(purchase.BillingTermToken) == 0 || len(purchase.BillingTermToken) > 4096 || purchase.PurchaseCycles != 1 || purchase.Quantity != 1 {
		return gerror.New("native purchase plan is invalid")
	}
	raw, err := json.Marshal(plan)
	if err != nil || len(raw) > 1<<20 {
		return gerror.New("native purchase plan exceeds payload limit")
	}
	return nil
}

// BuildNativePurchaseParameters validates the immutable non-server purchase and
// the connector's bounded JSON result before persistence. It lends no authority
// to create an order and cannot add callback or secret material to the plan.
func BuildNativePurchaseParameters(ctx context.Context, builder func(context.Context, NativePurchasePlan) (json.RawMessage, error), plan NativePurchasePlan) (json.RawMessage, error) {
	if ctx == nil || ctx.Err() != nil || builder == nil || plan.Validate() != nil {
		return nil, gerror.New("native purchase parameter input is invalid")
	}
	parameters, err := builder(ctx, plan)
	if err != nil {
		return nil, gerror.New("native purchase parameter mapper failed")
	}
	if err = validateJSONObject("native purchase parameters", parameters, false); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), parameters...), nil
}
