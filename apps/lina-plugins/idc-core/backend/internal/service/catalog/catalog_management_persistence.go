// This file persists categories and initial product drafts transactionally.

package catalog

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
)

const (
	catalogStatusActive = "active"
	catalogStatusDraft  = "draft"
	catalogSourceManual = "manual"
	catalogSourceCopy   = "copy"
	maximumSortOrder    = 1_000_000
)

var (
	categoryNumberPattern           = regexp.MustCompile(`^ctg_[0-9a-f]{32}$`)
	productNumberPattern            = regexp.MustCompile(`^prd_[0-9a-f]{32}$`)
	versionNumberPattern            = regexp.MustCompile(`^ver_[0-9a-f]{32}$`)
	existingCatalogReferencePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,39}$`)
	publicSlugPattern               = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

type normalizedCategoryCreate struct {
	parentCategoryNo string
	nameZhCN         string
	nameEnUS         string
	sortOrder        int
}

type normalizedProductCreate struct {
	directoryNo              string
	categoryNo               string
	publicSlug               string
	nameZhCN                 string
	nameEnUS                 string
	summaryZhCN              string
	summaryEnUS              string
	sortOrder                int
	contractPolicyCode       contractpolicy.Policy
	contractTemplateNo       string
	inventoryTrackingEnabled bool
	sellableQuantity         int64
}

func (s *managementServiceImpl) CreateCategory(
	ctx context.Context,
	input CategoryCreateInput,
) (*Category, error) {
	actorUserID, err := s.authorizer.AuthorizeCategoryCreate(ctx)
	if err != nil {
		return nil, err
	}
	if actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	normalized, err := normalizeCategoryCreate(input)
	if err != nil {
		return nil, err
	}
	categoryNo, err := newCatalogNumber(s.random, "ctg")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	var result *Category
	err = dao.CatalogCategory.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err = s.createCategoryInTransaction(ctx, categoryNo, actorUserID, normalized)
		return err
	})
	return managementResult(result, err)
}

func (s *managementServiceImpl) createCategoryInTransaction(
	ctx context.Context,
	categoryNo string,
	actorUserID int,
	input normalizedCategoryCreate,
) (*Category, error) {
	if input.parentCategoryNo != "" {
		if err := lockCatalogCategoryTree(ctx); err != nil {
			return nil, err
		}
		parents, err := loadCatalogCategoryTreeForUpdate(ctx)
		if err != nil {
			return nil, err
		}
		if err = validateCategoryTwoLevels(parents, categoryNo, input.parentCategoryNo); err != nil {
			return nil, err
		}
		_, err = loadActiveCategoryForUpdate(ctx, input.parentCategoryNo)
		if err != nil {
			return nil, err
		}
	}
	var parentCategoryNo *string
	if input.parentCategoryNo != "" {
		parentCategoryNo = &input.parentCategoryNo
	}
	insertResult, insertErr := dao.CatalogCategory.Ctx(ctx).Data(do.CatalogCategory{
		TenantId: idccap.DefaultTenantID, CategoryNo: categoryNo,
		ParentCategoryNo: parentCategoryNo,
		NameZhCn:         input.nameZhCN, NameEnUs: input.nameEnUS, SortOrder: input.sortOrder,
		StatusCode: catalogStatusActive, CreatedBy: actorUserID, UpdatedBy: actorUserID,
	}).InsertIgnore()
	if err := requireSingleCatalogInsert(insertResult, insertErr, "insert catalog category"); err != nil {
		return nil, err
	}
	result := &Category{
		CategoryNo: categoryNo, ParentCategoryNo: input.parentCategoryNo,
		NameZhCN: input.nameZhCN, NameEnUS: input.nameEnUS,
		SortOrder: input.sortOrder, Status: catalogStatusActive, Version: 1,
	}
	if err := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "catalog.category.create", IdempotencyKey: categoryNo,
		OperationCode: "catalog.category.create", TargetType: "catalog_category", TargetNo: categoryNo,
		ResultCode: "succeeded", ReasonCode: "manual_create", RiskCode: "medium",
		RequiredPermission: permissionCatalogCategoryCreate,
		BeforeSnapshot:     map[string]any{},
		AfterSnapshot: map[string]any{
			"state":              catalogStatusActive,
			"parent_category_no": input.parentCategoryNo,
		},
	}); err != nil {
		return nil, gerror.Wrap(err, "append catalog category creation audit")
	}
	return result, nil
}

func (s *managementServiceImpl) CreateProductDraft(
	ctx context.Context,
	input ProductCreateInput,
) (*ProductDraft, error) {
	actorUserID, err := s.authorizer.AuthorizeProductCreate(ctx)
	if err != nil {
		return nil, err
	}
	if actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	normalized, err := normalizeProductCreate(input)
	if err != nil {
		return nil, err
	}
	if err = s.validateProductContractRule(ctx, normalized); err != nil {
		return nil, err
	}
	productNo, err := newCatalogNumber(s.random, "prd")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	versionNo, err := newCatalogNumber(s.random, "ver")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	var result *ProductDraft
	err = dao.CatalogProduct.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err = s.createProductDraftInTransaction(
			ctx, productNo, versionNo, actorUserID, normalized,
		)
		return err
	})
	return managementResult(result, err)
}

func (s *managementServiceImpl) createProductDraftInTransaction(
	ctx context.Context,
	productNo string,
	versionNo string,
	actorUserID int,
	input normalizedProductCreate,
) (*ProductDraft, error) {
	_, err := loadActiveCategoryForUpdate(ctx, input.categoryNo)
	if err != nil {
		return nil, err
	}
	if gdb.TXFromCtx(ctx, dao.CatalogProduct.Group()) == nil {
		return nil, gerror.New("catalog product slug check requires a transaction")
	}
	if _, err = dao.CatalogProduct.DB().Exec(
		ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended(?, 0))",
		"idc-core:catalog-product-slug:"+input.publicSlug,
	); err != nil {
		return nil, gerror.Wrap(err, "lock catalog product slug")
	}
	slugCount, err := dao.CatalogProduct.Ctx(ctx).
		Where(do.CatalogProduct{
			TenantId: idccap.DefaultTenantID, PublicSlug: input.publicSlug,
		}).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "check catalog product slug")
	}
	if slugCount != 0 {
		return nil, bizerr.NewCode(CodeProductSlugOccupied)
	}
	insertResult, insertErr := dao.CatalogProduct.Ctx(ctx).Data(do.CatalogProduct{
		TenantId: idccap.DefaultTenantID, ProductNo: productNo, CategoryNo: input.categoryNo,
		StatusCode: catalogStatusDraft, PublicSlug: input.publicSlug,
		NameZhCn: input.nameZhCN, NameEnUs: input.nameEnUS,
		SummaryZhCn: input.summaryZhCN, SummaryEnUs: input.summaryEnUS,
		SortOrder: input.sortOrder, CreatedBy: actorUserID, UpdatedBy: actorUserID,
	}).InsertIgnore()
	if err = requireSingleCatalogInsert(insertResult, insertErr, "insert catalog product"); err != nil {
		return nil, err
	}
	if input.directoryNo != "" {
		if err = replaceProductDirectory(ctx, productNo, input.directoryNo); err != nil {
			return nil, err
		}
	}
	insertResult, insertErr = dao.CatalogProductVersion.Ctx(ctx).Data(do.CatalogProductVersion{
		TenantId: idccap.DefaultTenantID, VersionNo: versionNo, ProductNo: productNo,
		VersionNumber: 1, StatusCode: catalogStatusDraft, SourceCode: catalogSourceManual,
		RequiresContract:      input.contractPolicyCode == contractpolicy.PolicyRequired,
		ContractPolicyCode:    string(input.contractPolicyCode),
		ContractTemplateNo:    nullableCatalogContractTemplate(input.contractTemplateNo),
		ContractPolicyVersion: 1, CreatedBy: actorUserID,
	}).InsertIgnore()
	if err = requireSingleCatalogInsert(insertResult, insertErr, "insert initial catalog product version"); err != nil {
		return nil, err
	}
	insertResult, insertErr = dao.CatalogProductVersionInventory.Ctx(ctx).Data(do.CatalogProductVersionInventory{
		VersionNo: versionNo, TrackingEnabled: input.inventoryTrackingEnabled,
		SellableQuantity: input.sellableQuantity, Version: 1,
	}).InsertIgnore()
	if err = requireSingleCatalogInsert(insertResult, insertErr, "insert initial product inventory"); err != nil {
		return nil, err
	}
	result := &ProductDraft{
		ProductNo: productNo, VersionNo: versionNo, VersionNumber: 1,
		CategoryNo: input.categoryNo, Status: catalogStatusDraft,
		VersionStatus: catalogStatusDraft, Source: catalogSourceManual,
	}
	if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "catalog.product-draft.create", IdempotencyKey: productNo,
		OperationCode: "catalog.product-draft.create", TargetType: "catalog_product", TargetNo: productNo,
		ResultCode: "succeeded", ReasonCode: "manual_create", RiskCode: "medium",
		RequiredPermission: permissionCatalogProductCreate,
		BeforeSnapshot:     map[string]any{},
		AfterSnapshot: map[string]any{
			"state": catalogStatusDraft, "version_no": versionNo,
			"category_no":                input.categoryNo,
			"contract_policy_code":       input.contractPolicyCode,
			"contract_template_no":       input.contractTemplateNo,
			"inventory_tracking_enabled": input.inventoryTrackingEnabled,
			"sellable_quantity":          input.sellableQuantity,
		},
	}); err != nil {
		return nil, gerror.Wrap(err, "append catalog product draft creation audit")
	}
	return result, nil
}

func loadActiveCategoryForUpdate(ctx context.Context, categoryNo string) (*entity.CatalogCategory, error) {
	var category *entity.CatalogCategory
	if err := dao.CatalogCategory.Ctx(ctx).
		Where(do.CatalogCategory{
			TenantId: idccap.DefaultTenantID, CategoryNo: categoryNo, StatusCode: catalogStatusActive,
		}).
		LockUpdate().Scan(&category); err != nil {
		return nil, gerror.Wrap(err, "load active catalog category")
	}
	if category == nil {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return category, nil
}

func normalizeCategoryCreate(input CategoryCreateInput) (normalizedCategoryCreate, error) {
	parent := strings.TrimSpace(input.ParentCategoryNo)
	nameZhCN := strings.TrimSpace(input.NameZhCN)
	nameEnUS := strings.TrimSpace(input.NameEnUS)
	if (parent != "" && !categoryNumberPattern.MatchString(parent)) ||
		!validLocalizedText(nameZhCN, 128, false) ||
		!validLocalizedText(nameEnUS, 128, false) || !validSortOrder(input.SortOrder) {
		return normalizedCategoryCreate{}, bizerr.NewCode(CodeManagementInvalid)
	}
	return normalizedCategoryCreate{
		parentCategoryNo: parent,
		nameZhCN:         nameZhCN, nameEnUS: nameEnUS, sortOrder: input.SortOrder,
	}, nil
}

func normalizeProductCreate(input ProductCreateInput) (normalizedProductCreate, error) {
	categoryNo := strings.TrimSpace(input.CategoryNo)
	publicSlug := strings.ToLower(strings.TrimSpace(input.PublicSlug))
	nameZhCN := strings.TrimSpace(input.NameZhCN)
	nameEnUS := strings.TrimSpace(input.NameEnUS)
	summaryZhCN := strings.TrimSpace(input.SummaryZhCN)
	summaryEnUS := strings.TrimSpace(input.SummaryEnUS)
	if input.ContractPolicyCode == "" {
		input.ContractPolicyCode = contractpolicy.PolicyInherit
	}
	rule, ruleErr := contractpolicy.NormalizeRule(contractpolicy.Rule{
		Policy: input.ContractPolicyCode, TemplateNo: input.ContractTemplateNo,
	}, true)
	if input.DirectoryNo != "" && !existingCatalogReferencePattern.MatchString(strings.TrimSpace(input.DirectoryNo)) || !categoryNumberPattern.MatchString(categoryNo) ||
		!validLocalizedText(publicSlug, 128, false) || !publicSlugPattern.MatchString(publicSlug) ||
		!validLocalizedText(nameZhCN, 256, false) || !validLocalizedText(nameEnUS, 256, false) ||
		!validLocalizedText(summaryZhCN, 1024, true) || !validLocalizedText(summaryEnUS, 1024, true) ||
		!validSortOrder(input.SortOrder) || input.SellableQuantity < 0 || ruleErr != nil {
		return normalizedProductCreate{}, bizerr.NewCode(CodeManagementInvalid)
	}
	return normalizedProductCreate{
		directoryNo: strings.TrimSpace(input.DirectoryNo), categoryNo: categoryNo, publicSlug: publicSlug,
		nameZhCN: nameZhCN, nameEnUS: nameEnUS, summaryZhCN: summaryZhCN,
		summaryEnUS: summaryEnUS, sortOrder: input.SortOrder,
		contractPolicyCode: rule.Policy, contractTemplateNo: rule.TemplateNo,
		inventoryTrackingEnabled: input.InventoryTrackingEnabled,
		sellableQuantity:         input.SellableQuantity,
	}, nil
}

func (s *managementServiceImpl) validateProductContractRule(
	ctx context.Context,
	input normalizedProductCreate,
) error {
	if input.contractPolicyCode != contractpolicy.PolicyRequired {
		return nil
	}
	if s.templates == nil {
		return bizerr.NewCode(CodeManagementContractTemplateUnavailable)
	}
	selection := contracttemplatecap.Selection{Level: contracttemplatecap.SelectionLevelProduct}
	resolved, err := s.templates.ResolveAvailable(ctx, selection, []string{input.contractTemplateNo})
	if err != nil {
		if errors.Is(err, contracttemplatecap.ErrSourceUnavailable) ||
			errors.Is(err, contracttemplatecap.ErrTemplateUnavailable) {
			return bizerr.WrapCode(err, CodeManagementContractTemplateUnavailable)
		}
		return bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if len(resolved) != 1 || resolved[0].TemplateNo != input.contractTemplateNo ||
		contracttemplatecap.ValidateTemplate(resolved[0], selection) != nil {
		return bizerr.NewCode(CodeManagementContractTemplateUnavailable)
	}
	return nil
}

func nullableCatalogContractTemplate(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func validLocalizedText(value string, maximum int, allowEmpty bool) bool {
	length := utf8.RuneCountInString(value)
	return (allowEmpty || length > 0) && length <= maximum && !strings.ContainsRune(value, '\x00')
}

func validSortOrder(value int) bool {
	return value >= -maximumSortOrder && value <= maximumSortOrder
}

func newCatalogNumber(random io.Reader, prefix string) (string, error) {
	if random == nil {
		return "", gerror.New("invalid catalog number dependency")
	}
	var pattern *regexp.Regexp
	switch prefix {
	case "ctg":
		pattern = categoryNumberPattern
	case "prd":
		pattern = productNumberPattern
	case "ver":
		pattern = versionNumberPattern
	default:
		return "", gerror.New("invalid catalog number prefix")
	}
	randomBytes := make([]byte, 16)
	if _, err := io.ReadFull(random, randomBytes); err != nil {
		return "", gerror.Wrap(err, "read catalog number entropy")
	}
	result := prefix + "_" + hex.EncodeToString(randomBytes)
	if !pattern.MatchString(result) {
		return "", gerror.New("generated catalog number is invalid")
	}
	return result, nil
}

func managementResult[T any](result *T, err error) (*T, error) {
	if err == nil {
		return result, nil
	}
	if _, businessError := bizerr.As(err); businessError {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
}

func requireSingleCatalogInsert(result sql.Result, err error, operation string) error {
	if err != nil {
		return gerror.Wrap(err, operation)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read "+operation+" result")
	}
	if affected == 0 {
		return bizerr.NewCode(CodeManagementConflict)
	}
	if affected != 1 {
		return gerror.Newf("%s affected %d rows", operation, affected)
	}
	return nil
}
