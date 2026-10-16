// 本文件验证后台真实路由控制器调用原有服务，保留分页、客户范围及错误。

package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "lina-plugin-idc-channel/backend/api/admin/v1"
	"lina-plugin-idc-channel/backend/internal/service/channel"
)

// adminServiceStub only replaces the three operations used by the access page.
type adminServiceStub struct {
	channel.Service
	list   func(context.Context, channel.PageQuery) (channel.PageResult[channel.ChannelSummary], error)
	status func(context.Context, string, string) (channel.ChannelSummary, error)
	audit  func(context.Context, string, channel.PageQuery) (channel.PageResult[channel.RequestAudit], error)
}

func (s adminServiceStub) ListChannels(ctx context.Context, query channel.PageQuery) (channel.PageResult[channel.ChannelSummary], error) {
	return s.list(ctx, query)
}
func (s adminServiceStub) SetChannelStatus(ctx context.Context, id, action string) (channel.ChannelSummary, error) {
	return s.status(ctx, id, action)
}
func (s adminServiceStub) ListRequestAudit(ctx context.Context, id string, query channel.PageQuery) (channel.PageResult[channel.RequestAudit], error) {
	return s.audit(ctx, id, query)
}

func TestAccessPageControllers(t *testing.T) {
	var (
		ctx    = context.Background()
		stamp  = time.UnixMilli(1788163200000)
		denied = errors.New("service denied")
	)
	t.Run("list preserves page and projection", func(t *testing.T) {
		called := false
		controller := NewV1(adminServiceStub{list: func(received context.Context, q channel.PageQuery) (channel.PageResult[channel.ChannelSummary], error) {
			called = true
			if received != ctx || q != (channel.PageQuery{Page: 2, PageSize: 10, Keyword: "partner"}) {
				t.Fatalf("unexpected query: %+v", q)
			}
			return channel.PageResult[channel.ChannelSummary]{List: []channel.ChannelSummary{{ChannelID: "app_test", CustomerNo: "customer_test", CreatedAt: stamp, UpdatedAt: stamp}}, Page: 2, PageSize: 10, Total: 23}, nil
		}})
		res, err := controller.ListChannels(ctx, &v1.ListChannelsReq{PageRequest: v1.PageRequest{Page: 2, PageSize: 10}, Keyword: "partner"})
		if err != nil || !called || res.Total != 23 || res.Page != 2 || res.PageSize != 10 || len(res.List) != 1 || res.List[0].CustomerNo != "customer_test" || res.List[0].CreatedAt != stamp.UnixMilli() {
			t.Fatalf("unexpected response: %+v, %v", res, err)
		}
	})
	t.Run("status preserves the selected authorization", func(t *testing.T) {
		controller := NewV1(adminServiceStub{status: func(received context.Context, id, action string) (channel.ChannelSummary, error) {
			if received != ctx || id != "app_test" || action != "pause" {
				t.Fatalf("unexpected target: %s %s", id, action)
			}
			return channel.ChannelSummary{ChannelID: id, StatusCode: "paused", NewSalesEnabled: false}, nil
		}})
		res, err := controller.SetChannelStatus(ctx, &v1.SetChannelStatusReq{ChannelId: "app_test", Action: "pause"})
		if err != nil || res.Channel.ChannelId != "app_test" || res.Channel.StatusCode != "paused" || res.Channel.NewSalesEnabled {
			t.Fatalf("unexpected response: %+v, %v", res, err)
		}
	})
	t.Run("requests preserve account and timestamps", func(t *testing.T) {
		controller := NewV1(adminServiceStub{audit: func(received context.Context, id string, q channel.PageQuery) (channel.PageResult[channel.RequestAudit], error) {
			if received != ctx || id != "app_test" || q.Page != 3 || q.PageSize != 20 {
				t.Fatalf("unexpected query: %s %+v", id, q)
			}
			return channel.PageResult[channel.RequestAudit]{List: []channel.RequestAudit{{ID: 7, ChannelID: id, CreatedAt: stamp, CompletedAt: &stamp}}, Total: 45, Page: 3, PageSize: 20}, nil
		}})
		res, err := controller.ListRequestAudit(ctx, &v1.ListRequestAuditReq{ChannelId: "app_test", PageRequest: v1.PageRequest{Page: 3, PageSize: 20}})
		if err != nil || res.Total != 45 || res.Page != 3 || res.PageSize != 20 || len(res.List) != 1 || res.List[0].CompletedAt == nil || *res.List[0].CompletedAt != stamp.UnixMilli() {
			t.Fatalf("unexpected response: %+v, %v", res, err)
		}
	})
	t.Run("service failures are not replaced or swallowed", func(t *testing.T) {
		controller := NewV1(adminServiceStub{
			list: func(context.Context, channel.PageQuery) (channel.PageResult[channel.ChannelSummary], error) {
				return channel.PageResult[channel.ChannelSummary]{}, denied
			},
			status: func(context.Context, string, string) (channel.ChannelSummary, error) {
				return channel.ChannelSummary{}, denied
			},
			audit: func(context.Context, string, channel.PageQuery) (channel.PageResult[channel.RequestAudit], error) {
				return channel.PageResult[channel.RequestAudit]{}, denied
			},
		})
		if res, err := controller.ListChannels(ctx, &v1.ListChannelsReq{}); res != nil || !errors.Is(err, denied) {
			t.Fatalf("list: %+v %v", res, err)
		}
		if res, err := controller.SetChannelStatus(ctx, &v1.SetChannelStatusReq{}); res != nil || !errors.Is(err, denied) {
			t.Fatalf("status: %+v %v", res, err)
		}
		if res, err := controller.ListRequestAudit(ctx, &v1.ListRequestAuditReq{}); res != nil || !errors.Is(err, denied) {
			t.Fatalf("audit: %+v %v", res, err)
		}
	})
}
