// Native structured targets compose the existing customer quote and compiler
// freezer in one transaction. Product, currency and cycles come from the owned
// entitlement; public input never carries raw supplier fields or a cart identity.

package serveroperation

import (
	"context"
	"encoding/json"
	"lina-plugin-idc-core/backend/internal/service/principal"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (s *nativeCommercialService) QuoteTarget(ctx context.Context, input servercommercialcap.StructuredTargetInput) (*servercommercialcap.TargetQuote, error) {
	if s == nil || len(input.Values) > 128 || nilServerOperationDependency(s.targetQuotes) {
		return nil, PublicError(ErrInvalidInput)
	}
	if err := s.authorizeNativeTarget(ctx, input.Target.CustomerNo, serveroperationresourcecap.ActionResize); err != nil {
		return nil, err
	}
	var result *servercommercialcap.TargetQuote
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		target, err := s.readNativeCommercialTarget(ctx, input.Target, true)
		if err != nil {
			return err
		}
		if target.PurchaseCycles < 1 || !nativeSnapshotAllows(input.Target, serveroperationresourcecap.ActionResize) {
			return ErrActionUnavailable
		}
		busy, err := s.nativeCommercialBusy(ctx, target.EntitlementNo, "")
		if err != nil {
			return err
		}
		if busy {
			return ErrOperationConflict
		}
		values := nativeTypedPurchaseValues(input.Values)
		purchase := commerce.AccountTargetQuoteInput{EntitlementNo: target.EntitlementNo, Purchase: catalog.PurchaseConfigurationInput{PublicSlug: target.PublicSlug, ConfigurationRevision: input.ConfigurationRevision, BillingTermToken: input.BillingTermToken, CurrencyCode: target.CurrencyCode, PurchaseCycles: target.PurchaseCycles, Values: values}}
		audience, err := s.pricing.principal.Current(ctx)
		if err != nil || audience == nil {
			return ErrActionUnavailable
		}
		var quote *commerce.Quote
		if audience.Code == principal.AudienceStaff {
			quote, err = s.targetQuotes.QuoteStaffTarget(ctx, target.CustomerNo, purchase)
		} else {
			quote, err = s.targetQuotes.QuoteAccountTarget(ctx, purchase)
		}
		if err != nil {
			return err
		}
		if quote == nil || len(quote.Items) != 1 || quote.Items[0].Quantity != 1 || quote.CurrencyCode != target.CurrencyCode {
			return ErrOperationConflict
		}
		freeze := commerce.FreezeQuoteItemConfigurationInput{QuoteItemNo: quote.Items[0].QuoteItemNo, Purchase: catalog.PublicPurchaseConfigurationInput{PublicSlug: target.PublicSlug, ConfigurationRevision: input.ConfigurationRevision, Values: values}}
		var frozen *commerce.FrozenQuoteItemConfiguration
		if audience.Code == principal.AudienceStaff {
			frozen, err = s.targetQuotes.FreezeStaffTargetQuoteItemConfiguration(ctx, target.CustomerNo, target.EntitlementNo, freeze)
		} else {
			frozen, err = s.targetQuotes.FreezeAccountQuoteItemConfiguration(ctx, freeze)
		}
		if err != nil {
			return err
		}
		if frozen == nil || !frozen.ConfigurationFrozen || frozen.SnapshotNo == "" {
			return ErrOperationConflict
		}
		encoded, err := json.Marshal(input.Target)
		if err != nil {
			return err
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_native_server_target_quote
          (tenant_id,target_quote_no,quote_item_no,structured_configuration_no,entitlement_no,customer_no,expected_entitlement_version,customer_subject_version,currency_code,native_target_snapshot)
          VALUES (0,?,?,?,?,?,?,?,?,CAST(? AS jsonb))`, quote.QuoteNo, quote.Items[0].QuoteItemNo, frozen.SnapshotNo, target.EntitlementNo, target.CustomerNo, target.EntitlementVersion, target.CustomerVersion, target.CurrencyCode, string(encoded)); err != nil {
			return err
		}
		result = &servercommercialcap.TargetQuote{QuoteNo: quote.QuoteNo, QuoteItemNo: quote.Items[0].QuoteItemNo, StructuredConfigurationNo: frozen.SnapshotNo}
		return nil
	})
	return result, PublicError(err)
}

// nativeTypedPurchaseValues preserves every opaque compiler field exactly.
func nativeTypedPurchaseValues(values []servercommercialcap.TypedPurchaseValue) []catalog.PublicPurchaseConfigurationValue {
	result := make([]catalog.PublicPurchaseConfigurationValue, len(values))
	for i, v := range values {
		result[i] = catalog.PublicPurchaseConfigurationValue{FieldToken: v.FieldToken, Type: v.Type, TextValue: v.TextValue, IntegerValue: v.IntegerValue, DecimalValue: v.DecimalValue, BooleanValue: v.BooleanValue, DateValue: v.DateValue, DateTimeValue: v.DateTimeValue, OptionTokens: append([]string(nil), v.OptionTokens...), TreeOptionTokens: append([]string(nil), v.TreeOptionTokens...), AddressOptionTokens: append([]string(nil), v.AddressOptionTokens...), FileTokens: append([]string(nil), v.FileTokens...)}
	}
	return result
}
