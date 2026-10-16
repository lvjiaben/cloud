// This file rechecks sale status before renewal confirmation and payment.
// A shared product lock prevents a concurrent stop from passing the check
// while the enclosing transaction creates an invoice or spends balance.

package catalog

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
)

// RequireRenewalSale rejects stopped products and missing sale authority.
func (s *publicCatalogReaderImpl) RequireRenewalSale(ctx context.Context, productNo string) error {
	var row *struct {
		StatusCode string `orm:"status_code"`
	}
	err := s.database.GetScan(ctx, &row, `
SELECT status_code FROM plugin_idc_core_catalog_product
WHERE tenant_id=0 AND product_no=? AND deleted_at IS NULL
FOR SHARE`, productNo)
	if err != nil {
		return gerror.Wrap(err, "lock renewal product sale")
	}
	if row == nil || row.StatusCode != catalogStatusActive {
		return bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	return nil
}
