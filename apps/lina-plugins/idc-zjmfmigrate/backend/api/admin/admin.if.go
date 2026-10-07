// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package admin

import (
	"context"

	"lina-plugin-idc-zjmfmigrate/backend/api/admin/v1"
)

type IAdminV1 interface {
	Preflight(ctx context.Context, req *v1.PreflightReq) (res *v1.PreflightRes, err error)
	CreateBatch(ctx context.Context, req *v1.CreateBatchReq) (res *v1.CreateBatchRes, err error)
	Run(ctx context.Context, req *v1.RunReq) (res *v1.RunRes, err error)
	Conflicts(ctx context.Context, req *v1.ConflictsReq) (res *v1.ConflictsRes, err error)
	DecideConflict(ctx context.Context, req *v1.DecideConflictReq) (res *v1.DecideConflictRes, err error)
	Reconcile(ctx context.Context, req *v1.ReconcileReq) (res *v1.ReconcileRes, err error)
	Cutover(ctx context.Context, req *v1.CutoverReq) (res *v1.CutoverRes, err error)
	Rollback(ctx context.Context, req *v1.RollbackReq) (res *v1.RollbackRes, err error)
	Report(ctx context.Context, req *v1.ReportReq) (res *v1.ReportRes, err error)
}
