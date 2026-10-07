// This file keeps credit counts and bounded rows in one read-only snapshot.
// Callers supply the identical authorized filter before adding cursor/page bounds.

package billing

import (
	"context"
	"math"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

// creditPageOffset validates numeric pages without mixing cursor semantics.
func creditPageOffset(page, limit int, cursor string) (int, error) {
	if page < 0 || limit < 1 || limit > maximumCreditDirectoryLimit ||
		(page > 0 && cursor != "") || (page > 0 && page-1 > math.MaxInt/limit) {
		return 0, bizerr.NewCode(CodeDirectoryInvalid)
	}
	if page == 0 {
		return 0, nil
	}
	return (page - 1) * limit, nil
}

// readCreditDirectoryPage counts the pre-pagination filtered projection and
// scans only the requested bounded rows; concurrent writes cannot split totals.
func readCreditDirectoryPage(ctx context.Context, database gdb.DB, rows any,
	filteredQuery string, filteredArguments []any, pageQuery string, pageArguments []any,
) (int, error) {
	var total int
	err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := database.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); err != nil {
			return err
		}
		count, err := database.GetValue(ctx,
			"SELECT COUNT(*) FROM ("+filteredQuery+") credit_directory_count", filteredArguments...)
		if err != nil {
			return err
		}
		total = count.Int()
		return database.GetScan(ctx, rows, pageQuery, pageArguments...)
	})
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	return total, nil
}
