// This file reads bounded merchant and customer projections with fixed-count
// batched joins. Merchant navigation never changes purchased service access.
package productdirectory

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/do"
	"strings"
	"time"
)

const directoryFields = "directory_no,name_zh_cn,name_en_us,sort_order,status_code,version"

func validPage(page, size int) bool { return page >= 1 && page <= 1000000 && size >= 1 && size <= 100 }

// ListAdmin enforces ownership before accessing directory records.
func (s *serviceImpl) ListAdmin(ctx context.Context, page, size int) (out *Page, boundaryErr error) {
	defer func() { boundaryErr = directoryError(boundaryErr) }()
	if _, err := s.access.Admin(ctx, PermissionList); err != nil {
		return nil, err
	}
	return s.list(ctx, page, size, "")
}

// ListAccount enforces ownership before accessing directory records.
func (s *serviceImpl) ListAccount(ctx context.Context, page, size int) (out *Page, boundaryErr error) {
	defer func() { boundaryErr = directoryError(boundaryErr) }()
	customerNo, err := s.access.Customer(ctx)
	if err != nil {
		return nil, err
	}
	if customerNo == "" {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.list(ctx, page, size, customerNo)
}
func (s *serviceImpl) list(ctx context.Context, page, size int, customerNo string) (*Page, error) {
	if !validPage(page, size) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	model := s.database.Model(do.ProductDirectory{}).Ctx(ctx).Where(do.ProductDirectory{TenantId: 0})
	if customerNo != "" {
		model = model.Where(do.ProductDirectory{StatusCode: StatusEnabled})
	}
	total, err := model.Clone().Count()
	if err != nil {
		return nil, err
	}
	result := &Page{Items: []Directory{}, Total: total, Page: page, PageSize: size}
	if err = model.Fields(directoryFields).OrderAsc("sort_order").OrderAsc("id").Page(page, size).Scan(&result.Items); err != nil {
		return nil, err
	}
	if len(result.Items) == 0 {
		return result, nil
	}
	var (
		nos       = make([]any, len(result.Items))
		positions = make(map[string]int, len(result.Items))
	)
	for i := range result.Items {
		nos[i] = result.Items[i].DirectoryNo
		positions[result.Items[i].DirectoryNo] = i
		result.Items[i].ProductNos = []string{}
		result.Items[i].Products = []Product{}
	}
	if customerNo == "" {
		var links []struct{ DirectoryNo, ProductNo, NameZhCn, NameEnUs string }
		err = s.database.GetScan(ctx, &links, `SELECT m.directory_no,m.product_no,p.name_zh_cn,p.name_en_us FROM plugin_idc_core_product_directory_membership m JOIN plugin_idc_core_catalog_product p ON p.tenant_id=m.tenant_id AND p.product_no=m.product_no WHERE m.tenant_id=0 AND m.directory_no IN (`+placeholders(len(nos))+`) ORDER BY m.directory_no,m.product_no`, nos...)
		if err != nil {
			return nil, err
		}
		for _, link := range links {
			i := positions[link.DirectoryNo]
			result.Items[i].ProductNos = append(result.Items[i].ProductNos, link.ProductNo)
			result.Items[i].Products = append(result.Items[i].Products, Product{ProductNo: link.ProductNo, NameZhCn: link.NameZhCn, NameEnUs: link.NameEnUs})
			result.Items[i].ProductCount++
		}
	} else {
		var facts []struct {
			DirectoryNo  string
			ServiceCount int
		}
		args := append([]any{customerNo}, nos...)
		err = s.database.GetScan(ctx, &facts, `SELECT d.directory_no,COUNT(e.entitlement_no) AS service_count FROM plugin_idc_core_product_directory d LEFT JOIN plugin_idc_core_product_directory_membership m ON m.tenant_id=d.tenant_id AND m.directory_no=d.directory_no LEFT JOIN plugin_idc_core_service_entitlement e ON e.tenant_id=m.tenant_id AND e.product_no=m.product_no AND e.customer_no=? WHERE d.tenant_id=0 AND d.directory_no IN (`+placeholders(len(nos))+`) GROUP BY d.directory_no`, args...)
		if err != nil {
			return nil, err
		}
		for _, fact := range facts {
			i := positions[fact.DirectoryNo]
			result.Items[i].ServiceCount = fact.ServiceCount
		}
	}
	return result, nil
}
func placeholders(count int) string { return strings.TrimSuffix(strings.Repeat("?,", count), ",") }

// ListServices enforces ownership before accessing directory records.
func (s *serviceImpl) ListServices(ctx context.Context, directoryNo string, page, size int) (out *ServicePage, boundaryErr error) {
	defer func() { boundaryErr = directoryError(boundaryErr) }()
	if !validPage(page, size) || !referencePattern.MatchString(directoryNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	customerNo, err := s.access.Customer(ctx)
	if err != nil {
		return nil, err
	}
	if customerNo == "" {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	where := `e.tenant_id=0 AND e.customer_no=?`
	args := []any{customerNo}
	if directoryNo != AllDirectory {
		count, countErr := s.database.Model(do.ProductDirectory{}).Ctx(ctx).Where(do.ProductDirectory{TenantId: 0, DirectoryNo: directoryNo, StatusCode: StatusEnabled}).Count()
		if countErr != nil {
			return nil, countErr
		}
		if count != 1 {
			return nil, bizerr.NewCode(CodeNotAccessible)
		}
		where += ` AND EXISTS (SELECT 1 FROM plugin_idc_core_product_directory_membership m WHERE m.tenant_id=e.tenant_id AND m.product_no=e.product_no AND m.directory_no=?)`
		args = append(args, directoryNo)
	}
	total, err := s.database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_service_entitlement e WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		EntitlementNo, ProductNo, ProductNameZhCn, ProductNameEnUs, PluginId, StatusCode, OrderNo string
		StartedAt                                                                                 time.Time
		ExpiresAt                                                                                 *time.Time
	}
	args = append(args, size, (page-1)*size)
	err = s.database.GetScan(ctx, &rows, `SELECT e.entitlement_no,e.product_no,COALESCE(i.product_name_zh_cn,p.name_zh_cn,'') AS product_name_zh_cn,COALESCE(i.product_name_en_us,p.name_en_us,'') AS product_name_en_us,e.plugin_id,e.status_code,e.order_no,e.started_at,e.expires_at FROM plugin_idc_core_service_entitlement e LEFT JOIN plugin_idc_core_product_fulfillment_intent i ON i.tenant_id=e.tenant_id AND i.intent_no=e.source_intent_no LEFT JOIN plugin_idc_core_catalog_product p ON p.tenant_id=e.tenant_id AND p.product_no=e.product_no WHERE `+where+` ORDER BY e.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	result := &ServicePage{Items: []Entitlement{}, Total: total.Int(), Page: page, PageSize: size}
	for _, row := range rows {
		item := Entitlement{EntitlementNo: row.EntitlementNo, ProductNo: row.ProductNo, ProductNameZhCn: row.ProductNameZhCn, ProductNameEnUs: row.ProductNameEnUs, PluginId: row.PluginId, StatusCode: row.StatusCode, OrderNo: row.OrderNo, StartedAt: row.StartedAt.UnixMilli()}
		if row.ExpiresAt != nil {
			stamp := row.ExpiresAt.UnixMilli()
			item.ExpiresAt = &stamp
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}
