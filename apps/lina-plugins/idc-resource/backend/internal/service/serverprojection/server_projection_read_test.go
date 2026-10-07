// This file verifies trusted-current-customer projection and safe mapping.

package serverprojection

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serverdirectory"
)

type customerStub struct{ customerNo string }

func (stub customerStub) CurrentCustomer(context.Context) (*customerauthcap.Subject, error) {
	return &customerauthcap.Subject{CustomerNo: stub.customerNo}, nil
}

type directoryStub struct {
	listRequest   serverdirectory.ListRequest
	detailRequest serverdirectory.DetailRequest
	item          serverdirectory.Item
}

func (stub *directoryStub) List(_ context.Context, request serverdirectory.ListRequest) (*serverdirectory.Page, error) {
	stub.listRequest = request
	return &serverdirectory.Page{Items: []serverdirectory.Item{stub.item}, NextCursor: "v1.public"}, nil
}

func (stub *directoryStub) Detail(_ context.Context, request serverdirectory.DetailRequest) (*serverdirectory.Item, error) {
	stub.detailRequest = request
	item := stub.item
	return &item, nil
}

func TestCurrentCustomerProjectionUsesTrustedOwnerAndHidesActionReference(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0).UTC()
	directory := &directoryStub{item: serverdirectory.Item{
		ServerNo:        "SVR-0123456789ABCDEF0123456789ABCDEF",
		ActionReference: "srv_11111111111111111111111111111111",
		ProductNameZhCN: "云服务器", ProductNameEnUS: "Cloud server",
		ChannelName: "生产资源", PoolName: "华东资源池",
		Kind: serverdirectory.ServerKindCloud, State: serverdirectory.ResourceStateReady,
		LatestOperation:  serverdirectory.OperationStateIdle,
		AvailableActions: []serverdirectory.Action{serverdirectory.ActionReboot},
		BoundAt:          now, UpdatedAt: now,
	}}
	service, err := New(customerStub{customerNo: "customer-authenticated"}, directory)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.ListCurrentCustomerServers(context.Background(), resourcecap.CurrentCustomerServerListRequest{Limit: 20})
	if err != nil {
		t.Fatalf("list current customer servers: %v", err)
	}
	if directory.listRequest.Scope.CustomerNo != "customer-authenticated" || len(page.Items) != 1 ||
		page.Items[0].ServerNo != directory.item.ServerNo || page.Items[0].BoundAt != now.UnixMilli() {
		t.Fatalf("unexpected list projection: request=%+v page=%+v", directory.listRequest, page)
	}
	result, err := service.GetCurrentCustomerServer(context.Background(), resourcecap.CurrentCustomerServerGetRequest{ServerNo: directory.item.ServerNo})
	if err != nil {
		t.Fatalf("get current customer server: %v", err)
	}
	if directory.detailRequest.Scope.CustomerNo != "customer-authenticated" ||
		directory.detailRequest.ServerNo != directory.item.ServerNo || result.Item.ServerNo != directory.item.ServerNo {
		t.Fatalf("unexpected detail projection: request=%+v result=%+v", directory.detailRequest, result)
	}
}
