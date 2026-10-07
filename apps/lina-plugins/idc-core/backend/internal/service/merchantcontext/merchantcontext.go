// Package merchantcontext publishes the installation's bounded merchant profile projection.
package merchantcontext

import (
	"reflect"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
)

const merchantProfileTable = "plugin_idc_core_merchant_profile"

type serviceImpl struct{ database gdb.DB }

var _ idccap.Service = (*serviceImpl)(nil)

// New constructs the core-owned single-merchant context source.
func New(database gdb.DB) (idccap.Service, error) {
	if nilDatabase(database) {
		return nil, gerror.New("IDC merchant context database is unavailable")
	}
	return &serviceImpl{database: database}, nil
}

func nilDatabase(value gdb.DB) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	return reflected.Kind() == reflect.Pointer && reflected.IsNil()
}
