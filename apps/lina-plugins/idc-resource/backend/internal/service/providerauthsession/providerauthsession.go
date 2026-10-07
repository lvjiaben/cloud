// Package providerauthsession owns cluster-safe Provider login sessions.
package providerauthsession

import (
	"crypto/rand"
	"io"
	"reflect"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// Service coordinates one exact channel through PostgreSQL and secretcap.
// Access and Invalidate use shared database state; constructing multiple
// implementations never creates independent session authority.
type Service interface {
	resourcecap.AuthenticationSessions
}

type serviceImpl struct {
	database   gdb.DB
	channelRef string
	manager    secretcap.Manager
	accessor   secretcap.Accessor
	random     io.Reader
}

var _ Service = (*serviceImpl)(nil)

// New constructs a database-authoritative session service for one channel.
func New(
	database gdb.DB,
	channelRef string,
	manager secretcap.Manager,
	accessor secretcap.Accessor,
) (Service, error) {
	if database == nil || resourcecap.ValidateChannelRef(channelRef) != nil ||
		isNilInterface(manager) || isNilInterface(accessor) {
		return nil, gerror.New("resource Provider authentication session dependencies are invalid")
	}
	return &serviceImpl{
		database: database, channelRef: channelRef, manager: manager,
		accessor: accessor, random: rand.Reader,
	}, nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	typed := reflect.ValueOf(value)
	switch typed.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return typed.IsNil()
	default:
		return false
	}
}
