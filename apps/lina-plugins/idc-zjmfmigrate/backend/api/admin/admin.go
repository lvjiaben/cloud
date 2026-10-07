// Package admin declares the migration administration API contract.
package admin

import (
	"context"
	v1 "lina-plugin-idc-zjmfmigrate/backend/api/admin/v1"
)

// IMigrationV1 is the public migration API exposed by the dynamic plugin.
type IMigrationV1 interface {
	Preflight(context.Context, *v1.PreflightReq) (*v1.PreflightRes, error)
	CreateBatch(context.Context, *v1.CreateBatchReq) (*v1.CreateBatchRes, error)
	Run(context.Context, *v1.RunReq) (*v1.RunRes, error)
	Conflicts(context.Context, *v1.ConflictsReq) (*v1.ConflictsRes, error)
	DecideConflict(context.Context, *v1.DecideConflictReq) (*v1.DecideConflictRes, error)
	Reconcile(context.Context, *v1.ReconcileReq) (*v1.ReconcileRes, error)
	Cutover(context.Context, *v1.CutoverReq) (*v1.CutoverRes, error)
	Rollback(context.Context, *v1.RollbackReq) (*v1.RollbackRes, error)
	Report(context.Context, *v1.ReportReq) (*v1.ReportRes, error)
}
