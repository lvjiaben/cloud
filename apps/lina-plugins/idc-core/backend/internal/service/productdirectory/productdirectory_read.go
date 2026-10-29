// This file reads bounded merchant and customer projections with fixed-count
// batched joins. Merchant navigation never changes purchased service access.

package productdirectory

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/entitlementprogress"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// paidServiceCondition requires authoritative settlement, independent of entitlement lifecycle or price.
const paidServiceCondition = `EXISTS (SELECT 1 FROM plugin_idc_core_commerce_order paid_order WHERE paid_order.tenant_id=e.tenant_id AND paid_order.customer_no=e.customer_no AND paid_order.order_no=e.order_no AND paid_order.paid_at IS NOT NULL)`

// serviceProviderExpression keeps native owners together and lets generic managed services use their fulfillment plugin page.
const serviceProviderExpression = `CASE WHEN e.entitlement_owner_id IS NULL OR e.entitlement_owner_id IN ('','` + idccap.OwnerPluginID + `') THEN e.plugin_id ELSE e.entitlement_owner_id END`

// providerPattern bounds plugin block references before persistence reads.
var providerPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const directoryFields = "directory_no,name_zh_cn,name_en_us,sort_order,status_code,version,navigation_type,target_url,icon,parent_directory_no"

// Existing owner states are only used to derive a safe boolean projection;
// the service lifecycle and fulfillment intent are never rewritten by this reader.
type entitlementLifecycle string
type fulfillmentStage string

const (
	lifecyclePendingActivation entitlementLifecycle = "pending_activation"
	stageManualPending         fulfillmentStage     = "manual_pending"
	stageIncomplete            fulfillmentStage     = "incomplete"
)

func validPage(page, size int) bool { return page >= 1 && page <= 1000000 && size >= 1 && size <= 100 }

// ListAdmin enforces ownership before accessing directory records.
func (s *serviceImpl) ListAdmin(ctx context.Context, page, size int) (out *Page, boundaryErr error) {
	defer func() { boundaryErr = directoryError(boundaryErr) }()
	if _, err := s.access.Admin(ctx, PermissionList); err != nil {
		return nil, err
	}
	return s.list(ctx, page, size, "", "", false)
}

