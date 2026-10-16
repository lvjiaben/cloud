// Package triallifecycle converges local trial entitlement expiry through the
// resource owner's durable, idempotent lifecycle tasks. It never bills or deletes.
package triallifecycle

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
)

// Service is the trusted primary-node trial expiry worker, not a staff action.
type Service interface {
	// Run schedules and processes at most limit entitlements (1..32) per pass.
	// Only confirmed resource success changes status. Unknown results freeze;
	// transient failures and pending observations use bounded durable retry.
	Run(ctx context.Context, limit int) error
}

// serviceImpl shares startup-owned task, resource and notification boundaries.
type serviceImpl struct {
	database      gdb.DB
	tasks         asynctask.Service
	resource      lifecycleresourcecap.Source
	notifications notificationcap.EventSink
}

var _ Service = (*serviceImpl)(nil)

// New requires explicit startup dependencies; no worker constructs a service graph.
func New(database gdb.DB, tasks asynctask.Service, resource lifecycleresourcecap.Source, notifications notificationcap.EventSink) (Service, error) {
	if database == nil || tasks == nil || resource == nil || notifications == nil {
		return nil, gerror.New("trial lifecycle dependencies are unavailable")
	}
	return &serviceImpl{database: database, tasks: tasks, resource: resource, notifications: notifications}, nil
}

// RecoveryScheduler durably queues recovery in the caller's payment transaction.
// It does not call the supplier or mark the resource active before confirmation.
type RecoveryScheduler interface {
	// ScheduleRecovery requires an owned suspended formal entitlement. A replay
	// for the same revision returns the same durable task; ownership changes fail.
	ScheduleRecovery(ctx context.Context, entitlementNo, customerNo string) error
}

// recoveryScheduler shares the existing database and startup task service.
type recoveryScheduler struct {
	database gdb.DB
	tasks    asynctask.Service
}

// NewRecoveryScheduler constructs the payment-side producer without a worker graph.
func NewRecoveryScheduler(database gdb.DB, tasks asynctask.Service) (RecoveryScheduler, error) {
	if database == nil || tasks == nil {
		return nil, gerror.New("trial recovery scheduling dependencies are unavailable")
	}
	return &recoveryScheduler{database: database, tasks: tasks}, nil
}
