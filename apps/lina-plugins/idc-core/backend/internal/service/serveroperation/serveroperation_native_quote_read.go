// Native quote reads expose immutable commercial facts without reading legacy service records.

package serveroperation

import (
	"context"
	"encoding/json"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type nativeAccessoryQuoteRecord struct {
	AccessoryQuote
	Parameters, RequestDigest, ContentDigest, BaselineJSON string
}

const nativeAccessoryQuoteSelect = `SELECT quote.quote_no,quote.customer_no,quote.subject_type_code,quote.subject_no,
quote.expected_subject_version,quote.expected_service_version,quote.native_target_snapshot::TEXT AS native_target_snapshot,
quote.action_code AS action,quote.accessory_kind AS kind,quote.currency_code,quote.requested_quantity,quote.existing_object_count,
quote.reserved_create_count,quote.free_count_per_server,quote.charged_quantity,quote.unit_amount_minor,
quote.subtotal_minor,quote.net_amount_minor,quote.tax_amount_minor,quote.total_amount_minor,quote.period_unit_code,
quote.period_count,quote.policy_no,quote.policy_version,quote.customer_subject_version,quote.baseline_object_nos::TEXT AS baseline_json,
quote.baseline_snapshot_no,quote.baseline_observed_at,quote.issued_at,quote.expires_at,
quote.parameters::TEXT AS parameters,quote.request_digest,quote.content_digest,version.traffic_gb,
version.name_zh_cn AS package_name_zh_cn,version.name_en_us AS package_name_en_us
FROM plugin_idc_core_server_accessory_quote quote
JOIN plugin_idc_core_server_accessory_price_policy_version version
 ON version.policy_no=quote.policy_no AND version.policy_version=quote.policy_version
WHERE quote.tenant_id=0 AND quote.customer_no=? AND quote.subject_type_code='entitlement' AND quote.service_no IS NULL`

func (s *nativeCommercialService) loadNativeAccessoryQuote(ctx context.Context, customerNo, idempotencyDigest string) (*AccessoryQuote, string, error) {
	row, err := s.readNativeAccessoryQuote(ctx, customerNo, "", idempotencyDigest)
	if err != nil || row == nil {
		return nil, "", err
	}
	return &row.AccessoryQuote, row.RequestDigest, nil
}

func (s *nativeCommercialService) readNativeAccessoryQuote(ctx context.Context, customerNo, quoteNo, idempotencyDigest string) (*nativeAccessoryQuoteRecord, error) {
	var rows []nativeAccessoryQuoteRecord
	if err := s.database.GetScan(ctx, &rows, nativeAccessoryQuoteSelect+` AND (quote.quote_no=? OR quote.idempotency_digest=?) LIMIT 2`, customerNo, quoteNo, idempotencyDigest); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 {
		return nil, ErrOperationConflict
	}
	if json.Unmarshal([]byte(rows[0].BaselineJSON), &rows[0].BaselineObjectNos) != nil {
		return nil, ErrOperationConflict
	}
	return &rows[0], nil
}

// ListTrafficPackages returns at most 100 published packages with merchant inventory for this exact owned product.
func (s *nativeCommercialService) ListTrafficPackages(ctx context.Context, target servercommercialcap.TargetSnapshot) ([]servercommercialcap.TrafficPackage, error) {
	if err := s.authorizeNativeTarget(ctx, target.CustomerNo, serveroperationresourcecap.ActionTrafficAdd); err != nil {
		return nil, err
	}
	core, err := s.readNativeCommercialTarget(ctx, target, false)
	if err != nil {
		return nil, err
	}
	if !nativeSnapshotAllows(target, serveroperationresourcecap.ActionTrafficAdd) {
		return nil, ErrActionUnavailable
	}
	var rows []TrafficPackage
	if err = s.pricing.trafficPackageModel(ctx).Fields(trafficPackageProjection).Where("policy.enabled", true).
		Where("policy.currency_code", core.CurrencyCode).Where(`EXISTS(SELECT 1 FROM plugin_idc_core_traffic_package_product scope WHERE scope.tenant_id=0 AND scope.policy_no=policy.policy_no AND scope.policy_version=policy.current_version AND scope.product_no=?)`, core.ProductNo).
		OrderAsc("policy.policy_no").Limit(100).Scan(&rows); err != nil {
		return nil, err
	}
	if err = s.pricing.fillTrafficPackageFacts(ctx, rows); err != nil {
		return nil, err
	}
	result := make([]servercommercialcap.TrafficPackage, len(rows))
	for i, row := range rows {
		result[i] = servercommercialcap.TrafficPackage{PackageNo: row.PackageNo, NameZhCN: row.NameZhCN, NameEnUS: row.NameEnUS, CurrencyCode: row.CurrencyCode, TrafficGB: row.TrafficGB, AmountMinor: row.AmountMinor, RemainingStock: row.RemainingQuantity, UnlimitedStock: row.StockQuantity == nil, PolicyVersion: row.Version}
	}
	return result, nil
}

const legacyAccessorySubject = "service"

func accessorySubjectType(quote *AccessoryQuote) string {
	if quote.SubjectTypeCode == nativeAccessorySubject {
		return nativeAccessorySubject
	}
	return legacyAccessorySubject
}
func accessorySubjectNo(quote *AccessoryQuote) string {
	if quote.SubjectTypeCode == nativeAccessorySubject {
		return quote.SubjectNo
	}
	return quote.ServiceNo
}
func accessorySubjectVersion(quote *AccessoryQuote) int64 {
	if quote.SubjectTypeCode == nativeAccessorySubject {
		return quote.ExpectedSubjectVersion
	}
	return quote.ExpectedServiceVersion
}
func accessoryLegacyService(quote *AccessoryQuote) any {
	if quote.SubjectTypeCode == nativeAccessorySubject {
		return nil
	}
	return quote.ServiceNo
}
func accessoryNativeSnapshot(quote *AccessoryQuote) string {
	if quote.SubjectTypeCode == nativeAccessorySubject {
		return quote.NativeTargetSnapshot
	}
	return "{}"
}
