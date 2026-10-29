// This file lets authorized staff maintain bilingual sales copy and sales
// category without changing order snapshots, URL identity, prices or contracts.

package catalog

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// ProductDescriptionUpdateInput carries editable sales copy, optional sales
// category and member navigation, and the last-read nonzero product revision.
type ProductDescriptionUpdateInput struct {
	ProductNo                                    string
	SortOrder                                    *int
	DirectoryNo                                  *string
	CategoryNo                                   *string
	NameZhCN, NameEnUS, SummaryZhCN, SummaryEnUS string
	Version                                      int64
}

func (s *managementServiceImpl) UpdateProductDescription(ctx context.Context, input ProductDescriptionUpdateInput) (int64, error) {
	input.ProductNo = strings.TrimSpace(input.ProductNo)
	actor, err := s.authorizer.AuthorizeProductUpdate(ctx, input.ProductNo)
	if err != nil {
		return 0, err
	}
	if actor <= 0 {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	input.NameZhCN, input.NameEnUS = strings.TrimSpace(input.NameZhCN), strings.TrimSpace(input.NameEnUS)
	input.SummaryZhCN, input.SummaryEnUS = strings.TrimSpace(input.SummaryZhCN), strings.TrimSpace(input.SummaryEnUS)
	if input.SortOrder != nil && !validSortOrder(*input.SortOrder) {
		return 0, bizerr.NewCode(CodeManagementInvalid)
	}
	if input.Version < 1 || !existingCatalogReferencePattern.MatchString(input.ProductNo) ||
		!validLocalizedText(input.NameZhCN, 256, false) || !validLocalizedText(input.NameEnUS, 256, false) ||
		!validLocalizedText(input.SummaryZhCN, 1024, false) || !validLocalizedText(input.SummaryEnUS, 1024, false) {
		return 0, bizerr.NewCode(CodeManagementInvalid)
	}
	if input.DirectoryNo != nil && strings.TrimSpace(*input.DirectoryNo) == "" {
		return 0, bizerr.NewCode(CodeManagementInvalid)
	}
	if input.CategoryNo != nil {
		categoryNo := strings.TrimSpace(*input.CategoryNo)
		if !categoryNumberPattern.MatchString(categoryNo) {
			return 0, bizerr.NewCode(CodeManagementInvalid)
		}
		input.CategoryNo = &categoryNo
	}

	err = dao.CatalogProduct.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var previous *entity.CatalogProduct
		if err := dao.CatalogProduct.Ctx(txCtx).Where(do.CatalogProduct{TenantId: idccap.DefaultTenantID, ProductNo: input.ProductNo}).LockUpdate().Scan(&previous); err != nil {
			return err
		}
		if previous == nil {
			return bizerr.NewCode(CodeManagementNotAccessible)
		}
		if previous.Version != input.Version {
			return bizerr.NewCode(CodeManagementConflict)
		}
		nextSortOrder := previous.SortOrder
		if input.SortOrder != nil {
			nextSortOrder = *input.SortOrder
		}
		nextCategory := previous.CategoryNo
		if input.CategoryNo != nil && *input.CategoryNo != previous.CategoryNo {
			if _, err := loadActiveCategoryForUpdate(txCtx, *input.CategoryNo); err != nil {
				return err
			}
			nextCategory = *input.CategoryNo
		}
		previousDirectory, err := dao.ProductDirectoryMembership.Ctx(txCtx).Where(do.ProductDirectoryMembership{TenantId: idccap.DefaultTenantID, ProductNo: input.ProductNo}).Value("directory_no")
		if err != nil {
			return err
		}
		nextDirectory := previousDirectory.String()
		if input.DirectoryNo != nil {
			nextDirectory = strings.TrimSpace(*input.DirectoryNo)
			if err := replaceProductDirectory(txCtx, input.ProductNo, *input.DirectoryNo); err != nil {
				return err
			}
		}
		result, err := dao.CatalogProduct.Ctx(txCtx).Where(do.CatalogProduct{TenantId: idccap.DefaultTenantID, ProductNo: input.ProductNo, Version: input.Version}).Data(do.CatalogProduct{SortOrder: nextSortOrder, CategoryNo: nextCategory, NameZhCn: input.NameZhCN, NameEnUs: input.NameEnUS, SummaryZhCn: input.SummaryZhCN, SummaryEnUs: input.SummaryEnUS, Version: input.Version + 1, UpdatedBy: actor}).Update()
		if err = requireSingleCatalogWrite(result, err, "update product description"); err != nil {
			return err
		}
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{IdempotencyScope: "catalog.product-description.update", IdempotencyKey: input.ProductNo + ":" + strconv.FormatInt(input.Version, 10), OperationCode: "catalog.product-description.update", TargetType: "catalog_product", TargetNo: input.ProductNo, ResultCode: "succeeded", ReasonCode: "manual_update", RiskCode: "medium", RequiredPermission: permissionCatalogProductUpdate,
			BeforeSnapshot: map[string]any{"sort_order": previous.SortOrder, "category_no": previous.CategoryNo, "directory_no": previousDirectory.String(), "name_zh_cn": previous.NameZhCn, "name_en_us": previous.NameEnUs, "summary_zh_cn": previous.SummaryZhCn, "summary_en_us": previous.SummaryEnUs, "version": previous.Version},
			AfterSnapshot:  map[string]any{"sort_order": nextSortOrder, "category_no": nextCategory, "directory_no": nextDirectory, "name_zh_cn": input.NameZhCN, "name_en_us": input.NameEnUS, "summary_zh_cn": input.SummaryZhCN, "summary_en_us": input.SummaryEnUS, "version": input.Version + 1}})
	})
	if err != nil {
		_, wrapped := managementResult[int64](nil, err)
		return 0, wrapped
	}
	return input.Version + 1, nil
}
