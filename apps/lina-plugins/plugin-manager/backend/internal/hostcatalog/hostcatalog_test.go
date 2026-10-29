// This file tests public HTTP subrequests against a real GoFrame HTTP handler.
package hostcatalog

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
)

func TestReaderRepeatsPublicAuthorizationAndPreservesTenant(t *testing.T) {
	server := ghttp.GetServer(fmt.Sprintf("plugin-manager-%d", time.Now().UnixNano()))
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	calls := 0
	server.BindHandler("/api/v1/plugins", func(request *ghttp.Request) {
		calls++
		if request.Header.Get("Authorization") != "Bearer test-only" {
			request.Response.WriteJson(map[string]any{"code": 51, "message": "denied"})
			return
		}
		if request.Header.Get("X-Tenant-Id") != "7" || request.Header.Get("Cookie") != "" {
			t.Error("tenant was dropped or unapproved cookie forwarded")
		}
		if request.Get("type").String() != "dynamic" || request.Get("pageSize").Int() != 100 {
			t.Error("public type/page bounds lost")
		}
		request.Response.WriteJson(map[string]any{"code": 0, "data": map[string]any{"list": []any{map[string]any{"id": "example", "name": "Example", "type": "dynamic", "distribution": "managed"}}, "total": 1}})
	})
	server.BindHandler("/workspace", func(request *ghttp.Request) {
		out, err := (&Reader{}).List(request.Context(), 1, 100, "example", "")
		if err != nil {
			request.Response.WriteStatus(http.StatusForbidden)
			return
		}
		request.Response.WriteJson(out)
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	client := &http.Client{Timeout: 5 * time.Second}
	base := fmt.Sprintf("http://127.0.0.1:%d/workspace", server.GetListenedPort())
	request, err := http.NewRequest(http.MethodGet, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer test-only")
	request.Header.Set("X-Tenant-Id", "7")
	request.Header.Set("Cookie", "must-not-forward=private")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Total int `json:"total"`
	}
	decodeErr := json.NewDecoder(response.Body).Decode(&payload)
	closeErr := response.Body.Close()
	if decodeErr != nil || closeErr != nil || response.StatusCode != http.StatusOK || payload.Total != 1 {
		t.Fatalf("public summary request failed: %v %v %+v", decodeErr, closeErr, response.StatusCode)
	}
	response, err = client.Get(base)
	if err != nil {
		t.Fatal(err)
	}
	if err = response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden || calls != 2 {
		t.Fatalf("missing identity did not fail closed: status=%d calls=%d", response.StatusCode, calls)
	}
}
