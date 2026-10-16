// This file lists owned local trials with a single bounded batch of formal
// schedules. Current version mismatch leaves no purchasable option; quotation
// still revalidates all authoritative price and configuration facts.

package renewal

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func (s *serviceImpl) ListTrialConversions(ctx context.Context, page, pageSize int) (*TrialConversionPage, error) {
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if page < 1 || page > 100000 || pageSize < 1 || pageSize > 100 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	result := &TrialConversionPage{Items: []TrialConversionItem{}}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		model := s.database.Model(do.ServiceEntitlement{}).Ctx(ctx).Where(do.ServiceEntitlement{TenantId: 0, CustomerNo: audience.CustomerNo, ChargeTypeCode: "trial", EntitlementOwnerId: "idc-resource", RenewalModeCode: "local_entitlement", UpstreamServiceNo: ""}).WhereIn("status_code", []string{"active", "suspended"})
		count, err := model.Clone().Count()
		if err != nil {
			return err
		}
		result.Total = count
		var rows []struct {
			TrialConversionItem
			VersionNo string
		}
		if err = s.database.GetScan(ctx, &rows, `SELECT e.entitlement_no,COALESCE(item.product_name_zh_cn,product.name_zh_cn,'') AS product_name_zh_cn,COALESCE(item.product_name_en_us,product.name_en_us,'') AS product_name_en_us,
 e.currency_code,e.status_code,e.expires_at,
 CASE WHEN product.status_code='active' AND product.current_version_no=e.version_no AND product.deleted_at IS NULL
 THEN e.version_no ELSE '' END AS version_no
 FROM plugin_idc_core_service_entitlement e
 LEFT JOIN plugin_idc_core_commerce_order_item item ON item.tenant_id=e.tenant_id AND item.order_item_no=e.order_item_no AND item.order_no=e.order_no
 LEFT JOIN plugin_idc_core_catalog_product product ON product.tenant_id=e.tenant_id AND product.product_no=e.product_no
 WHERE e.tenant_id=0 AND e.customer_no=? AND e.charge_type_code='trial' AND e.entitlement_owner_id='idc-resource'
 AND e.renewal_mode_code='local_entitlement' AND e.upstream_service_no='' AND e.status_code IN ('active','suspended')
 ORDER BY e.id DESC LIMIT ? OFFSET ?`, audience.CustomerNo, pageSize, (page-1)*pageSize); err != nil {
			return err
		}
		versions := make([]string, 0, len(rows))
		for _, row := range rows {
			if row.VersionNo != "" {
				versions = append(versions, row.VersionNo)
			}
		}
		periods := make([]struct {
			TrialConversionPeriod
			VersionNo string
		}, 0)
		if len(versions) > 0 && s.recovery != nil {
			if err = s.database.Model(do.CatalogBillingRule{}).Ctx(ctx).Fields("version_no,billing_rule_no,period_unit_code,period_count").Where(do.CatalogBillingRule{TenantId: 0, ChargeTypeCode: "recurring", StatusCode: "active"}).WhereIn("version_no", versions).WhereNot("price_phase_code", "renewal").OrderAsc("period_count").OrderAsc("billing_rule_no").Limit(2001).Scan(&periods); err != nil {
				return err
			}
			if len(periods) > 2000 {
				return bizerr.NewCode(CodeUnavailable)
			}
		}
		byVersion := make(map[string][]TrialConversionPeriod, len(versions))
		for _, period := range periods {
			byVersion[period.VersionNo] = append(byVersion[period.VersionNo], period.TrialConversionPeriod)
		}
		for _, row := range rows {
			row.Periods = append([]TrialConversionPeriod{}, byVersion[row.VersionNo]...)
			result.Items = append(result.Items, row.TrialConversionItem)
		}
		return nil
	})
	return result, normalizeError(err)
}
