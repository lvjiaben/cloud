package accountnative

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
)

// The stub isolates HTTP binding; ownership and transactions have real PG tests.
type nativeHTTPService struct {
	operations
	mu          sync.Mutex
	creates     int
	serverNo    string
	command     customeroperations.Command
	page, limit int
	action      string
}

func (s *nativeHTTPService) Create(_ context.Context, command customeroperations.Command) (*customeroperations.Operation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creates++
	s.command = command
	if command.ServerNo != s.serverNo {
		return nil, customeroperations.ErrUnavailable
	}
	return &customeroperations.Operation{OperationNo: "nso_" + strings.Repeat("1", 32), Action: command.Action, Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (s *nativeHTTPService) Operations(_ context.Context, serverNo string, page, limit int, action string) (*customeroperations.OperationPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.page, s.limit, s.action = page, limit, action
	if serverNo != s.serverNo {
		return nil, customeroperations.ErrUnavailable
	}
	return &customeroperations.OperationPage{Items: []customeroperations.Operation{}, Total: 27}, nil
}
func TestNativeManagementHTTPBindingAndValidation(t *testing.T) {
	service := &nativeHTTPService{serverNo: "SVR-" + strings.Repeat("A", 32)}
	// Only tested free-management routes are bound; no test can invoke a fake
	// commercial service or cross into a real upstream through this controller.
	controller := &ControllerV1{service: service}
	server := ghttp.GetServer(guid.S())
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	server.SetAccessLogEnabled(false)
	server.Group("/api/v1", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		group.Bind(controller.NativeServerOperations, controller.NativeServerOperationCreate)
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	request := func(method, path, body string) (int, json.RawMessage) {
		t.Helper()
		req, err := http.NewRequest(method, fmt.Sprintf("http://127.0.0.1:%d/api/v1%s", server.GetListenedPort(), path), bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
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
	path := "/account/native-servers/" + service.serverNo + "/operations"
	for _, action := range []string{"snapshot_restore", "snapshot_delete", "backup_restore", "backup_delete"} {
		body := fmt.Sprintf(`{"serverNo":"SVR-%s","customerNo":"foreign","action":%q,"parameters":{"objectNo":"ACO_%s","confirmedServiceNo":%q},"confirmed":true,"idempotencyKey":"http-test"}`, strings.Repeat("B", 32), action, strings.Repeat("1", 32), service.serverNo)
		if code, _ := request(http.MethodPost, path, body); code != 0 {
			t.Fatalf("valid %s rejected: %d", action, code)
		}
		service.mu.Lock()
		bound := service.command
		service.mu.Unlock()
		if bound.ServerNo != service.serverNo || bound.Action != action || !bound.Confirmed {
			t.Fatal("path or action not bound to owned route")
		}
	}
	service.mu.Lock()
	before := service.creates
	service.mu.Unlock()
	for _, body := range []string{`{"action":"snapshot_create","idempotencyKey":"http-test"}`, `{"action":"stop"}`, `{"action":"stop","idempotencyKey":"bad key"}`} {
		if code, _ := request(http.MethodPost, path, body); code == 0 {
			t.Fatal("invalid free-management request reached service")
		}
	}
	if code, _ := request(http.MethodPost, "/account/native-servers/not-a-server/operations", `{"serverNo":"`+service.serverNo+`","action":"stop","idempotencyKey":"http-test"}`); code == 0 {
		t.Fatal("body overrode invalid path")
	}
	service.mu.Lock()
	after := service.creates
	service.mu.Unlock()
	if after != before {
		t.Fatal("invalid request called write service")
	}
	if code, data := request(http.MethodGet, path+"?page=3&limit=10&action=snapshot_restore&customerNo=foreign", ""); code != 0 || !bytes.Contains(data, []byte(`"total":27`)) {
		t.Fatalf("history pagination lost exact total: %d %s", code, data)
	}
	service.mu.Lock()
	page, limit, action := service.page, service.limit, service.action
	service.mu.Unlock()
	if page != 3 || limit != 10 || action != "snapshot_restore" {
		t.Fatal("query pagination/filter not bound")
	}
	for _, query := range []string{"limit=51", "page=0", "limit=-1"} {
		if code, _ := request(http.MethodGet, path+"?"+query, ""); code == 0 {
			t.Fatal("invalid page accepted")
		}
	}
}
