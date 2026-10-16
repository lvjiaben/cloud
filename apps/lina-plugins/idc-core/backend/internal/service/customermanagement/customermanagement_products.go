// This file pages purchased order items and their delivered products without cross-plugin reads.

package customermanagement

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

// productDeliveryKind identifies the authoritative delivery owner, independent of number prefixes.
type productDeliveryKind string

const (
	productDeliveryServer      productDeliveryKind = "server"
	productDeliveryEntitlement productDeliveryKind = "entitlement"
	productDeliveryPending     productDeliveryKind = "pending"
)

// productLifecycleState contains closed delivery states excluded from expiry reminders.
type productLifecycleState string

const (
	productLifecycleTerminated productLifecycleState = "terminated"
	productLifecycleCancelled  productLifecycleState = "cancelled"
	productLifecycleRefunded   productLifecycleState = "refunded"
)

// purchasePurpose selects original purchases instead of counting renewals as new products.
type purchasePurpose string

const purchasePurposeNew purchasePurpose = "new_purchase"

// ProductItem combines a frozen order line with its optional current delivery.
type ProductItem struct {
	Number          string
	DeliveryKind    productDeliveryKind
	OrderNo         string
	ProductNo       string
	NameZhCn        string
	NameEnUs        string
	Information     string
	InformationEnUs string
	ConfigurationNo string
	AmountMinor     int64
	CurrencyCode    string
	ProductType     string
	PluginID        string
	PeriodUnit      string
	PeriodCount     int
	CreatedAt       time.Time
	ExpiresAt       *time.Time
	Status          string
}

// ProductPage returns a bounded slice and exact customer-scoped count.
type ProductPage struct {
	Items []ProductItem
	Total int
}

