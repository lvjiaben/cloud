package customer

import (
	"context"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
	api "lina-plugin-idc-zjmfcloud/backend/api/customer/v1"
)

type serverServiceStub struct {
	listRequest resourcecap.CurrentCustomerServerListRequest
	getRequest  resourcecap.CurrentCustomerServerGetRequest
	item        resourcecap.CurrentCustomerServerItem
}

func (stub *serverServiceStub) ListCurrentCustomerServers(_ context.Context, request resourcecap.CurrentCustomerServerListRequest) (*resourcecap.CurrentCustomerServerPage, error) {
	stub.listRequest = request
	return &resourcecap.CurrentCustomerServerPage{Items: []resourcecap.CurrentCustomerServerItem{stub.item}, NextCursor: "cursor_next"}, nil
}

func (stub *serverServiceStub) GetCurrentCustomerServer(_ context.Context, request resourcecap.CurrentCustomerServerGetRequest) (*resourcecap.CurrentCustomerServerResult, error) {
	stub.getRequest = request
	return &resourcecap.CurrentCustomerServerResult{Item: stub.item}, nil
}

func TestCustomerServerControllerUsesOnlyCurrentCustomerContract(t *testing.T) {
	serverNo := "SVR-0123456789ABCDEF0123456789ABCDEF"
	stub := &serverServiceStub{item: resourcecap.CurrentCustomerServerItem{
		ServerNo: serverNo, ProductNameZhCN: "商务云", ProductNameEnUS: "Business Cloud",
		ChannelName: "华东渠道", PoolName: "上海资源池", Kind: "cloud_server", State: "ready",
		LatestOperation: "idle", AvailableActions: []resourcecap.ServerActionCode{resourcecap.ServerActionReboot}, BoundAt: 1777000000000, UpdatedAt: 1777000100000,
	}}
	controller := NewWithService(stub)
	page, err := controller.ServerList(context.Background(), &api.ServerListReq{Cursor: "cursor_prev", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if stub.listRequest.Cursor != "cursor_prev" || stub.listRequest.Limit != 20 || len(page.Items) != 1 || page.Items[0].ServerNo != serverNo || page.Items[0].AvailableActions[0] != "reboot" {
		t.Fatalf("unexpected current-customer list projection: request=%#v response=%#v", stub.listRequest, page)
	}
	detail, err := controller.ServerDetail(context.Background(), &api.ServerDetailReq{ServerNo: serverNo})
	if err != nil {
		t.Fatal(err)
	}
	if stub.getRequest.ServerNo != serverNo || detail.Item == nil || detail.Item.ProductNameZhCN != "商务云" {
		t.Fatalf("unexpected current-customer detail projection: request=%#v response=%#v", stub.getRequest, detail)
	}
}
