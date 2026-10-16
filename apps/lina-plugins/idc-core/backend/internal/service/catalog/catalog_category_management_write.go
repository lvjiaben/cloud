// This file owns audited catalog-category maintenance commands.

package catalog

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

type normalizedCategoryUpdate struct {
	categoryNo       string
	parentCategoryNo string
	nameZhCN         string
	nameEnUS         string
	sortOrder        int
	version          int64
}

func (s *managementServiceImpl) UpdateCategory(
	ctx context.Context,
	input CategoryUpdateInput,
) (*Category, error) {
	actorUserID, err := s.authorizer.AuthorizeCategoryUpdate(ctx, strings.TrimSpace(input.CategoryNo))
	if err != nil {
		return nil, err
	}
	if actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	normalized, err := normalizeCategoryUpdate(input)
	if err != nil {
		return nil, err
	}
	var result *Category
	err = dao.CatalogCategory.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := lockCatalogCategoryTree(ctx); lockErr != nil {
			return lockErr
		}
		parentByCategory, treeErr := loadCatalogCategoryTreeForUpdate(ctx)
		if treeErr != nil {
			return treeErr
		}
		row, loadErr := loadCategoryForMaintenance(ctx, normalized.categoryNo, false)
		if loadErr != nil {
			return loadErr
		}
		if row.Version != normalized.version {
			return bizerr.NewCode(CodeManagementConflict)
		}
		if normalized.parentCategoryNo != "" {
			if parentErr := validateCategoryParentChain(
				parentByCategory, normalized.categoryNo, normalized.parentCategoryNo,
			); parentErr != nil {
				return parentErr
			}
			if parentErr := validateCategoryTwoLevels(parentByCategory, normalized.categoryNo, normalized.parentCategoryNo); parentErr != nil {
				return parentErr
			}
		}
		var parentCategoryNo *string
		if normalized.parentCategoryNo != "" {
			parentCategoryNo = &normalized.parentCategoryNo
		}
		updateResult, updateErr := dao.CatalogCategory.Ctx(ctx).
			Where(do.CatalogCategory{TenantId: idccap.DefaultTenantID, CategoryNo: normalized.categoryNo, Version: normalized.version}).
			Data(do.CatalogCategory{
				ParentCategoryNo: parentCategoryNo, NameZhCn: normalized.nameZhCN,
				NameEnUs: normalized.nameEnUS, SortOrder: normalized.sortOrder,
				Version: normalized.version + 1, UpdatedBy: actorUserID,
			}).Update()
		if writeErr := requireSingleCatalogWrite(updateResult, updateErr, "update catalog category"); writeErr != nil {
			return writeErr
		}
		result = categoryProjection(row)
		result.ParentCategoryNo = normalized.parentCategoryNo
		result.NameZhCN = normalized.nameZhCN
		result.NameEnUS = normalized.nameEnUS
		result.SortOrder = normalized.sortOrder
		result.Version = normalized.version + 1
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "catalog.category.update", IdempotencyKey: normalized.categoryNo + ":" + strconv.FormatInt(normalized.version, 10),
			OperationCode: "catalog.category.update", TargetType: "catalog_category", TargetNo: normalized.categoryNo,
			ResultCode: "succeeded", ReasonCode: "manual_update", RiskCode: "medium",
			RequiredPermission: permissionCatalogCategoryUpdate,
			BeforeSnapshot:     categoryAuditSnapshot(categoryProjection(row)),
			AfterSnapshot:      categoryAuditSnapshot(result),
		})
	})
	return managementResult(result, err)
}

// lockCatalogCategoryTree serializes parent changes before any category row is
// locked. This prevents two concurrent updates from each creating one half of
// a cycle while still keeping the lock scoped to the current transaction.
func lockCatalogCategoryTree(ctx context.Context) error {
	if gdb.TXFromCtx(ctx, dao.CatalogCategory.Group()) == nil {
		return gerror.New("catalog category tree lock requires a transaction")
	}
	_, err := dao.CatalogCategory.DB().Exec(
		ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended(?, 0))",
		"idc-core:catalog-category-tree",
	)
	if err != nil {
		return gerror.Wrap(err, "lock catalog category tree")
	}
	return nil
}

type catalogCategoryParentRow struct {
	CategoryNo       string  `orm:"category_no"`
	ParentCategoryNo *string `orm:"parent_category_no"`
}