// SearchProducts checks staff and customer access before paging all purchased types.
func (s *serviceImpl) SearchProducts(ctx context.Context, customerNo string, page, size int) (*ProductPage, error) {
	if err := s.authorize(ctx, permissionCustomerQuery); err != nil {
		return nil, err
	}
	if !validPersistedCustomerNo(customerNo) || page < 1 || page > 1000000 || size < 1 || size > s.config.MaximumPageSize {
		return nil, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	countCustomer, err := dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{TenantId: 0, CustomerNo: customerNo}).Count()
	if err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	if countCustomer != 1 {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}

	// A pending line remains visible; delivered units use their own number and lifecycle.
	base := customerProductsFrom
	result := &ProductPage{Items: []ProductItem{}}
	count, err := dao.CommerceOrderItem.DB().GetValue(ctx, "SELECT COUNT(*)"+base, customerNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	result.Total = count.Int()
	query := `SELECT COALESCE(s.service_no,e.entitlement_no,i.order_item_no) AS number,CASE WHEN s.service_no IS NOT NULL THEN ? WHEN e.entitlement_no IS NOT NULL THEN ? ELSE ? END AS delivery_kind,o.order_no,i.product_no,
 i.product_name_zh_cn AS name_zh_cn,i.product_name_en_us AS name_en_us,
 COALESCE(NULLIF(s.customer_alias,''),NULLIF(s.customer_remark,''),'') AS information,
 COALESCE(p.total_amount_minor,i.total_amount_minor) AS amount_minor,i.currency_code,
 ` + customerProductTypeProjection + ` AS product_type,
 ` + customerProductPluginProjection + ` AS plugin_id,COALESCE(s.period_unit_code,e.period_unit_code,i.period_unit_code) AS period_unit,
 COALESCE(s.period_count,e.period_count,i.period_count) AS period_count,o.created_at,
 COALESCE(s.entitlement_expires_at,e.expires_at) AS expires_at,
 COALESCE(s.configuration_snapshot_no,e.configuration_snapshot_no,i.configuration_snapshot_no) AS configuration_no,
 COALESCE(s.entitlement_status_code,e.status_code,o.status_code) AS status` + base
	// Price snapshots are immutable and local to the commerce owner.
	query += fmt.Sprintf(" ORDER BY o.created_at DESC,i.id DESC,number LIMIT %d OFFSET %d", size, (page-1)*size)
	if err = dao.CommerceOrderItem.DB().GetScan(ctx, &result.Items, query, productDeliveryServer, productDeliveryEntitlement, productDeliveryPending, customerNo); err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	if err = s.decorateProductInformation(ctx, result.Items); err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	return result, nil
}

// Shared category projections keep overview classification identical to the product list.
const (
	customerProductTypeProjection   = "COALESCE(NULLIF(s.product_type_code,''),NULLIF(e.entitlement_owner_id,''),b.plugin_id,'')"
	customerProductPluginProjection = "COALESCE(e.plugin_id,b.plugin_id,'')"
)

const customerProductsFrom = ` FROM plugin_idc_core_commerce_order_item i
 JOIN plugin_idc_core_commerce_order o ON o.tenant_id=i.tenant_id AND o.order_no=i.order_no
 JOIN plugin_idc_core_commerce_quote_snapshot q ON q.tenant_id=o.tenant_id AND q.quote_no=o.quote_no AND q.customer_no=o.customer_no
 LEFT JOIN plugin_idc_core_catalog_product_version_fulfillment_binding b ON b.tenant_id=i.tenant_id AND b.version_no=i.version_no
 LEFT JOIN plugin_idc_core_service_instance s ON s.tenant_id=i.tenant_id AND s.order_item_no=i.order_item_no AND s.customer_no=o.customer_no
 LEFT JOIN plugin_idc_core_service_entitlement e ON e.tenant_id=i.tenant_id AND e.order_item_no=i.order_item_no AND e.customer_no=o.customer_no
 LEFT JOIN plugin_idc_core_service_price_snapshot p ON p.tenant_id=i.tenant_id AND p.price_snapshot_no=COALESCE(s.current_price_snapshot_no,e.current_price_snapshot_no)
 WHERE i.tenant_id=0 AND o.customer_no=? AND q.purpose_code='` + string(purchasePurposeNew) + `'`

// decorateProductInformation reads safe frozen labels for the current page in one bounded query.
// Raw configuration values, provider identifiers and credentials never leave this projection.
func (s *serviceImpl) decorateProductInformation(ctx context.Context, items []ProductItem) error {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if item.ConfigurationNo != "" {
			keys = append(keys, item.ConfigurationNo)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	var rows []struct {
		ConfigurationSnapshotNo, OptionNameZhCn, OptionNameEnUs string
		Quantity                                                int
	}
	err := dao.CommerceConfigurationSnapshotItem.Ctx(ctx).Fields("configuration_snapshot_no,option_name_zh_cn,option_name_en_us,quantity").Where(do.CommerceConfigurationSnapshotItem{TenantId: 0}).WhereIn("configuration_snapshot_no", keys).OrderAsc("sort_order").OrderAsc("id").Limit(len(keys)*128 + 1).Scan(&rows)
	if err != nil {
		return err
	}
	if len(rows) > len(keys)*128 {
		return bizerr.NewCode(customer.CodeUnavailable)
	}
	zh, en := map[string][]string{}, map[string][]string{}
	for _, row := range rows {
		if len(zh[row.ConfigurationSnapshotNo]) >= 8 {
			continue
		}
		suffix := ""
		if row.Quantity > 1 {
			suffix = fmt.Sprintf(" × %d", row.Quantity)
		}
		if row.OptionNameZhCn != "" {
			zh[row.ConfigurationSnapshotNo] = append(zh[row.ConfigurationSnapshotNo], row.OptionNameZhCn+suffix)
		}
		if row.OptionNameEnUs != "" {
			en[row.ConfigurationSnapshotNo] = append(en[row.ConfigurationSnapshotNo], row.OptionNameEnUs+suffix)
		}
	}
	for i := range items {
		note := items[i].Information
		items[i].Information = strings.Join(zh[items[i].ConfigurationNo], " / ")
		items[i].InformationEnUs = strings.Join(en[items[i].ConfigurationNo], " / ")
		if note != "" {
			if items[i].Information != "" {
				items[i].Information += " · "
			}
			items[i].Information += note
			if items[i].InformationEnUs != "" {
				items[i].InformationEnUs += " · "
			}
			items[i].InformationEnUs += note
		}
	}
	return nil
}
