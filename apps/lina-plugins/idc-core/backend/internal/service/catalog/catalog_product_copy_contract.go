// This file preserves imported source contracts and local fulfillment policy.
// Automation bindings are deliberately copied by their owning service, later
// in the same transaction, before the new draft can become visible.

package catalog

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *managementServiceImpl) copyProductImportAndPolicy(ctx context.Context, source, target string, rules map[string]string, actor int) error {
	db := dao.CatalogProduct.DB()
	var contract *entity.CatalogImportContract
	if err := dao.CatalogImportContract.Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: source}).Scan(&contract); err != nil {
		return err
	}
	if contract != nil {
		result, err := dao.CatalogImportContract.Ctx(ctx).Data(do.CatalogImportContract{TenantId: 0, VersionNo: target, InterfaceInstanceNo: contract.InterfaceInstanceNo, Snapshot: contract.Snapshot, SnapshotDigest: contract.SnapshotDigest, ReadEvidence: contract.ReadEvidence}).Insert()
		if err = requireSingleCatalogInsert(result, err, "copy immutable import contract"); err != nil {
			return err
		}
	}
	var terms []entity.CatalogImportTerm
	if err := dao.CatalogImportTerm.Ctx(ctx).Where(do.CatalogImportTerm{TenantId: 0, VersionNo: source}).OrderAsc("id").Limit(4097).Scan(&terms); err != nil {
		return err
	}
	if len(terms) > 4096 {
		return bizerr.NewCode(CodeManagementConflict)
	}
	rows := make([]do.CatalogImportTerm, 0, len(terms))
	for _, term := range terms {
		rule := rules[term.BillingRuleNo]
		if rule == "" {
			return bizerr.NewCode(CodeManagementConflict)
		}
		rows = append(rows, do.CatalogImportTerm{TenantId: 0, VersionNo: target, BillingRuleNo: rule, CurrencyCode: term.CurrencyCode, UpstreamTermReference: term.UpstreamTermReference, WholeItemRule: optionalStringPointer(term.WholeItemRule), MarkupPercent: term.MarkupPercent})
	}
	if len(rows) > 0 {
		result, err := dao.CatalogImportTerm.Ctx(ctx).Data(rows).Batch(256).Insert()
		if err = requireCatalogBatchInsert(result, err, len(rows), "copy imported billing terms"); err != nil {
			return err
		}
	}
	var candidates []entity.CatalogImportCandidate
	if err := dao.CatalogImportCandidate.Ctx(ctx).Where(do.CatalogImportCandidate{TenantId: 0, VersionNo: source}).OrderAsc("id").Limit(4097).Scan(&candidates); err != nil {
		return err
	}
	if len(candidates) > 4096 {
		return bizerr.NewCode(CodeManagementConflict)
	}
	candidateRows := make([]do.CatalogImportCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		candidateRows = append(candidateRows, do.CatalogImportCandidate{TenantId: 0, VersionNo: target, FieldKey: candidate.FieldKey, CandidateValue: candidate.CandidateValue, OptionToken: candidate.OptionToken})
	}
	if len(candidateRows) > 0 {
		result, err := dao.CatalogImportCandidate.Ctx(ctx).Data(candidateRows).Batch(256).Insert()
		if err = requireCatalogBatchInsert(result, err, len(candidateRows), "copy frozen import candidates"); err != nil {
			return err
		}
	}
	_, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy(tenant_id,version_no,strategy_code,granularity_code,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest,created_by,ongoing_management,renewal_mode_code,renewal_method,entitlement_owner_id) SELECT tenant_id,?,strategy_code,granularity_code,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest,?,ongoing_management,renewal_mode_code,renewal_method,entitlement_owner_id FROM plugin_idc_core_catalog_product_version_fulfillment_policy WHERE tenant_id=0 AND version_no=?`, target, actor, source)
	return err
}
