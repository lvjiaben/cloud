// This file exercises generated GoFrame HTTP parameter binding and the existing
// account adapter. It does not replace the host's signed-ticket E2E verification.

package serverpage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
	accountcontroller "lina-plugin-idc-resource/backend/internal/controller/account"
)

// pageServerReader records only the already-authenticated account service contract.
type pageServerReader struct {
	mu                     sync.Mutex
	list                   resourcecap.CurrentCustomerServerListRequest
	detail                 resourcecap.CurrentCustomerServerGetRequest
	listCalls, detailCalls int
}

func (reader *pageServerReader) ListCurrentCustomerServers(ctx context.Context, input resourcecap.CurrentCustomerServerListRequest) (*resourcecap.CurrentCustomerServerPage, error) {
	reader.mu.Lock()
	defer reader.mu.Unlock()
	reader.list, reader.listCalls = input, reader.listCalls+1
	return &resourcecap.CurrentCustomerServerPage{Items: []resourcecap.CurrentCustomerServerItem{}, NextCursor: "next"}, nil
}
func (reader *pageServerReader) GetCurrentCustomerServer(ctx context.Context, input resourcecap.CurrentCustomerServerGetRequest) (*resourcecap.CurrentCustomerServerResult, error) {
	if err := resourcecap.ValidateCurrentCustomerServerGetRequest(input); err != nil {
		return nil, err
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	reader.detail, reader.detailCalls = input, reader.detailCalls+1
	return &resourcecap.CurrentCustomerServerResult{Item: resourcecap.CurrentCustomerServerItem{ServerNo: input.ServerNo}}, nil
}

func TestServerPageHTTPBindingAndDirectAccountContract(t *testing.T) {
	var (
		reader  = &pageServerReader{}
		account = accountcontroller.NewV1(reader)
		server  = ghttp.GetServer(guid.S())
		client  = &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	)
	controller, err := NewV1(account)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewV1(nil); err == nil {
		t.Fatal("missing account dependency was accepted")
	}
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	server.SetAccessLogEnabled(false)
	server.Group("/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		group.Bind(controller, account)
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	request := func(path string) (int, json.RawMessage) {
		t.Helper()
		response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1%s", server.GetListenedPort(), path))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := response.Body.Close(); err != nil {
				t.Error(err)
			}
		}()
		var envelope struct {
			Code int             `json:"code"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Code, envelope.Data
	}
	if code, _ := request("/account/plugin-servers?cursor=v1.safe&limit=2&customerNo=foreign"); code != 0 {
		t.Fatalf("valid page code=%d", code)
	}
	reader.mu.Lock()
	if reader.list.Cursor != "v1.safe" || reader.list.Limit != 2 || reader.listCalls != 1 {
		t.Fatalf("incorrect bounded list: %+v", reader.list)
	}
	reader.mu.Unlock()
	for _, query := range []string{"limit=51", "limit=-1", "cursor=" + strings.Repeat("A", 65)} {
		if code, _ := request("/account/plugin-servers?" + query); code == 0 {
			t.Fatalf("invalid list accepted %s", query)
		}
	}
	serverNo := "SVR-" + strings.Repeat("A", 32)
	if code, data := request("/account/plugin-servers/" + serverNo + "?serverNo=SVR-" + strings.Repeat("B", 32) + "&customerNo=foreign"); code != 0 || !strings.Contains(string(data), serverNo) {
		t.Fatalf("path did not dominate payload: code=%d data=%s", code, data)
	}
	if code, _ := request("/account/plugin-servers/not-a-server"); code == 0 {
		t.Fatal("invalid server key accepted")
	}
	if code, _ := request("/account/plugin-servers/not-a-server?serverNo=" + serverNo); code == 0 {
		t.Fatal("query turned an invalid ticket path into a valid target")
	}
	if code, _ := request("/account/servers?limit=3"); code != 0 {
		t.Fatal("direct list broken")
	}
	if code, _ := request("/account/servers/" + serverNo); code != 0 {
		t.Fatal("direct detail broken")
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if reader.listCalls != 2 || reader.detailCalls != 2 || reader.detail.ServerNo != serverNo {
		t.Fatalf("invalid request reached owner or direct route changed: %+v", reader)
	}
}
