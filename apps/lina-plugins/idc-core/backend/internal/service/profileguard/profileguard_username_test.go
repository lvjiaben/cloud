// This file verifies username-policy rejection before original handlers write,
// and preserves unchanged legacy usernames in normal edit forms.
package profileguard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"lina-core/pkg/plugin/capability/usercap"
)

type usernameUsers struct{ usercap.Service }

func (usernameUsers) Get(_ context.Context, id usercap.UserID) (*usercap.UserInfo, error) {
	if id == "42" {
		return &usercap.UserInfo{ID: id, Username: "old@example.test"}, nil
	}
	if id == "1" {
		return &usercap.UserInfo{ID: id, Username: "12345"}, nil
	}
	return nil, nil
}

type usernameCreateInput struct {
	g.Meta   `path:"/api/v1/auth/register" method:"post"`
	Username any `json:"username"`
}
type usernameAdminInput struct {
	g.Meta   `path:"/api/v1/user" method:"post"`
	Username any `json:"username"`
}
type usernameEditInput struct {
	g.Meta   `path:"/api/v1/user/{id}" method:"put"`
	Username any `json:"username"`
}
type usernameController struct{ calls atomic.Int64 }

func (c *usernameController) Register(context.Context, *usernameCreateInput) (*profileResult, error) {
	c.calls.Add(1)
	return &profileResult{Saved: true}, nil
}
func (c *usernameController) Create(context.Context, *usernameAdminInput) (*profileResult, error) {
	c.calls.Add(1)
	return &profileResult{Saved: true}, nil
}
func (c *usernameController) Update(context.Context, *usernameEditInput) (*profileResult, error) {
	c.calls.Add(1)
	return &profileResult{Saved: true}, nil
}

func TestUsernamePolicyAtPublicWriteBoundary(t *testing.T) {
	s := g.Server(t.Name())
	s.SetAddr("127.0.0.1:0")
	s.SetDumpRouterMap(false)
	s.SetAccessLogEnabled(false)
	s.SetErrorLogEnabled(false)
	protection := &guard{translations: testTranslations{}, bizctx: testContext{}, users: usernameUsers{}}
	s.BindMiddleware("/*", onlyUsernameCreate(protection.handleUsernameCreate))
	s.BindMiddleware("/*", onlyProfileUpdate(func(r *ghttp.Request) {
		if protection.checkUsername(r, false) {
			r.Middleware.Next()
		}
	}))
	controller := &usernameController{}
	s.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(ghttp.MiddlewareHandlerResponse)
		group.Bind(controller)
	})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	base := fmt.Sprintf("http://127.0.0.1:%d", s.GetListenedPort())
	for _, tc := range []struct {
		name, method, path, body, contentType string
		allowed                               bool
	}{
		{"register numeric", http.MethodPost, "/api/v1/auth/register", `{"username":"12345"}`, "application/json", false},
		{"register unicode numeric", http.MethodPost, "/api/v1/auth/register", `{"username":"１２３４５"}`, "application/json", false},
		{"register email", http.MethodPost, "/api/v1/auth/register", `{"username":"new@example.test"}`, "application/json", false},
		{"register international phone", http.MethodPost, "/api/v1/auth/register", `{"username":"+8613800138000"}`, "application/json", false},
		{"register independent", http.MethodPost, "/api/v1/auth/register", `{"username":"customer_123"}`, "application/json", true},
		{"admin numeric", http.MethodPost, "/api/v1/user", `{"username":12345}`, "application/json", false},
		{"form numeric", http.MethodPost, "/api/v1/user", "username=12345", "application/x-www-form-urlencoded", false},
		{"query numeric", http.MethodPost, "/api/v1/user?username=12345", `{}`, "application/json", false},
		{"admin independent", http.MethodPost, "/api/v1/user", `{"username":"客户123"}`, "application/json", true},
		{"edit email", http.MethodPut, "/api/v1/user/42", `{"username":"new@example.test"}`, "application/json", false},
		{"unchanged legacy email", http.MethodPut, "/api/v1/user/42", `{"username":"old@example.test","nickname":"Changed"}`, "application/json", true},
		{"unchanged legacy numeric", http.MethodPut, "/api/v1/user/1", `{"username":"12345","nickname":"Changed"}`, "application/json", true},
		{"unseen legacy email", http.MethodPut, "/api/v1/user/43", `{"username":"old@example.test"}`, "application/json", false},
		{"rename independent", http.MethodPut, "/api/v1/user/42", `{"username":"new_customer"}`, "application/json", true},
		{"nickname only", http.MethodPut, "/api/v1/user/42", `{"nickname":"Changed"}`, "application/json", true},
		{"password only", http.MethodPut, "/api/v1/user/42", `{"password":"new-password"}`, "application/json", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := controller.calls.Load()
			request, err := http.NewRequest(tc.method, base+tc.path, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", tc.contentType)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if tc.allowed {
				if response.StatusCode != http.StatusOK || controller.calls.Load() != before+1 {
					t.Fatalf("valid or unchanged save rejected: %d %s", response.StatusCode, body)
				}
				return
			}
			var envelope struct {
				ErrorCode string `json:"errorCode"`
			}
			if err := json.Unmarshal(body, &envelope); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusBadRequest || envelope.ErrorCode != codeUsernameInvalid.RuntimeCode() || controller.calls.Load() != before {
				t.Fatalf("forbidden username reached write: %d %s", response.StatusCode, body)
			}
		})
	}
}
