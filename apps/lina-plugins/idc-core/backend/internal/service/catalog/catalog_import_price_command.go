// Source-import receipts remain immutable and unique per product version.
// Price-only saves use an independent, transactional idempotency receipt.
package catalog

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

func loadImportedPriceCommand(ctx context.Context, actor int, input ImportInput, digest string) (*ImportResult, error) {
	imported, err := dao.CatalogImportContract.Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: input.TargetVersionNo}).Exist()
	if err != nil || !imported {
		return nil, err
	}
	var receipt *struct{ VersionNo, RequestDigest string }
	if err = dao.CatalogImportPriceCommand.Ctx(ctx).Where(do.CatalogImportPriceCommand{TenantId: 0, ActorUserId: actor, RequestKey: input.RequestKey}).Fields("version_no,request_digest").Scan(&receipt); err != nil {
		return nil, err
	}
	if receipt == nil {
		return nil, nil
	}
	if receipt.VersionNo != input.TargetVersionNo || receipt.RequestDigest != digest {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	var row *struct {
		ProductNo, VersionNo, CategoryNo, StatusCode, VersionStatus, SourceCode string
		VersionNumber                                                           int
	}
	if err = dao.CatalogProductVersion.Ctx(ctx).As("version").InnerJoin("plugin_idc_core_catalog_product product", "product.tenant_id=version.tenant_id AND product.product_no=version.product_no").Where("version.tenant_id", 0).Where("version.version_no", input.TargetVersionNo).Fields("version.product_no,version.version_no,version.version_number,version.status_code AS version_status,version.source_code,product.category_no,product.status_code").Scan(&row); err != nil {
		return nil, err
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return &ImportResult{PricingOnly: true, Item: ProductDraft{ProductNo: row.ProductNo, VersionNo: row.VersionNo, CategoryNo: row.CategoryNo, VersionNumber: row.VersionNumber, Status: row.StatusCode, VersionStatus: row.VersionStatus, Source: row.SourceCode}, MissingConfiguration: []ImportMissingConfiguration{}}, nil
}
func insertImportedPriceCommand(ctx context.Context, actor int, input ImportInput, digest string) error {
	result, err := dao.CatalogImportPriceCommand.Ctx(ctx).Data(do.CatalogImportPriceCommand{TenantId: 0, ActorUserId: actor, RequestKey: input.RequestKey, RequestDigest: digest, VersionNo: input.TargetVersionNo}).InsertIgnore()
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return bizerr.NewCode(CodeManagementConflict)
	}
	return nil
}
