// Package principalprovision owns durable upstream-customer account convergence.
package principalprovision

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

// Service creates an idempotent local convergence task before server provisioning.
type Service interface {
	// EnsureForPool returns true only after an exact upstream lookup has
	// confirmed and persisted the channel-local customer mapping.
	EnsureForPool(ctx context.Context, customerNo string, poolNo string) (bool, error)
}

type serviceImpl struct {
	database gdb.DB
	binder   ManagerBinder
}

var _ Service = (*serviceImpl)(nil)

// ManagerBinder supplies the exact channel-scoped secret lifecycle manager.
type ManagerBinder func(channelNo string) (secretcap.Manager, error)

// New constructs the owner-side principal task service.
func New(database gdb.DB, binder ManagerBinder) (Service, error) {
	if database == nil || binder == nil {
		return nil, gerror.New("resource principal provisioning dependencies are required")
	}
	return &serviceImpl{database: database, binder: binder}, nil
}
