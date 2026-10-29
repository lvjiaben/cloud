// Native form context performs authorization and version reads around catalog
// evaluation without holding entitlement locks across candidate owner I/O.
package serveroperation

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

var _ servercommercialcap.ConfigurationContextReader = (*nativeCommercialService)(nil)

func (s *nativeCommercialService) ReadConfigurationContext(ctx context.Context, in servercommercialcap.ConfigurationContextRequest) (*servercommercialcap.ConfigurationContext, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || s.prices == nil || len(in.Values) > 128 || (in.Values != nil && in.ConfigurationRevision == "") || gdb.TXFromCtx(ctx, s.database.GetGroup()) != nil {
		return nil, PublicError(ErrInvalidInput)
	}
	if err := s.authorizeNativeTarget(ctx, in.Target.CustomerNo, serveroperationresourcecap.ActionResize); err != nil {
		return nil, err
	}
	if !nativeSnapshotAllows(in.Target, serveroperationresourcecap.ActionResize) {
		return nil, PublicError(ErrActionUnavailable)
	}
	target, err := s.readNativeCommercialTarget(ctx, in.Target, false)
	if err != nil {
		return nil, err
	}
	if target.StructuredConfigurationNo == "" || target.PurchaseCycles < 1 {
		return nil, PublicError(ErrActionUnavailable)
	}
	reader, ok := s.prices.catalog.(catalog.OwnedPurchaseConfigurationReader)
	if !ok || nilServerOperationDependency(reader) {
		return nil, PublicError(ErrActionUnavailable)
	}
	var values []catalog.PublicPurchaseConfigurationValue
	if in.Values != nil {
		values = nativeTypedPurchaseValues(in.Values)
	}
	projected, err := reader.ReadOwnedPurchaseConfiguration(ctx, catalog.OwnedPurchaseConfigurationInput{EntitlementNo: target.EntitlementNo, CustomerNo: target.CustomerNo, SnapshotNo: target.StructuredConfigurationNo, PublicSlug: target.PublicSlug, ProductNo: target.ProductNo, VersionNo: target.VersionNo, CurrencyCode: target.CurrencyCode, BillingRuleNo: target.BillingRuleNo, ConfigurationRevision: in.ConfigurationRevision, Values: values, PeriodUnitCode: target.PeriodUnitCode, PeriodCount: target.PeriodCount, PurchaseCycles: target.PurchaseCycles})
	if err != nil {
		return nil, PublicError(err)
	}
	if projected == nil {
		return nil, PublicError(ErrOperationConflict)
	}
	// Authorization may have changed while the candidate owner was resolving.
	if err = s.authorizeNativeTarget(ctx, in.Target.CustomerNo, serveroperationresourcecap.ActionResize); err != nil {
		return nil, err
	}
	current, err := s.readNativeCommercialTarget(ctx, in.Target, false)
	if err != nil {
		return nil, err
	}
	if current.CustomerVersion != target.CustomerVersion || current.StructuredConfigurationNo != target.StructuredConfigurationNo || current.ProductNo != target.ProductNo || current.VersionNo != target.VersionNo || current.BillingRuleNo != target.BillingRuleNo || current.CurrencyCode != target.CurrencyCode || current.PeriodUnitCode != target.PeriodUnitCode || current.PeriodCount != target.PeriodCount || current.PurchaseCycles != target.PurchaseCycles {
		return nil, PublicError(ErrOperationConflict)
	}
	result := &servercommercialcap.ConfigurationContext{EntitlementNo: target.EntitlementNo, EntitlementVersion: target.EntitlementVersion, StructuredConfigurationNo: target.StructuredConfigurationNo, CurrencyCode: target.CurrencyCode, PurchaseCycles: target.PurchaseCycles, BillingTermToken: projected.BillingTermToken, ConfigurationRevision: projected.ConfigurationRevision, Ready: projected.Valid}
	// Both types mirror the existing public projection. JSON cannot cross this
	// boundary with raw compiler keys; no raw snapshot participates in conversion.
	encoded, err := json.Marshal(struct {
		Fields []catalog.PublicPurchaseConfigurationField
		Values []catalog.PublicPurchaseConfigurationValue
		Issues []catalog.PublicPurchaseConfigurationIssue
	}{projected.Fields, projected.Values, projected.Issues})
	if err != nil {
		return nil, PublicError(err)
	}
	if err = json.Unmarshal(encoded, result); err != nil {
		return nil, PublicError(err)
	}
	for _, value := range result.Values {
		if len(value.FileTokens) > 0 {
			return nil, PublicError(ErrActionUnavailable)
		}
	}
	return result, nil
}
