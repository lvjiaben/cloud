// This file atomically replaces a product's member-navigation assignment under
// the catalog product lock. Directory membership never changes service ownership.

package catalog

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/productdirectory"
	"strings"
)

// replaceProductDirectory requires the caller's authorized catalog transaction.
// An empty directory removes only the navigation assignment, never paid services.
func replaceProductDirectory(ctx context.Context, productNo, directoryNo string) error {
	directoryNo = strings.TrimSpace(directoryNo)
	current, err := dao.ProductDirectoryMembership.Ctx(ctx).Where(do.ProductDirectoryMembership{TenantId: idccap.DefaultTenantID, ProductNo: productNo}).Value("directory_no")
	if err != nil {
		return err
	}
	if current.String() == directoryNo {
		return nil
	}
	if directoryNo != "" {
		if !existingCatalogReferencePattern.MatchString(directoryNo) {
			return bizerr.NewCode(CodeManagementInvalid)
		}
		var target *struct{ DirectoryNo string }
		err := dao.ProductDirectory.Ctx(ctx).Where(do.ProductDirectory{TenantId: idccap.DefaultTenantID, DirectoryNo: directoryNo, StatusCode: productdirectory.StatusEnabled, NavigationType: string(productdirectory.NavigationProduct)}).Fields("directory_no").LockUpdate().Scan(&target)
		if err != nil {
			return err
		}
		if target == nil {
			return bizerr.NewCode(CodeManagementNotAccessible)
		}
	}
	if _, err := dao.ProductDirectoryMembership.Ctx(ctx).Where(do.ProductDirectoryMembership{TenantId: idccap.DefaultTenantID, ProductNo: productNo}).Delete(); err != nil {
		return err
	}
	if directoryNo == "" {
		return nil
	}
	_, err = dao.ProductDirectoryMembership.Ctx(ctx).Data(do.ProductDirectoryMembership{TenantId: idccap.DefaultTenantID, ProductNo: productNo, DirectoryNo: directoryNo}).Insert()
	return err
}
