// This file reads a bounded page of menu rows with the ancestor context required
// to render a two-level tree; it never fetches a separate detail per node.
package productdirectory

import (
	"context"
	"lina-plugin-idc-core/backend/internal/model/do"
	"strings"
)

// ListAdminTree checks the same shared merchant permission as the flat list.
func (s *serviceImpl) ListAdminTree(ctx context.Context, page, size int) (out *Page, boundaryErr error) {
	defer func() { boundaryErr = directoryError(boundaryErr) }()
	if _, err := s.access.Admin(ctx, PermissionList); err != nil {
		return nil, err
	}
	return s.list(ctx, page, size, "", "", true)
}

// readTreePage keeps real row pagination and adds missing parents in one query.
func (s *serviceImpl) readTreePage(ctx context.Context, page, size int, result *Page) error {
	model := s.database.Model(do.ProductDirectory{}).Ctx(ctx).As("d").
		LeftJoin("plugin_idc_core_product_directory p", "p.tenant_id=d.tenant_id AND p.directory_no=d.parent_directory_no").
		Where("d.tenant_id", 0).
		Fields("d." + strings.ReplaceAll(directoryFields, ",", ",d.")).
		OrderAsc("COALESCE(p.sort_order,d.sort_order)").OrderAsc("COALESCE(p.id,d.id)").
		OrderAsc("CASE WHEN d.parent_directory_no='' THEN 0 ELSE 1 END").OrderAsc("d.sort_order").OrderAsc("d.id")
	if err := model.Page(page, size).Scan(&result.Items); err != nil {
		return err
	}
	var (
		present = make(map[string]bool, len(result.Items))
		missing = make([]string, 0, len(result.Items))
	)
	for _, item := range result.Items {
		present[item.DirectoryNo] = true
	}
	for _, item := range result.Items {
		if item.ParentDirectoryNo != "" && !present[item.ParentDirectoryNo] {
			missing = append(missing, item.ParentDirectoryNo)
			present[item.ParentDirectoryNo] = true
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return s.database.Model(do.ProductDirectory{}).Ctx(ctx).Where(do.ProductDirectory{TenantId: 0}).
		WhereIn("directory_no", missing).Fields(directoryFields).OrderAsc("sort_order").OrderAsc("id").Scan(&result.Parents)
}
