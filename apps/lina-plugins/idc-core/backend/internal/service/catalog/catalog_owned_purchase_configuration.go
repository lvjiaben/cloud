// Owned configuration reads reuse the public compiler projection. The caller
// authorizes the entitlement; this boundary independently checks frozen SCF
// provenance and the exact published product version before resolving choices.
package catalog

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

// OwnedPurchaseConfigurationInput identifies a current source-owned SCF. Values
// nil restores the compiler snapshot; non-nil reevaluates an opaque public edit.
type OwnedPurchaseConfigurationInput struct {
	EntitlementNo, CustomerNo, SnapshotNo, PublicSlug, ProductNo, VersionNo, CurrencyCode, BillingRuleNo string
	ConfigurationRevision                                                                                string
	Values                                                                                               []PublicPurchaseConfigurationValue
	PeriodUnitCode                                                                                       string
	PeriodCount, PurchaseCycles                                                                          int
}

// OwnedPurchaseConfigurationContext includes no raw snapshot or final money.
type OwnedPurchaseConfigurationContext struct {
	PublicPurchaseConfigurationEvaluation
	BillingTermToken string
}

// OwnedPurchaseConfigurationReader is a trusted owner-only read boundary.
type OwnedPurchaseConfigurationReader interface {
	// ReadOwnedPurchaseConfiguration restores the exact current SCF and projects
	// fresh public candidates; forged provenance and stale revisions fail closed.
	ReadOwnedPurchaseConfiguration(context.Context, OwnedPurchaseConfigurationInput) (*OwnedPurchaseConfigurationContext, error)
}

func (s *publicCatalogReaderImpl) ReadOwnedPurchaseConfiguration(ctx context.Context, in OwnedPurchaseConfigurationInput) (*OwnedPurchaseConfigurationContext, error) {
	var frozen *struct{ ConfigurationRevision, TypedValues string }
	err := s.database.GetScan(ctx, &frozen, `SELECT sc.configuration_revision,sc.typed_value_snapshot::TEXT AS typed_values
 FROM plugin_idc_core_service_entitlement e
 JOIN plugin_idc_core_commerce_structured_configuration_snapshot sc ON sc.tenant_id=e.tenant_id AND sc.snapshot_no=e.configuration_snapshot_no
 JOIN plugin_idc_core_catalog_product p ON p.tenant_id=e.tenant_id AND p.product_no=e.product_no AND p.current_version_no=e.version_no AND p.status_code='active' AND p.deleted_at IS NULL
 WHERE e.tenant_id=0 AND e.entitlement_no=? AND e.customer_no=? AND sc.snapshot_no=? AND sc.customer_no=e.customer_no
 AND e.product_no=? AND e.version_no=? AND sc.product_no=e.product_no AND sc.version_no=e.version_no
 AND ((sc.order_no=e.order_no AND sc.order_item_no=e.order_item_no)
 OR EXISTS(SELECT 1 FROM plugin_idc_core_native_server_target_quote binding WHERE binding.tenant_id=e.tenant_id AND binding.entitlement_no=e.entitlement_no AND binding.structured_configuration_no=sc.snapshot_no AND binding.customer_no=e.customer_no))
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_commerce_structured_configuration_file_snapshot f WHERE f.tenant_id=sc.tenant_id AND f.snapshot_no=sc.snapshot_no)`, in.EntitlementNo, in.CustomerNo, in.SnapshotNo, in.ProductNo, in.VersionNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if frozen == nil {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	snapshot, err := s.loadStructuredConfigurationForEvaluation(ctx, in.PublicSlug, frozen.ConfigurationRevision)
	if err != nil {
		return nil, err
	}
	if snapshot.Target.ProductNo != in.ProductNo || snapshot.Target.VersionNo != in.VersionNo || snapshot.Revision != frozen.ConfigurationRevision || (in.ConfigurationRevision != "" && in.ConfigurationRevision != snapshot.Revision) {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	values, err := configurationcontract.DecodeCompiledValues(snapshot.Schema, []byte(frozen.TypedValues))
	if err != nil {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	if in.Values != nil {
		values, err = s.decodePublicStructuredValues(snapshot, in.Values)
	}
	if err != nil {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	for _, value := range values {
		if len(value.FileTokens) > 0 {
			return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
		}
	}
	evaluation, resolved, err := s.evaluateInteractiveStructuredConfiguration(ctx, snapshot, values)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	view, err := s.projectStructuredConfiguration(snapshot, evaluation, resolved)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	result := &OwnedPurchaseConfigurationContext{PublicPurchaseConfigurationEvaluation: PublicPurchaseConfigurationEvaluation{PublicPurchaseConfigurationView: *view, Valid: evaluation.Valid, Issues: []PublicPurchaseConfigurationIssue{}}}
	for _, issue := range evaluation.Issues {
		token, tokenErr := s.revision.encodeStructuredField(snapshot.Revision, issue.FieldKey)
		if tokenErr != nil {
			return nil, tokenErr
		}
		result.Issues = append(result.Issues, PublicPurchaseConfigurationIssue{FieldToken: token, IssueCode: issue.Code})
	}
	if !evaluation.Valid {
		if err = s.recheckStructuredConfigurationRevision(ctx, snapshot); err != nil {
			return nil, err
		}
		return result, nil
	}
	quoted, err := s.QuotePurchaseConfiguration(ctx, PublicPurchaseConfigurationQuoteInput{PublicPurchaseConfigurationInput: PublicPurchaseConfigurationInput{PublicSlug: in.PublicSlug, ConfigurationRevision: snapshot.Revision, Values: view.Values}, CurrencyCode: in.CurrencyCode})
	if err != nil {
		return nil, err
	}
	selections, selectionDigest, err := loadSelectedConfigurationPriceDimensions(ctx, s.database, snapshot.Target.VersionNo, evaluation.Values, snapshot.Target.CustomerPresentation)
	if err != nil {
		return nil, err
	}
	if len(selections) > 128 {
		return nil, bizerr.NewCode(CodePublicConfigurationInvalid)
	}
	expectedTerm, err := s.revision.encodeStructuredQuoteTerm(in.PublicSlug, in.VersionNo, in.BillingRuleNo, selectionDigest)
	if err != nil {
		return nil, err
	}
	for _, term := range quoted.Terms {
		if term.BillingTermToken == expectedTerm && string(term.ChargeType) == string(ChargeTypeRecurring) && string(term.PeriodUnit) == in.PeriodUnitCode && term.PeriodCount == in.PeriodCount && in.PurchaseCycles >= term.MinimumPurchaseCycles {
			if result.BillingTermToken != "" {
				return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
			}
			result.BillingTermToken = term.BillingTermToken
		}
	}
	if result.BillingTermToken == "" {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	if err = s.recheckStructuredConfigurationRevision(ctx, snapshot); err != nil {
		return nil, err
	}
	return result, nil
}
