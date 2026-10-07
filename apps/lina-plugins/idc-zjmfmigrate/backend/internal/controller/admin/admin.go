// Package admin adapts the migration service to the HTTP API.
package admin

import (
	"context"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/pluginbridge"
	v1 "lina-plugin-idc-zjmfmigrate/backend/api/admin/v1"
	"lina-plugin-idc-zjmfmigrate/backend/internal/service/migrate"
)

type Controller struct {
	svc migrate.Service
	biz bizctxcap.Service
}

// New creates the dynamic-plugin controller from the guest's governed host
// services. The zero-argument form is also used by the generated WASM route
// dispatcher.
func New() *Controller {
	services := pluginbridge.Default()
	return NewWithServices(migrate.New(services.RecordStore()), services.BizCtx())
}

// NewWithServices keeps service injection available for focused tests.
func NewWithServices(svc migrate.Service, biz bizctxcap.Service) *Controller {
	return &Controller{svc: svc, biz: biz}
}
func (c *Controller) Preflight(ctx context.Context, req *v1.PreflightReq) (*v1.PreflightRes, error) {
	r := c.svc.Preflight(ctx, migrate.PreflightInput{req.SourceSystem, req.SourceVersion, req.Charset, req.Timezone, req.Currencies})
	return &v1.PreflightRes{r.SourceSystem, r.SourceVersion, r.Supported, r.Blockers, r.Warnings, r.Capabilities}, nil
}
func (c *Controller) CreateBatch(ctx context.Context, req *v1.CreateBatchReq) (*v1.CreateBatchRes, error) {
	r, e := c.svc.CreateBatch(ctx, migrate.CreateBatchInput{req.BatchNo, req.SourceSystem, req.SourceVersion, req.Mode})
	if e != nil {
		return nil, e
	}
	return &v1.CreateBatchRes{r.Id, r.BatchNo, r.Status}, nil
}
func (c *Controller) Run(ctx context.Context, req *v1.RunReq) (*v1.RunRes, error) {
	r, e := c.svc.Run(ctx, req.Id, req.Watermark)
	if e != nil {
		return nil, e
	}
	return &v1.RunRes{r.Id, r.Status, r.Mapped, r.Blockers}, nil
}
func (c *Controller) Conflicts(ctx context.Context, req *v1.ConflictsReq) (*v1.ConflictsRes, error) {
	list, total, e := c.svc.Conflicts(ctx, req.Id, req.Page, req.PageSize)
	if e != nil {
		return nil, e
	}
	out := make([]*v1.Conflict, 0, len(list))
	for _, x := range list {
		y := x
		out = append(out, &v1.Conflict{y.Id, y.ObjectType, y.SourceId, y.Summary, y.Decision})
	}
	return &v1.ConflictsRes{out, total}, nil
}
func (c *Controller) DecideConflict(ctx context.Context, req *v1.DecideConflictReq) (*v1.DecideConflictRes, error) {
	r, e := c.svc.DecideConflict(ctx, req.Id, req.Decision, req.Reason, c.actor(ctx))
	if e != nil {
		return nil, e
	}
	return &v1.DecideConflictRes{r.Id, r.Decision}, nil
}
func (c *Controller) Reconcile(ctx context.Context, req *v1.ReconcileReq) (*v1.ReconcileRes, error) {
	r, e := c.svc.Reconcile(ctx, req.Id)
	if e != nil {
		return nil, e
	}
	return &v1.ReconcileRes{r.Id, r.Status, r.Differences}, nil
}
func (c *Controller) Cutover(ctx context.Context, req *v1.CutoverReq) (*v1.CutoverRes, error) {
	r, e := c.svc.Cutover(ctx, req.Id)
	if e != nil {
		return nil, e
	}
	return &v1.CutoverRes{r.Id, r.Allowed, r.Checks}, nil
}
func (c *Controller) Rollback(ctx context.Context, req *v1.RollbackReq) (*v1.RollbackRes, error) {
	r, e := c.svc.Rollback(ctx, req.Id)
	if e != nil {
		return nil, e
	}
	return &v1.RollbackRes{req.Id, r}, nil
}
func (c *Controller) Report(ctx context.Context, req *v1.ReportReq) (*v1.ReportRes, error) {
	r, e := c.svc.Report(ctx, req.Id)
	if e != nil {
		return nil, e
	}
	return &v1.ReportRes{r.Id, r.Status, r.ByType, r.Blockers, r.Warnings}, nil
}
func (c *Controller) actor(ctx context.Context) int64 {
	if c.biz == nil {
		return 0
	}
	return int64(c.biz.Current(ctx).UserID)
}
