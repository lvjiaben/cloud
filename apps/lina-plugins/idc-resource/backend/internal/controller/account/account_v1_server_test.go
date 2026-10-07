package account

import (
	"context"
	"reflect"
	"testing"

	v1 "lina-plugin-idc-resource/backend/api/account/v1"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type serverServiceStub struct {
	listRequest   resourcecap.CurrentCustomerServerListRequest
	detailRequest resourcecap.CurrentCustomerServerGetRequest
	item          resourcecap.CurrentCustomerServerItem
}

func (stub *serverServiceStub) ListCurrentCustomerServers(_ context.Context, request resourcecap.CurrentCustomerServerListRequest) (*resourcecap.CurrentCustomerServerPage, error) {
	stub.listRequest = request
	return &resourcecap.CurrentCustomerServerPage{Items: []resourcecap.CurrentCustomerServerItem{stub.item}, NextCursor: "next"}, nil
}

func (stub *serverServiceStub) GetCurrentCustomerServer(_ context.Context, request resourcecap.CurrentCustomerServerGetRequest) (*resourcecap.CurrentCustomerServerResult, error) {
	stub.detailRequest = request
	return &resourcecap.CurrentCustomerServerResult{Item: stub.item}, nil
}

func TestServerRoutesUseOnlyCustomerSafeOwnerService(t *testing.T) {
	t.Parallel()
	servers := &serverServiceStub{item: resourcecap.CurrentCustomerServerItem{
		ServerNo:        "SVR-0123456789ABCDEF0123456789ABCDEF",
		ProductNameZhCN: "云服务器", ProductNameEnUS: "Cloud server",
		ChannelName: "生产资源", PoolName: "华东资源池",
		Kind: "cloud_server", State: "ready", LatestOperation: "idle",
		AvailableActions: []resourcecap.ServerActionCode{resourcecap.ServerActionReboot},
		BoundAt:          1_700_000_000_000, UpdatedAt: 1_700_000_000_000,
	}}
	controller := NewV1(servers)

	list, err := controller.ServerList(context.Background(), &v1.ServerListReq{Cursor: "cursor", Limit: 20})
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if servers.listRequest.Cursor != "cursor" || servers.listRequest.Limit != 20 {
		t.Fatalf("unexpected trusted list request: %#v", servers.listRequest)
	}
	if len(list.Items) != 1 || list.Items[0].ServerNo != servers.item.ServerNo ||
		!reflect.DeepEqual(list.Items[0].AvailableActions, []string{"reboot"}) {
		t.Fatalf("unexpected list projection: %#v", list)
	}

	detail, err := controller.ServerDetail(context.Background(), &v1.ServerDetailReq{ServerNo: servers.item.ServerNo})
	if err != nil {
		t.Fatalf("read server detail: %v", err)
	}
	if servers.detailRequest.ServerNo != servers.item.ServerNo ||
		detail.Item.ProductNameEnUS != "Cloud server" {
		t.Fatalf("unexpected trusted detail projection: request=%#v detail=%#v", servers.detailRequest, detail)
	}
}

func TestServerRequestsDoNotAcceptCustomerNumber(t *testing.T) {
	t.Parallel()
	for _, request := range []any{v1.ServerListReq{}, v1.ServerDetailReq{}} {
		typeOf := reflect.TypeOf(request)
		for index := 0; index < typeOf.NumField(); index++ {
			field := typeOf.Field(index)
			if field.Name == "CustomerNo" || field.Tag.Get("json") == "customerNo" {
				t.Fatalf("customer number leaked into %s", typeOf.Name())
			}
		}
	}
	itemType := reflect.TypeOf(v1.ServerItem{})
	if _, exists := itemType.FieldByName("ActionReference"); exists {
		t.Fatal("customer server response exposes the hidden action reference")
	}
}
