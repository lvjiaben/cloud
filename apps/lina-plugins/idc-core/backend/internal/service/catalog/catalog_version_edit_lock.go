// This file owns the general product-version parent lock shared by pricing writes.

package catalog

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func lockEditableCatalogVersion(
	ctx context.Context,
	versionNo string,
) (*entity.CatalogProductVersion, error) {
	var version *entity.CatalogProductVersion
	if err := dao.CatalogProductVersion.Ctx(ctx).
		Where(do.CatalogProductVersion{
			TenantId: idccap.DefaultTenantID, VersionNo: versionNo,
		}).LockUpdate().Scan(&version); err != nil {
		return nil, gerror.Wrap(err, "lock editable catalog product version")
	}
	if version == nil {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	if version.StatusCode != catalogStatusDraft || version.PublishedAt != nil || version.RetiredAt != nil ||
		(version.SourceCode != catalogSourceManual && version.SourceCode != catalogSourceCopy) {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	return version, nil
}
