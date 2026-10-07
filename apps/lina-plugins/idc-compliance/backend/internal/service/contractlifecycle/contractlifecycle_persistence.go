// This file keeps the lifecycle service on its explicitly injected database
// while reusing generated DAO table metadata and DO write objects.

package contractlifecycle

import (
	"context"
	"database/sql"

	"github.com/gogf/gf/v2/database/gdb"
)

type scopedModel interface {
	Ctx(context.Context) *gdb.Model
	Exec(context.Context, string, ...any) (sql.Result, error)
	Table() string
	Transaction(context.Context, func(context.Context, gdb.TX) error) error
}

type scopedModelImpl struct {
	database gdb.DB
	table    string
}

func newScopedModel(database gdb.DB, table string) scopedModel {
	return &scopedModelImpl{database: database, table: table}
}

func (model *scopedModelImpl) Ctx(ctx context.Context) *gdb.Model {
	return model.database.Model(model.table).Safe().Ctx(ctx)
}

func (model *scopedModelImpl) Exec(
	ctx context.Context,
	query string,
	arguments ...any,
) (sql.Result, error) {
	return model.database.Exec(ctx, query, arguments...)
}

func (model *scopedModelImpl) Table() string {
	return model.table
}

func (model *scopedModelImpl) Transaction(
	ctx context.Context,
	callback func(context.Context, gdb.TX) error,
) error {
	return model.database.Transaction(ctx, callback)
}
