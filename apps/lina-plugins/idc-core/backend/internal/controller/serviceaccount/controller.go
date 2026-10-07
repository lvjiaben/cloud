package serviceaccount

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	v1 "lina-plugin-idc-core/backend/api/serviceaccount/v1"
	"lina-plugin-idc-core/backend/internal/service/serviceaction"
)

func (c *ControllerV1) EntitlementList(ctx context.Context, req *v1.EntitlementListReq) (*v1.EntitlementListRes, error) {
	actor, err := c.access.Customer(ctx)
	if err != nil {
		return nil, err
	}
	before, err := DecodeManagedServiceCursor(req.Cursor)
	if err != nil {
		return nil, err
	}
	items, next, err := c.service.ListEntitlements(ctx, actor, before, req.Limit)
	if err != nil {
		return nil, err
	}
	result := make([]v1.ServiceEntitlement, len(items))
	for index := range items {
		result[index] = AccountEntitlement(items[index])
	}
	return &v1.EntitlementListRes{Items: result, NextCursor: EncodeManagedServiceCursor(next)}, nil
}
func (c *ControllerV1) EntitlementGet(ctx context.Context, req *v1.EntitlementGetReq) (*v1.EntitlementGetRes, error) {
	actor, err := c.access.Customer(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.service.GetEntitlement(ctx, req.EntitlementNo, actor)
	if err != nil {
		return nil, err
	}
	projected := AccountEntitlement(*item)
	return &v1.EntitlementGetRes{Item: &projected}, nil
}

type ControllerV1 struct {
	service serviceaction.Service
	access  serviceaction.Access
}

func NewV1(service serviceaction.Service, access serviceaction.Access) (*ControllerV1, error) {
	if service == nil || access == nil {
		return nil, gerror.New("managed customer service controller is unavailable")
	}
	return &ControllerV1{service: service, access: access}, nil
}

func (c *ControllerV1) ActionCreate(ctx context.Context, req *v1.ActionCreateReq) (*v1.ActionCreateRes, error) {
	actor, err := c.access.Customer(ctx)
	if err != nil {
		return nil, err
	}
	task, err := c.service.Create(ctx, serviceaction.CreateInput{EntitlementNo: req.EntitlementNo, ActionKey: req.ActionKey, IdempotencyKey: req.IdempotencyKey, Input: req.Input, Actor: actor})
	if err != nil {
		return nil, err
	}
	return &v1.ActionCreateRes{Task: accountTask(task)}, nil
}
func (c *ControllerV1) ActionTaskGet(ctx context.Context, req *v1.ActionTaskGetReq) (*v1.ActionTaskGetRes, error) {
	actor, err := c.access.Customer(ctx)
	if err != nil {
		return nil, err
	}
	task, err := c.service.Get(ctx, req.TaskNo, actor)
	if err != nil {
		return nil, err
	}
	return &v1.ActionTaskGetRes{Task: accountTask(task)}, nil
}
func (c *ControllerV1) DownloadTicketCreate(ctx context.Context, req *v1.DownloadTicketCreateReq) (*v1.DownloadTicketCreateRes, error) {
	actor, err := c.access.Customer(ctx)
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
	actor, err := c.access.Customer(ctx)
	if err != nil {
		return nil, err
	}
	file, err := c.service.RedeemDownload(ctx, req.Ticket, actor)
	if err != nil {
		return nil, err
	}
	if err = WriteManagedServiceDownload(ctx, file); err != nil {
		return nil, err
	}
	return &v1.DownloadOpenRes{}, nil
}

func accountTask(task *serviceaction.Task) *v1.ActionTask {
	if task == nil {
		return nil
	}
	return &v1.ActionTask{TaskNo: task.TaskNo, EntitlementNo: task.EntitlementNo, ActionKey: task.ActionKey, Status: task.Status, SafeProjection: task.SafeProjection, Version: task.Version}
}

func AccountEntitlement(item serviceaction.Entitlement) v1.ServiceEntitlement {
	projected := v1.ServiceEntitlement{EntitlementNo: item.EntitlementNo, ProductNameZhCN: item.ProductNameZhCN, ProductNameEnUS: item.ProductNameEnUS, Status: item.Status, StatusCode: item.Status, CustomerName: item.CustomerName, PrimaryDomain: item.PrimaryDomain, ValidationMethod: item.ValidationMethod, Validation: item.Validation, AvailableActions: append([]string(nil), item.AvailableActions...), StartedAt: item.StartedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(), Actions: make([]v1.ServiceActionDeclaration, len(item.Actions)), Downloads: make([]v1.ServiceDownloadDeclaration, len(item.Downloads))}
	if !item.IssuedAt.IsZero() {
		projected.IssuedAt = item.IssuedAt.UnixMilli()
	}
	if item.ExpiresAt != nil {
		projected.ExpiresAt = item.ExpiresAt.UnixMilli()
	}
	for index, action := range item.Actions {
		projected.Actions[index] = v1.ServiceActionDeclaration{Key: action.Key, Title: v1.LocalizedTitle{EnUS: action.Title.EnUS, ZhCN: action.Title.ZhCN}, Execution: action.Execution, Availability: action.Availability}
	}
	for index, download := range item.Downloads {
		projected.Downloads[index] = v1.ServiceDownloadDeclaration{Key: download.Key, Title: v1.LocalizedTitle{EnUS: download.Title.EnUS, ZhCN: download.Title.ZhCN}}
	}
	return projected
}
func DecodeManagedServiceCursor(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, gerror.New("service cursor is invalid")
	}
	var id int64
	if _, err = fmt.Sscanf(string(raw), "%d", &id); err != nil || id < 1 {
		return 0, gerror.New("service cursor is invalid")
	}
	return id, nil
}
func EncodeManagedServiceCursor(value int64) string {
	if value < 1 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d", value)))
}

func WriteManagedServiceDownload(ctx context.Context, file *serviceaction.DownloadContent) error {
	request := g.RequestFromCtx(ctx)
	if request == nil || file == nil || len(file.Content) == 0 {
		return gerror.New("service download is unavailable")
	}
	name := strings.NewReplacer("\\", "_", "\"", "_", "\r", "_", "\n", "_").Replace(file.FileName)
	request.Response.Header().Set("Content-Type", file.ContentType)
	request.Response.Header().Set("Content-Length", strconv.Itoa(len(file.Content)))
	request.Response.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
	request.Response.Header().Set("Cache-Control", "private, no-store")
	request.Response.Header().Set("X-Content-Type-Options", "nosniff")
	_, err := request.Response.RawWriter().Write(file.Content)
	clear(file.Content)
	request.ExitAll()
	return err
}
