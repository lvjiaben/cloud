package marketing

import (
	"context"
	"math"

	"github.com/gogf/gf/v2/database/gdb"
)

func validMarketingNumberedPage(page, limit int, cursor string) bool {
	return page >= 0 && limit > 0 && limit <= maximumMarketingManagementPageSize &&
		(page == 0 || (cursor == "" && page-1 <= math.MaxInt/limit))
}

// The caller authorizes the directory first. Count and rows share its exact
// filtered relation, while expensive per-item statistics stay page-bounded.
func readMarketingNumberedPage(ctx context.Context, database gdb.DB, rows any,
	countSQL string, countArgs []any, pageSQL string, pageArgs []any,
) (total int, err error) {
	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, readErr := database.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); readErr != nil {
			return readErr
		}
		count, readErr := database.GetValue(ctx, countSQL, countArgs...)
		if readErr != nil {
			return readErr
		}
		total = count.Int()
		return database.GetScan(ctx, rows, pageSQL, pageArgs...)
	})
	return total, err
}
