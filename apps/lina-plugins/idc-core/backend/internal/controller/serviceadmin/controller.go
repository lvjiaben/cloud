package serviceadmin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	accountv1 "lina-plugin-idc-core/backend/api/serviceaccount/v1"
	accountcontroller "lina-plugin-idc-core/backend/internal/controller/serviceaccount"
	"lina-plugin-idc-core/backend/internal/service/serviceaction"
)

type ControllerV1 struct {
	service serviceaction.Service
	access  serviceaction.Access
}

func NewV1(service serviceaction.Service, access serviceaction.Access) (*ControllerV1, error) {
	if service == nil || access == nil {
		return nil, gerror.New("managed service admin controller is unavailable")
	}
	return &ControllerV1{service: service, access: access}, nil
}
func (c *ControllerV1) ManagedServiceEntitlementList(ctx context.Context, req *v1.ManagedServiceEntitlementListReq) (*v1.ManagedServiceEntitlementListRes, error) {
	actor, err := c.access.Admin(ctx, serviceaction.PermissionList)
	if err != nil {
		return nil, err
	}
	before, err := accountcontroller.DecodeManagedServiceCursor(req.Cursor)
	if err != nil {
		return nil, err
	}
	items, next, err := c.service.ListEntitlements(ctx, actor, before, req.Limit)
	if err != nil {
		return nil, err
	}
	result := make([]v1.ServiceEntitlement, len(items))
	for index := range items {
		source := accountcontroller.AccountEntitlement(items[index])
		result[index] = adminEntitlement(source)
		for ai, a := range source.Actions {
			result[index].Actions[ai] = v1.ServiceActionDeclaration{Key: a.Key, Title: v1.LocalizedTitle{EnUS: a.Title.EnUS, ZhCN: a.Title.ZhCN}, Execution: a.Execution, Availability: a.Availability}
		}
		for di, d := range source.Downloads {
			result[index].Downloads[di] = v1.ServiceDownloadDeclaration{Key: d.Key, Title: v1.LocalizedTitle{EnUS: d.Title.EnUS, ZhCN: d.Title.ZhCN}}
		}
	}
	return &v1.ManagedServiceEntitlementListRes{Items: result, NextCursor: accountcontroller.EncodeManagedServiceCursor(next)}, nil
}
func (c *ControllerV1) ManagedServiceEntitlementGet(ctx context.Context, req *v1.ManagedServiceEntitlementGetReq) (*v1.ManagedServiceEntitlementGetRes, error) {
	actor, err := c.access.Admin(ctx, serviceaction.PermissionList)
	if err != nil {
		return nil, err
	}
	item, err := c.service.GetEntitlement(ctx, req.EntitlementNo, actor)
	if err != nil {
		return nil, err
	}
	source := accountcontroller.AccountEntitlement(*item)
	projected := adminEntitlement(source)
	result := &projected
	for i, a := range source.Actions {
		result.Actions[i] = v1.ServiceActionDeclaration{Key: a.Key, Title: v1.LocalizedTitle{EnUS: a.Title.EnUS, ZhCN: a.Title.ZhCN}, Execution: a.Execution, Availability: a.Availability}
	}
	for i, d := range source.Downloads {
		result.Downloads[i] = v1.ServiceDownloadDeclaration{Key: d.Key, Title: v1.LocalizedTitle{EnUS: d.Title.EnUS, ZhCN: d.Title.ZhCN}}
	}
	return &v1.ManagedServiceEntitlementGetRes{Item: result}, nil
}

func adminEntitlement(source accountv1.ServiceEntitlement) v1.ServiceEntitlement {
	return v1.ServiceEntitlement{EntitlementNo: source.EntitlementNo, ProductNameZhCN: source.ProductNameZhCN, ProductNameEnUS: source.ProductNameEnUS, Status: source.Status, StatusCode: source.StatusCode, CustomerName: source.CustomerName, PrimaryDomain: source.PrimaryDomain, ValidationMethod: source.ValidationMethod, Validation: source.Validation, AvailableActions: append([]string(nil), source.AvailableActions...), StartedAt: source.StartedAt, IssuedAt: source.IssuedAt, UpdatedAt: source.UpdatedAt, ExpiresAt: source.ExpiresAt, Actions: make([]v1.ServiceActionDeclaration, len(source.Actions)), Downloads: make([]v1.ServiceDownloadDeclaration, len(source.Downloads))}
}
func (c *ControllerV1) ActionCreate(ctx context.Context, req *v1.ActionCreateReq) (*v1.ActionCreateRes, error) {
	actor, err := c.access.Admin(ctx, serviceaction.PermissionAction)
	if err != nil {
		return nil, err
	}
	task, err := c.service.Create(ctx, serviceaction.CreateInput{EntitlementNo: req.EntitlementNo, ActionKey: req.ActionKey, IdempotencyKey: req.IdempotencyKey, Input: req.Input, Actor: actor})
	if err != nil {
		return nil, err
	}
	return &v1.ActionCreateRes{Task: adminTask(task)}, nil
}
func (c *ControllerV1) ActionTaskGet(ctx context.Context, req *v1.ActionTaskGetReq) (*v1.ActionTaskGetRes, error) {
	actor, err := c.access.Admin(ctx, serviceaction.PermissionList)
	if err != nil {
		return nil, err
	}
	task, err := c.service.Get(ctx, req.TaskNo, actor)
	if err != nil {
		return nil, err
	}
	return &v1.ActionTaskGetRes{Task: adminTask(task)}, nil
}
func (c *ControllerV1) ActionTakeover(ctx context.Context, req *v1.ActionTakeoverReq) (*v1.ActionTakeoverRes, error) {
	actor, err := c.access.Admin(ctx, serviceaction.PermissionTakeover)
	if err != nil {
		return nil, err
	}
	task, err := c.service.Takeover(ctx, req.TaskNo, req.ExpectedVersion, actor)
	if err != nil {
		return nil, err
	}
	return &v1.ActionTakeoverRes{Task: adminTask(task)}, nil
}
func (c *ControllerV1) DownloadTicketCreate(ctx context.Context, req *v1.DownloadTicketCreateReq) (*v1.DownloadTicketCreateRes, error) {
	actor, err := c.access.Admin(ctx, serviceaction.PermissionDownload)
	if err != nil {
		return nil, err
	}
	ticket, err := c.service.CreateDownloadTicket(ctx, serviceaction.DownloadInput{EntitlementNo: req.EntitlementNo, DownloadKey: req.DownloadKey, Actor: actor})
	if err != nil {
		return nil, err
	}
	return &v1.DownloadTicketCreateRes{Ticket: ticket.Ticket, FileName: ticket.FileName, ExpiresAt: ticket.ExpiresAt.UnixMilli()}, nil
}
func (c *ControllerV1) DownloadOpen(ctx context.Context, req *v1.DownloadOpenReq) (*v1.DownloadOpenRes, error) {
	actor, err := c.access.Admin(ctx, serviceaction.PermissionDownload)
	if err != nil {
		return nil, err
	}
	file, err := c.service.RedeemDownload(ctx, req.Ticket, actor)
	if err != nil {
		return nil, err
	}
	if err = accountcontroller.WriteManagedServiceDownload(ctx, file); err != nil {
		return nil, err
	}
	return &v1.DownloadOpenRes{}, nil
}
func adminTask(task *serviceaction.Task) *v1.ActionTask {
	if task == nil {
		return nil
	}
	return &v1.ActionTask{TaskNo: task.TaskNo, EntitlementNo: task.EntitlementNo, ActionKey: task.ActionKey, Status: task.Status, SafeProjection: task.SafeProjection, Version: task.Version}
}
