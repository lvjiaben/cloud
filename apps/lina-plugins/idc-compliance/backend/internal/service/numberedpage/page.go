// Package numberedpage implements bounded reads within the caller's authorized
// query. It neither adds authorization nor invents counts from returned rows.
package numberedpage

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
)

func Valid(page, size int, cursor string) bool {
	return page >= 0 && page <= 1000000 && size >= 1 && size <= 100 && !(page > 0 && cursor != "")
}

func Scan(ctx context.Context, db gdb.DB, countModel, pageModel *gdb.Model, rows any) (int, error) {
	var total int
	err := db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, err := db.Exec(txCtx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); err != nil {
			return err
		}
		var err error
		total, err = countModel.Clone().Ctx(txCtx).Count()
		if err != nil {
			return err
		}
		return pageModel.Clone().Ctx(txCtx).Scan(rows)
	})
	return total, err
}

func Query(ctx context.Context, db gdb.DB, rows any, countSQL string, countArgs []any, pageSQL string, pageArgs []any) (int64, error) {
	var total int64
	err := db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, err := db.Exec(txCtx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); err != nil {
			return err
		}
		value, err := db.GetValue(txCtx, countSQL, countArgs...)
		if err != nil {
			return err
		}
		total = value.Int64()
		return db.GetScan(txCtx, rows, pageSQL, pageArgs...)
	})
	return total, err
}
