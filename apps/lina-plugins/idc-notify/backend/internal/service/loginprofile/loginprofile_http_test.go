package loginprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func TestCurrentUsesAuthenticatedLocalRouteAndRejectsInvalidProfiles(t *testing.T) {
	server := g.Server(t.Name())
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	server.SetAccessLogEnabled(false)
	server.SetErrorLogEnabled(false)
	body := `{"code":0,"data":{"id":42,"email":"old@example.com","phone":"13800138000"}}`
	status := 200
	calls := 0
	server.BindHandler("/api/v1/user/profile", func(r *ghttp.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer local-proof" {
			r.Response.WriteHeader(401)
			return
		}
		r.Response.Header().Set("Location", "https://example.invalid/must-not-follow")
		r.Response.WriteHeader(status)
		r.Response.Write(body)
	})
	reader := New()
	server.BindHandler("/probe", func(r *ghttp.Request) {
		profile, err := reader.Current(r.Context())
		r.Response.WriteJson(struct {
			ID       int
			Rejected bool
		}{profile.ID, err != nil})
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	for _, tc := range []struct {
		name, body    string
		status        int
		auth, allowed bool
	}{
		{"valid", body, 200, true, true},
		{"no authentication", body, 200, false, false},
		{"redirect", body, 302, true, false},
		{"business rejection", `{"code":403,"data":{"id":42}}`, 200, true, false},
		{"invalid identity", `{"code":0,"data":{"id":0}}`, 200, true, false},
		{"oversized", body + strings.Repeat(" ", 32769), 200, true, false},
		{"invalid JSON", `{`, 200, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, status = tc.body, tc.status
			before := calls
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/probe", server.GetListenedPort()), nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = "untrusted.example.invalid"
			if tc.auth {
				req.Header.Set("Authorization", "Bearer local-proof")
			}
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			content, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil || closeErr != nil {
				t.Fatal(readErr, closeErr)
			}
			var result struct {
				ID       int
				Rejected bool
			}
			if err = json.Unmarshal(content, &result); err != nil {
				t.Fatal(err)
			}
			if result.Rejected == tc.allowed || tc.allowed && result.ID != 42 {
				t.Fatalf("unexpected local profile result: %s", content)
			}
			expected := before
			if tc.auth {
				expected++
			}
			if calls != expected {
				t.Fatal("profile authentication or redirect boundary violated")
			}
		})
	}
	if _, err := reader.Current(context.Background()); err == nil {
		t.Fatal("missing request context accepted")
	}
}