func loadCatalogCategoryTreeForUpdate(ctx context.Context) (map[string]string, error) {
	columns := dao.CatalogCategory.Columns()
	rows := make([]catalogCategoryParentRow, 0, maximumCatalogCategoryCount)
	if err := dao.CatalogCategory.Ctx(ctx).
		Fields(columns.CategoryNo, columns.ParentCategoryNo).
		Where(do.CatalogCategory{
			TenantId: idccap.DefaultTenantID, StatusCode: catalogStatusActive,
		}).OrderAsc(columns.CategoryNo).
		Limit(maximumCatalogCategoryCount + 1).LockUpdate().Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "load catalog category tree for update")
	}
	if len(rows) > maximumCatalogCategoryCount {
		return nil, bizerr.NewCode(CodeManagementUnavailable)
	}
	parentByCategory := make(map[string]string, len(rows))
	for _, row := range rows {
		parent := ""
		if row.ParentCategoryNo != nil {
			parent = *row.ParentCategoryNo
		}
		parentByCategory[row.CategoryNo] = parent
	}
	return parentByCategory, nil
}

// validateCategoryParentChain walks the single bounded snapshot loaded above.
// Reaching the category being updated, an existing cycle, a missing parent, or
// the bound fails closed before any parent pointer is changed.
func validateCategoryParentChain(parentByCategory map[string]string, categoryNo, parentCategoryNo string) error {
	visited := make(map[string]struct{}, maximumCatalogCategoryCount)
	cursor := parentCategoryNo
	for range maximumCatalogCategoryCount {
		if cursor == categoryNo {
			return bizerr.NewCode(CodeCategoryParentCycle)
		}
		if _, exists := visited[cursor]; exists {
			return bizerr.NewCode(CodeCategoryParentCycle)
		}
		visited[cursor] = struct{}{}
		parent, exists := parentByCategory[cursor]
		if !exists {
			return bizerr.NewCode(CodeManagementNotAccessible)
		}
		if parent == "" {
			return nil
		}
		cursor = parent
	}
	return bizerr.NewCode(CodeCategoryParentCycle)
}

// validateCategoryTwoLevels rejects third-level parents and moving a parent
// with children beneath another category. The caller holds the tree lock.
func validateCategoryTwoLevels(parents map[string]string, categoryNo, parentNo string) error {
	if parentNo == "" {
		return nil
	}
	parent, exists := parents[parentNo]
	if !exists {
		return bizerr.NewCode(CodeManagementNotAccessible)
	}
	if parent != "" {
		return bizerr.NewCode(CodeCategoryDepthExceeded)
	}
	for _, parent := range parents {
		if parent == categoryNo {
			return bizerr.NewCode(CodeCategoryDepthExceeded)
		}
	}
	return nil
}

func (s *managementServiceImpl) DeleteCategory(ctx context.Context, input CategoryStateInput) error {
	categoryNo := strings.TrimSpace(input.CategoryNo)
	actorUserID, err := s.authorizer.AuthorizeCategoryDelete(ctx, categoryNo)
	if err != nil {
		return err
	}
	if actorUserID <= 0 || !categoryNumberPattern.MatchString(categoryNo) || input.Version < 1 {
		return bizerr.NewCode(CodeManagementInvalid)
	}
	err = dao.CatalogCategory.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, loadErr := loadCategoryForMaintenance(ctx, categoryNo, false)
		if loadErr != nil {
			return loadErr
		}
		if row.Version != input.Version {
			return bizerr.NewCode(CodeManagementConflict)
		}
		deleteResult, deleteErr := dao.CatalogCategory.Ctx(ctx).
			Where(do.CatalogCategory{TenantId: idccap.DefaultTenantID, CategoryNo: categoryNo, Version: input.Version}).Delete()
		if writeErr := requireSingleCatalogWrite(deleteResult, deleteErr, "delete catalog category"); writeErr != nil {
			return writeErr
		}
		after := categoryProjection(row)
		after.Deleted = true
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "catalog.category.delete", IdempotencyKey: categoryNo + ":" + strconv.FormatInt(input.Version, 10),
			OperationCode: "catalog.category.delete", TargetType: "catalog_category", TargetNo: categoryNo,
			ResultCode: "succeeded", ReasonCode: "manual_delete", RiskCode: "medium",
			RequiredPermission: permissionCatalogCategoryDelete,
			BeforeSnapshot:     categoryAuditSnapshot(categoryProjection(row)),
			AfterSnapshot:      categoryAuditSnapshot(after),
		})
	})
	if err != nil {
		_, businessError := bizerr.As(err)
		if businessError {
			return err
		}
		return bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	return nil
}

