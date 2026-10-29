package profileguard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/profilechangecap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type testAudience struct{ code principal.AudienceCode }

func (a *testAudience) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{UserID: 1, Code: a.code}, nil
}

type testContext struct{}

func (testContext) Current(context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: 1, TenantID: 0}
}

type testTranslations struct{}

func (testTranslations) GetLocale(context.Context) string { return "en-US" }
func (testTranslations) Translate(_ context.Context, _ string, fallback string) string {
	return fallback
}

type profileInput struct {
	g.Meta   `path:"/api/v1/user/profile" method:"put"`
	Nickname *string `json:"nickname"`
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
	Password *string `json:"password"`
}
type profileResult struct {
	Saved bool `json:"saved"`
}
type profileController struct{ calls atomic.Int64 }

func (c *profileController) Save(_ context.Context, _ *profileInput) (*profileResult, error) {
	c.calls.Add(1)
	return &profileResult{Saved: true}, nil
}

func TestProfileGuardRejectsCredentialInputsBeforeBinding(t *testing.T) {
	s := g.Server(t.Name())
	s.SetAddr("127.0.0.1:0")
	s.SetDumpRouterMap(false)
	s.SetAccessLogEnabled(false)
	s.SetErrorLogEnabled(false)
	audience := &testAudience{code: principal.AudienceCustomer}
	protection := &guard{translations: testTranslations{}, audiences: audience, bizctx: testContext{}}
	s.BindMiddleware("/*", onlyProfileUpdate(protection.handle))
	controller := &profileController{}
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
	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	if err := writer.WriteField("email", "new@example.test"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, body, contentType string
		allowed                       bool
	}{
		{"email", "/api/v1/user/profile", `{"email":"new@example.test"}`, "application/json", false},
		{"phone", "/api/v1/user/profile", `{"phone":"13800138000"}`, "application/json", false},
		{"case", "/api/v1/user/profile", `{"Email":"new@example.test"}`, "application/json", false},
		{"separator", "/api/v1/user/profile", `{"e_mail":"new@example.test"}`, "application/json", false},
		{"empty contact", "/api/v1/user/profile", `{"email":""}`, "application/json", false},
		{"null contact", "/api/v1/user/profile", `{"phone":null}`, "application/json", false},
		{"query", "/api/v1/user/profile?phone=13800138000", `{"nickname":"safe"}`, "application/json", false},
		{"form", "/api/v1/user/profile", "email=new%40example.test", "application/x-www-form-urlencoded", false},
		{"multipart", "/api/v1/user/profile", multipartBody.String(), writer.FormDataContentType(), false},
		{"trailing slash", "/api/v1/user/profile/", `{"email":"new@example.test"}`, "application/json", false},
		{"large body", "/api/v1/user/profile", strings.Repeat("x", maxProfileBody+1), "application/json", false},
		{"nickname", "/api/v1/user/profile", `{"nickname":"Changed"}`, "application/json", true},
		{"password existing flow", "/api/v1/user/profile", `{"password":"Existing-password-flow"}`, "application/json", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := controller.calls.Load()
			req, err := http.NewRequest(http.MethodPut, base+tc.path, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", tc.contentType)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			payload, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if tc.allowed {
				if res.StatusCode != http.StatusOK || controller.calls.Load() != before+1 {
					t.Fatalf("normal save blocked: %d %s", res.StatusCode, payload)
				}
			} else {
				var envelope struct {
					ErrorCode string `json:"errorCode"`
				}
				if err := json.Unmarshal(payload, &envelope); err != nil {
					t.Fatal(err)
				}
				if res.StatusCode != http.StatusForbidden || envelope.ErrorCode != codeContactVerificationRequired.RuntimeCode() || controller.calls.Load() != before {
					t.Fatalf("unverified change reached handler: %d %s", res.StatusCode, payload)
				}
			}
		})
	}
	audience.code = principal.AudienceStaff
	req, err := http.NewRequest(http.MethodPut, base+"/api/v1/user/profile", strings.NewReader(`{"nickname":"Staff","email":"staff@example.test","phone":"13800138000"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	before := controller.calls.Load()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || controller.calls.Load() != before+1 {
		t.Fatal("staff full-profile save was blocked")
	}

}

type guardPluginState struct{ enabled bool }

func (s *guardPluginState) IsEnabled(context.Context, plugincap.PluginID) (bool, error) {
	return s.enabled, nil
}
func (s *guardPluginState) IsProviderEnabled(context.Context, plugincap.PluginID) (bool, error) {
	return s.enabled, nil
}
func (s *guardPluginState) IsEnabledAuthoritative(context.Context, plugincap.PluginID) (bool, error) {
	return s.enabled, nil
}

type guardVerifier struct {
	calls int
	input profilechangecap.Input
}

func (v *guardVerifier) Apply(ctx context.Context, input profilechangecap.Input, write profilechangecap.Write) error {
	v.calls++
	v.input = input
	return write(ctx)
}
func (v *guardVerifier) Serialize(ctx context.Context, _ int, write profilechangecap.Write) error {
	return write(ctx)
}
func TestProfileGuardVerifiedWriteAndDisabledOwner(t *testing.T) {
	s := g.Server(t.Name())
	s.SetAddr("127.0.0.1:0")
	s.SetDumpRouterMap(false)
	s.SetAccessLogEnabled(false)
	s.SetErrorLogEnabled(false)
	state := &guardPluginState{enabled: true}
	verifier := &guardVerifier{}
	controller := &profileController{}
	protection := &guard{translations: testTranslations{}, audiences: &testAudience{code: principal.AudienceCustomer}, bizctx: testContext{}, plugins: state, verifier: func() profilechangecap.Provider { return verifier }}
	s.BindMiddleware("/*", onlyProfileUpdate(protection.handle))
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
		name, body, proof string
		enabled, allowed  bool
	}{
		{"verified", `{"email":"new@example.com"}`, "nver_old.nver_new", true, true},
		{"missing proof", `{"email":"new@example.com"}`, "", true, false},
		{"two fields", `{"email":"new@example.com","phone":"13900139000"}`, "nver_old.nver_new", true, false},
		{"ambiguous key", `{"email":"new@example.com","Email":"other@example.com"}`, "nver_old.nver_new", true, false},
		{"disabled owner", `{"email":"new@example.com"}`, "nver_old.nver_new", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state.enabled = tc.enabled
			before := controller.calls.Load()
			req, err := http.NewRequest(http.MethodPut, base+"/api/v1/user/profile", strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Contact-Verification", tc.proof)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if tc.allowed {
				if res.StatusCode != 200 || controller.calls.Load() != before+1 || verifier.input.OldVerification != "nver_old" || verifier.input.NewVerification != "nver_new" || verifier.input.Email == nil || *verifier.input.Email != "new@example.com" {
					t.Fatal("verified write did not reach original handler")
				}
			} else if res.StatusCode != 403 || controller.calls.Load() != before {
				t.Fatal("invalid contact update reached handler")
			}
		})
	}
	if verifier.calls != 1 {
		t.Fatal("invalid input reached verifier")
	}
}