// ListAccount enforces ownership before accessing directory records.
func (s *serviceImpl) ListAccount(ctx context.Context, page, size int, navigationType NavigationType) (out *Page, boundaryErr error) {
	defer func() { boundaryErr = directoryError(boundaryErr) }()
	customerNo, err := s.access.Customer(ctx)
	if err != nil {
		return nil, err
	}
	if customerNo == "" {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.list(ctx, page, size, customerNo, navigationType, false)
}
func (s *serviceImpl) list(ctx context.Context, page, size int, customerNo string, navigationType NavigationType, tree bool) (*Page, error) {
	if !validPage(page, size) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	switch navigationType {
	case "", NavigationProduct, NavigationSystem, NavigationCustom, NavigationPlugin:
	default:
		return nil, bizerr.NewCode(CodeInvalid)
	}
	model := s.database.Model(do.ProductDirectory{}).Ctx(ctx).Where(do.ProductDirectory{TenantId: 0})
	if customerNo != "" {
		model = model.Where(do.ProductDirectory{StatusCode: StatusEnabled}).Where("(parent_directory_no='' OR EXISTS (SELECT 1 FROM plugin_idc_core_product_directory parent WHERE parent.tenant_id=plugin_idc_core_product_directory.tenant_id AND parent.directory_no=plugin_idc_core_product_directory.parent_directory_no AND parent.status_code=?))", StatusEnabled)
	}
	if navigationType != "" {
		model = model.Where(do.ProductDirectory{NavigationType: string(navigationType)})
	}
	total, err := model.Clone().Count()
	if err != nil {
		return nil, err
	}
	result := &Page{Items: []Directory{}, Total: total, Page: page, PageSize: size}
	if tree {
		err = s.readTreePage(ctx, page, size, result)
	} else {
		err = model.Fields(directoryFields).OrderAsc("sort_order").OrderAsc("id").Page(page, size).Scan(&result.Items)
	}
	if err != nil {
		return nil, err
	}
	if len(result.Items) == 0 {
		return result, nil
	}
	var (
		projected = make([]*Directory, 0, len(result.Items)+len(result.Parents))
		nos       = make([]any, 0, cap(projected))
		positions = make(map[string]*Directory, cap(projected))
	)
	for i := range result.Items {
		projected = append(projected, &result.Items[i])
	}
	for i := range result.Parents {
		projected = append(projected, &result.Parents[i])
	}
	for _, item := range projected {
		nos = append(nos, item.DirectoryNo)
		positions[item.DirectoryNo] = item
	}
	if customerNo == "" {
		var facts []struct {
			DirectoryNo  string
			ProductCount int
		}
		err = s.database.GetScan(ctx, &facts, `SELECT directory_no,COUNT(*) AS product_count FROM plugin_idc_core_product_directory_membership WHERE tenant_id=0 AND directory_no IN (`+placeholders(len(nos))+`) GROUP BY directory_no`, nos...)
		if err != nil {
			return nil, err
		}
		for _, fact := range facts {
			positions[fact.DirectoryNo].ProductCount = fact.ProductCount
		}
	} else {
		var facts []struct {
			DirectoryNo  string
			ServiceCount int
		}
		args := append([]any{customerNo}, nos...)
		err = s.database.GetScan(ctx, &facts, `SELECT d.directory_no,COUNT(e.entitlement_no) AS service_count FROM plugin_idc_core_product_directory d LEFT JOIN plugin_idc_core_product_directory_membership m ON m.tenant_id=d.tenant_id AND m.directory_no=d.directory_no LEFT JOIN plugin_idc_core_service_entitlement e ON e.tenant_id=m.tenant_id AND e.product_no=m.product_no AND e.customer_no=? AND `+paidServiceCondition+` WHERE d.tenant_id=0 AND d.directory_no IN (`+placeholders(len(nos))+`) GROUP BY d.directory_no`, args...)
		if err != nil {
			return nil, err
		}
		for _, fact := range facts {
			positions[fact.DirectoryNo].ServiceCount = fact.ServiceCount
		}
	}
	if err = s.attachPagePlugins(ctx, projected); err != nil {
		return nil, err
	}
	return result, nil
}
func placeholders(count int) string { return strings.TrimSuffix(strings.Repeat("?,", count), ",") }

// ListServices enforces ownership before accessing directory records.
func (s *serviceImpl) ListServices(ctx context.Context, directoryNo string, page, size int, keywords ...string) (*ServicePage, error) {
	if len(keywords) > 1 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	keyword := ""
	if len(keywords) == 1 {
		keyword = keywords[0]
	}
	return s.ListProviderServices(ctx, directoryNo, page, size, keyword, "")
}

// ListProviderServices filters paid, customer-owned entitlements before grouping and pagination.
func (s *serviceImpl) ListProviderServices(ctx context.Context, directoryNo string, page, size int, keyword, providerID string) (out *ServicePage, boundaryErr error) {
	defer func() { boundaryErr = directoryError(boundaryErr) }()
	if !validPage(page, size) || !referencePattern.MatchString(directoryNo) || (providerID != "" && (!providerPattern.MatchString(providerID) || len(providerID) > 128)) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	customerNo, err := s.access.Customer(ctx)
	if err != nil {
		return nil, err
	}
	if customerNo == "" {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	where := `e.tenant_id=0 AND e.customer_no=? AND ` + paidServiceCondition
	args := []any{customerNo}
	if directoryNo != AllDirectory {
		count, countErr := s.database.Model(do.ProductDirectory{}).Ctx(ctx).Where(do.ProductDirectory{TenantId: 0, DirectoryNo: directoryNo, StatusCode: StatusEnabled, NavigationType: NavigationProduct}).Count()
		if countErr != nil {
			return nil, countErr
		}
		if count != 1 {
			return nil, bizerr.NewCode(CodeNotAccessible)
		}
		where += ` AND EXISTS (SELECT 1 FROM plugin_idc_core_product_directory_membership m WHERE m.tenant_id=e.tenant_id AND m.product_no=e.product_no AND m.directory_no=?)`
		args = append(args, directoryNo)
	}
	keyword = strings.TrimSpace(keyword)
	if providerID != "" {
		where += ` AND (` + serviceProviderExpression + `)=?`
		args = append(args, providerID)
	}
	if utf8.RuneCountInString(keyword) > 100 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if keyword != "" {
		literal := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(keyword) + "%"
		where += ` AND (e.entitlement_no ILIKE ? ESCAPE '\' OR e.customer_remark ILIKE ? ESCAPE '\' OR EXISTS (SELECT 1 FROM plugin_idc_core_product_fulfillment_intent search_i WHERE search_i.tenant_id=e.tenant_id AND search_i.customer_no=e.customer_no AND search_i.intent_no=e.source_intent_no AND (search_i.product_name_zh_cn ILIKE ? ESCAPE '\' OR search_i.product_name_en_us ILIKE ? ESCAPE '\')) OR EXISTS (SELECT 1 FROM plugin_idc_core_catalog_product search_p WHERE search_p.tenant_id=e.tenant_id AND search_p.product_no=e.product_no AND (search_p.name_zh_cn ILIKE ? ESCAPE '\' OR search_p.name_en_us ILIKE ? ESCAPE '\')))`
		for range 6 {
			args = append(args, literal)
		}
	}
	var providers []ServiceProvider
	if err := s.database.GetScan(ctx, &providers, `SELECT `+serviceProviderExpression+` AS plugin_id,COUNT(*) AS total FROM plugin_idc_core_service_entitlement e WHERE `+where+` GROUP BY `+serviceProviderExpression+` ORDER BY plugin_id`, args...); err != nil {
		return nil, err
	}
	configured := &Directory{DirectoryNo: directoryNo}
	if directoryNo != AllDirectory {
		if err := s.attachPagePlugins(ctx, []*Directory{configured}); err != nil {
			return nil, err
		}
	}
	ordered := make([]ServiceProvider, 0, len(providers)+len(configured.PagePluginIDs))
	used := make(map[string]bool)
	for _, id := range configured.PagePluginIDs {
		if providerID != "" && providerID != id {
			continue
		}
		item := ServiceProvider{PluginID: id}
		for _, actual := range providers {
			if actual.PluginID == id {
				item.Total = actual.Total
				break
			}
		}
		ordered = append(ordered, item)
		used[id] = true
	}
	for _, item := range providers {
		if !used[item.PluginID] {
			ordered = append(ordered, item)
		}
	}
	providers = ordered
	total, err := s.database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_service_entitlement e WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		EntitlementNo, ProductNo, ProductNameZhCn, ProductNameEnUs, PluginId, StatusCode, OrderNo string
		EntitlementOwnerID                                                                        string
		ManagementDeliveryState                                                                   string
		CustomerRemark                                                                            string
		CustomerRemarkRevision                                                                    int64
		CurrentPriceAmountMinor                                                                   *int64
		CurrencyCode, PeriodUnitCode                                                              string
		PeriodCount, ChargeCycles                                                                 int
		RequiresManualProcessing                                                                  bool
		StartedAt                                                                                 time.Time
		ExpiresAt                                                                                 *time.Time
	}
	// The already-bounded owner join supplies one boolean, with no per-row lookup
	// or provider calls. Customer equality also protects frozen name projections.
	queryArgs := append([]any{lifecyclePendingActivation, stageManualPending, stageIncomplete}, args...)
	queryArgs = append(queryArgs, size, (page-1)*size)
	err = s.database.GetScan(ctx, &rows, `SELECT e.entitlement_no,e.customer_remark,e.customer_remark_revision,price.total_amount_minor AS current_price_amount_minor,COALESCE(price.currency_code,'') AS currency_code,COALESCE(price.period_unit_code,'') AS period_unit_code,COALESCE(price.period_count,0) AS period_count,COALESCE(price.purchase_cycles,0) AS charge_cycles,e.product_no,COALESCE(i.product_name_zh_cn,p.name_zh_cn,'') AS product_name_zh_cn,COALESCE(i.product_name_en_us,p.name_en_us,'') AS product_name_en_us,e.plugin_id,e.entitlement_owner_id,e.status_code,COALESCE(event.status_code,'') AS management_delivery_state,COALESCE(e.status_code=? AND i.status_code IN (?,?),FALSE) AS requires_manual_processing,e.order_no,e.started_at,e.expires_at
FROM plugin_idc_core_service_entitlement e
LEFT JOIN plugin_idc_core_service_price_snapshot price ON price.tenant_id=e.tenant_id AND price.customer_no=e.customer_no AND price.price_snapshot_no=e.current_price_snapshot_no
LEFT JOIN plugin_idc_core_product_fulfillment_intent i ON i.tenant_id=e.tenant_id AND i.customer_no=e.customer_no AND i.intent_no=e.source_intent_no
LEFT JOIN plugin_idc_core_catalog_product p ON p.tenant_id=e.tenant_id AND p.product_no=e.product_no
LEFT JOIN plugin_idc_core_entitlement_created_outbox event ON event.tenant_id=e.tenant_id AND event.entitlement_no=e.entitlement_no AND event.intent_no=e.source_intent_no AND event.customer_no=e.customer_no AND event.owner_id=e.entitlement_owner_id
WHERE `+where+` ORDER BY e.id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, err
	}
	result := &ServicePage{Providers: providers, Items: []Entitlement{}, Total: total.Int(), Page: page, PageSize: size}
	for _, row := range rows {
		if row.StatusCode == string(lifecyclePendingActivation) {
			row.ManagementDeliveryState = ""
		}
		item := Entitlement{EntitlementOwnerID: row.EntitlementOwnerID, EntitlementNo: row.EntitlementNo, ProductNo: row.ProductNo, ProductNameZhCn: row.ProductNameZhCn, ProductNameEnUs: row.ProductNameEnUs, PluginId: row.PluginId, StatusCode: row.StatusCode, RequiresManualProcessing: row.RequiresManualProcessing, ManagementPreparation: string(entitlementprogress.FromDelivery(row.ManagementDeliveryState)), OrderNo: row.OrderNo, StartedAt: row.StartedAt.UnixMilli()}
		item.CustomerRemark = row.CustomerRemark
		item.CustomerRemarkRevision = row.CustomerRemarkRevision
		item.CurrencyCode = row.CurrencyCode
		item.PeriodUnitCode = row.PeriodUnitCode
		item.PeriodCount = row.PeriodCount
		item.ChargeCycles = row.ChargeCycles
		if row.CurrentPriceAmountMinor != nil {
			amount := strconv.FormatInt(*row.CurrentPriceAmountMinor, 10)
			item.CurrentPriceAmountMinor = &amount
		}
		if row.ExpiresAt != nil {
			stamp := row.ExpiresAt.UnixMilli()
			item.ExpiresAt = &stamp
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}
