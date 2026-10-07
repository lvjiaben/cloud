// Package taxinvoicesource reads bounded core-owned billing truth for the
// compliance tax-invoice workflow.
package taxinvoicesource

import (
	"reflect"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

// serviceImpl implements the source-only tax-invoice boundary.
type serviceImpl struct {
	database  gdb.DB
	customers identitycap.CurrentCustomerReader
}

// Ensure serviceImpl continues to satisfy the public source contract.
var _ taxinvoicecap.Service = (*serviceImpl)(nil)

// New constructs the bounded source service from the shared database and the
// current-customer identity projection.
func New(
	database gdb.DB,
	customers identitycap.CurrentCustomerReader,
) (taxinvoicecap.Service, error) {
	if nilDependency(database) || nilDependency(customers) {
		return nil, gerror.New("tax invoice source dependencies are invalid")
	}
	return &serviceImpl{database: database, customers: customers}, nil
}

// nilDependency detects absent interface values, including typed nil values.
func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
