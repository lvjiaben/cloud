// This file centralizes guarded-row update result validation across transitions.

package enterpriseverification

import (
	"database/sql"

	"github.com/gogf/gf/v2/errors/gerror"
)

func requireOneRow(result sql.Result, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, operation)
	}
	if affected != 1 {
		return gerror.New(operation + " lost its state fence")
	}
	return nil
}
