// This file reads the bounded active-category selector for staff catalog writes.

package catalog

import (
	"context"
	"encoding/json"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// ListCategories authorizes before validating filters or reading persistence.
func (s *productManagementReaderImpl) ListCategories(
	ctx context.Context,
	deletionState string,
) ([]CategoryManagementSummary, error) {
	if _, err := s.authorizer.AuthorizeCategoryList(ctx); err != nil {
		return nil, err
	}
	if deletionState != "" && deletionState != "active" && deletionState != "deleted" && deletionState != "all" {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	columns := dao.CatalogCategory.Columns()
	query := dao.CatalogCategory.Ctx(ctx).Fields(
		columns.CategoryNo, columns.ParentCategoryNo,
		columns.NameZhCn, columns.NameEnUs, columns.SortOrder, columns.StatusCode,
		columns.Version, columns.DeletedAt,
	).Where(do.CatalogCategory{
		TenantId: idccap.DefaultTenantID, StatusCode: catalogStatusActive,
	}).Where(columns.CategoryNo+" ~ ?", categoryNumberPattern.String()).
		Where("(COALESCE("+columns.ParentCategoryNo+", '') = '' OR "+columns.ParentCategoryNo+" ~ ?)", categoryNumberPattern.String())
	if deletionState == "deleted" || deletionState == "all" {
		query = query.Unscoped()
	}
	if deletionState == "deleted" {
		query = query.WhereNotNull(columns.DeletedAt)
	}
	var rows []entity.CatalogCategory
	if err := query.OrderAsc(columns.SortOrder).OrderAsc(columns.Id).
		Limit(maximumCatalogCategoryCount + 1).Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if len(rows) > maximumCatalogCategoryCount {
		return nil, bizerr.NewCode(CodeManagementUnavailable)
	}
	result := make([]CategoryManagementSummary, 0, len(rows))
	for _, row := range rows {
		parent := ""
		if row.ParentCategoryNo != nil {
			parent = *row.ParentCategoryNo
		}
		item := CategoryManagementSummary{
			CategoryNo: row.CategoryNo, ParentCategoryNo: parent,
			NameZhCN: row.NameZhCn,
			NameEnUS: row.NameEnUs, SortOrder: row.SortOrder, Status: row.StatusCode,
			Version: row.Version, Deleted: row.DeletedAt != nil,
		}
		if !categoryNumberPattern.MatchString(item.CategoryNo) ||
			(item.ParentCategoryNo != "" && !categoryNumberPattern.MatchString(item.ParentCategoryNo)) ||
			item.Status != catalogStatusActive || item.Version < 1 ||
			!validLocalizedText(item.NameZhCN, 128, false) ||
			!validLocalizedText(item.NameEnUS, 128, false) {
			return nil, bizerr.NewCode(CodeManagementUnavailable)
		}
		result = append(result, item)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if len(encoded) > maximumCatalogCategoryResponseBytes {
		return nil, bizerr.NewCode(CodeManagementUnavailable)
	}
	return result, nil
}
