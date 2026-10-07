// Privacy transaction helpers keep service transactions on the host-injected database.
// They deliberately hide the raw transaction handle so callers can only use the
// transaction-bound context and cannot bypass the plugin's database boundary.

package dao

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
)

// TransactionWithDatabase runs a privacy transaction on the plugin-injected database.
func (d privacyActionDao) TransactionWithDatabase(
	ctx context.Context,
	database gdb.DB,
	callback func(context.Context) error,
) error {
	return database.Transaction(ctx, func(transactionContext context.Context, _ gdb.TX) error {
		return callback(transactionContext)
	})
}
