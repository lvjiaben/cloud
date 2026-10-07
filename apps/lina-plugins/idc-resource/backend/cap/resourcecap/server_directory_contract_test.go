// This file verifies customer-safe server directory contract bounds.

package resourcecap

import (
	"reflect"
	"testing"
)

func TestCurrentCustomerServerContractContainsOnlyPublicReferences(t *testing.T) {
	t.Parallel()
	requestType := reflect.TypeOf(CurrentCustomerServerListRequest{})
	for index := 0; index < requestType.NumField(); index++ {
		field := requestType.Field(index)
		if field.Name == "CustomerNo" || field.Tag.Get("json") == "customerNo" {
			t.Fatal("current-customer request must not accept a customer number")
		}
	}
	itemType := reflect.TypeOf(CurrentCustomerServerItem{})
	for _, forbidden := range []string{"ActionReference", "EntitlementNo", "MappingNo", "TaskNo", "ProviderPluginID"} {
		if _, exists := itemType.FieldByName(forbidden); exists {
			t.Fatalf("customer-safe item exposes %s", forbidden)
		}
	}
	if _, exists := itemType.FieldByName("ServerNo"); !exists {
		t.Fatal("customer-safe item is missing the public server number")
	}
}

func TestCurrentCustomerServerContractValidatesBoundsAndOwnershipKey(t *testing.T) {
	t.Parallel()
	request := CurrentCustomerServerListRequest{Limit: MaximumCurrentCustomerServerPageSize}
	item := CurrentCustomerServerItem{
		ServerNo:        "SVR-0123456789ABCDEF0123456789ABCDEF",
		ProductNameZhCN: "云服务器", ProductNameEnUS: "Cloud server",
		ChannelName: "生产资源", PoolName: "华东资源池",
		Kind: "cloud_server", State: "ready", LatestOperation: "idle",
		AvailableActions: []ServerActionCode{ServerActionStart}, BoundAt: 1, UpdatedAt: 1,
	}
	page := &CurrentCustomerServerPage{Items: []CurrentCustomerServerItem{item}}
	if err := ValidateCurrentCustomerServerListRequest(request); err != nil {
		t.Fatalf("valid list request: %v", err)
	}
	if err := ValidateCurrentCustomerServerPage(page, request); err != nil {
		t.Fatalf("valid page: %v", err)
	}
	get := CurrentCustomerServerGetRequest{ServerNo: item.ServerNo}
	if err := ValidateCurrentCustomerServerResult(&CurrentCustomerServerResult{Item: item}, get); err != nil {
		t.Fatalf("valid detail: %v", err)
	}
	if ValidateCurrentCustomerServerListRequest(CurrentCustomerServerListRequest{Limit: MaximumCurrentCustomerServerPageSize + 1}) == nil {
		t.Fatal("oversized page request was accepted")
	}
	get.ServerNo = "srv_0123456789abcdef0123456789abcdef"
	if ValidateCurrentCustomerServerGetRequest(get) == nil {
		t.Fatal("hidden action reference was accepted as a public server number")
	}
}