func (s *managementServiceImpl) RestoreCategory(
	ctx context.Context,
	input CategoryStateInput,
) (*Category, error) {
	categoryNo := strings.TrimSpace(input.CategoryNo)
	actorUserID, err := s.authorizer.AuthorizeCategoryRestore(ctx, categoryNo)
	if err != nil {
		return nil, err
	}
	if actorUserID <= 0 || !categoryNumberPattern.MatchString(categoryNo) || input.Version < 1 {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	var result *Category
	err = dao.CatalogCategory.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := lockCatalogCategoryTree(ctx); err != nil {
			return err
		}
		row, loadErr := loadCategoryForMaintenance(ctx, categoryNo, true)
		if loadErr != nil {
			return loadErr
		}
		if row.Version != input.Version {
			return bizerr.NewCode(CodeManagementConflict)
		}
		if row.ParentCategoryNo != nil && *row.ParentCategoryNo != "" {
			parents, treeErr := loadCatalogCategoryTreeForUpdate(ctx)
			if treeErr != nil {
				return treeErr
			}
			if treeErr = validateCategoryTwoLevels(parents, categoryNo, *row.ParentCategoryNo); treeErr != nil {
				return treeErr
			}
			if _, parentErr := loadActiveCategoryForUpdate(ctx, *row.ParentCategoryNo); parentErr != nil {
				return parentErr
			}
		}
		columns := dao.CatalogCategory.Columns()
		updateResult, updateErr := dao.CatalogCategory.Ctx(ctx).Unscoped().
			Where(do.CatalogCategory{TenantId: idccap.DefaultTenantID, CategoryNo: categoryNo, Version: input.Version}).
			Data(columns.DeletedAt, nil, columns.Version, input.Version+1, columns.UpdatedBy, actorUserID).Update()
		if writeErr := requireSingleCatalogWrite(updateResult, updateErr, "restore catalog category"); writeErr != nil {
			return writeErr
		}
		before := categoryProjection(row)
		before.Deleted = true
		result = categoryProjection(row)
		result.Version = input.Version + 1
		result.Deleted = false
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "catalog.category.restore", IdempotencyKey: categoryNo + ":" + strconv.FormatInt(input.Version, 10),
			OperationCode: "catalog.category.restore", TargetType: "catalog_category", TargetNo: categoryNo,
			ResultCode: "succeeded", ReasonCode: "manual_restore", RiskCode: "medium",
			RequiredPermission: permissionCatalogCategoryRestore,
			BeforeSnapshot:     categoryAuditSnapshot(before),
			AfterSnapshot:      categoryAuditSnapshot(result),
		})
	})
	return managementResult(result, err)
}

func normalizeCategoryUpdate(input CategoryUpdateInput) (normalizedCategoryUpdate, error) {
	categoryNo := strings.TrimSpace(input.CategoryNo)
	parent := strings.TrimSpace(input.ParentCategoryNo)
	nameZhCN := strings.TrimSpace(input.NameZhCN)
	nameEnUS := strings.TrimSpace(input.NameEnUS)
	if !categoryNumberPattern.MatchString(categoryNo) ||
		(parent != "" && !categoryNumberPattern.MatchString(parent)) || input.Version < 1 ||
		!validLocalizedText(nameZhCN, 128, false) || !validLocalizedText(nameEnUS, 128, false) ||
		!validSortOrder(input.SortOrder) {
		return normalizedCategoryUpdate{}, bizerr.NewCode(CodeManagementInvalid)
	}
	return normalizedCategoryUpdate{categoryNo: categoryNo, parentCategoryNo: parent, nameZhCN: nameZhCN, nameEnUS: nameEnUS, sortOrder: input.SortOrder, version: input.Version}, nil
}

func loadCategoryForMaintenance(ctx context.Context, categoryNo string, deleted bool) (*entity.CatalogCategory, error) {
	columns := dao.CatalogCategory.Columns()
	query := dao.CatalogCategory.Ctx(ctx)
	if deleted {
		query = query.Unscoped().WhereNotNull(columns.DeletedAt)
	}
	var row *entity.CatalogCategory
	if err := query.Where(do.CatalogCategory{TenantId: idccap.DefaultTenantID, CategoryNo: categoryNo}).LockUpdate().Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load catalog category for maintenance")
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return row, nil
}

func categoryProjection(row *entity.CatalogCategory) *Category {
	parent := ""
	if row.ParentCategoryNo != nil {
		parent = *row.ParentCategoryNo
	}
	return &Category{CategoryNo: row.CategoryNo, ParentCategoryNo: parent, NameZhCN: row.NameZhCn, NameEnUS: row.NameEnUs, SortOrder: row.SortOrder, Status: row.StatusCode, Version: row.Version, Deleted: row.DeletedAt != nil}
}

func categoryAuditSnapshot(category *Category) map[string]any {
	return map[string]any{"category_no": category.CategoryNo, "parent_category_no": category.ParentCategoryNo, "name_zh_cn": category.NameZhCN, "name_en_us": category.NameEnUS, "sort_order": category.SortOrder, "state": category.Status, "version": category.Version, "deleted": category.Deleted}
}

func requireSingleCatalogWrite(result interface{ RowsAffected() (int64, error) }, err error, operation string) error {
	if err != nil {
		return gerror.Wrap(err, operation)
	}
	affected, affectedErr := result.RowsAffected()
	if affectedErr != nil {
		return gerror.Wrap(affectedErr, "read "+operation+" result")
	}
	if affected != 1 {
		return gerror.Newf("%s affected %d rows", operation, affected)
	}
	return nil
}
