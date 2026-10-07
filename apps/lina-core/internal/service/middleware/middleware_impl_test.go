// This file verifies optional authentication preserves anonymous discovery and
// delegates supplied identities to the existing online-session authority.

package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"

	"lina-core/internal/model"
	"lina-core/internal/service/auth"
	"lina-core/internal/service/bizctx"
	"lina-core/pkg/bizerr"
	tokencap "lina-core/pkg/plugin/capability/authcap/token"
)

// optionalAuthAuthority records the single shared authentication call without
// replacing the production token parser or introducing persistent test data.
type optionalAuthAuthority struct {
	auth.Service
	claims *auth.Claims
	calls  int
	token  string
}

// AuthenticateAccessToken represents either an accepted online session or the
// authority's invalid/revoked-session error, without logging token contents.
func (s *optionalAuthAuthority) AuthenticateAccessToken(_ context.Context, token string) (*auth.Claims, error) {
	s.calls++
	s.token = token
	if s.claims == nil {
		return nil, bizerr.NewCode(auth.CodeAuthTokenInvalid)
	}
	return s.claims, nil
}

// TestOptionalAuthPreservesDiscoveryIdentity checks request-local customer and
// acting-staff fields, anonymous reads, and fail-closed invalid identities.
func TestOptionalAuthPreservesDiscoveryIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		claims *auth.Claims
		status int
		calls  int
	}{
		{name: "anonymous", status: http.StatusOK},
		{name: "malformed header", header: "Basic invalid", status: http.StatusUnauthorized},
		{name: "invalid or revoked token", header: "Bearer invalid", status: http.StatusUnauthorized, calls: 1},
		{name: "customer", header: "Bearer valid", claims: &auth.Claims{TokenId: "fixture-session", UserId: 42, Username: "customer", Status: 1, TenantId: 18, ClientType: tokencap.ClientTypeWeb}, status: http.StatusOK, calls: 1},
		{name: "staff assistance", header: "Bearer valid", claims: &auth.Claims{TokenId: "fixture-assistance", UserId: 42, Username: "customer", Status: 1, TenantId: 18, ActingUserId: 7, IsImpersonation: true, ClientType: tokencap.ClientTypeWeb}, status: http.StatusOK, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var (
				authority = &optionalAuthAuthority{claims: tc.claims}
				contexts  = bizctx.New()
				svc       = &serviceImpl{authSvc: authority, bizCtxSvc: contexts}
				server    = ghttp.GetServer("optional-auth-" + guid.S())
			)
			server.SetPort(0)
			server.SetDumpRouterMap(false)
			server.Group("/", func(group *ghttp.RouterGroup) {
				group.Middleware(func(r *ghttp.Request) { contexts.Init(r, &model.Context{}); r.Middleware.Next() }, svc.OptionalAuth)
				group.GET("/discovery", func(r *ghttp.Request) { r.Response.WriteJson(contexts.Get(r.Context())) })
			})
			if err := server.Start(); err != nil {
				t.Fatalf("start optional authentication server: %v", err)
			}
			t.Cleanup(func() {
				if err := server.Shutdown(); err != nil {
					t.Errorf("shutdown optional authentication server: %v", err)
				}
			})
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/discovery", server.GetListenedPort()), nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Authorization", tc.header)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := response.Body.Close(); err != nil {
					t.Errorf("close authentication response: %v", err)
				}
			})
			if response.StatusCode != tc.status || authority.calls != tc.calls {
				t.Fatalf("status=%d calls=%d; want status=%d calls=%d", response.StatusCode, authority.calls, tc.status, tc.calls)
			}
			if tc.status != http.StatusOK {
				return
			}
			var actual model.Context
			if err := json.NewDecoder(response.Body).Decode(&actual); err != nil {
				t.Fatal(err)
			}
			if tc.claims == nil {
				if actual.UserId != 0 || actual.ActingUserId != 0 || actual.TokenId != "" {
					t.Fatal("anonymous discovery acquired authenticated identity")
				}
				return
			}
			if authority.token != "valid" || actual.UserId != tc.claims.UserId || actual.TokenId != tc.claims.TokenId || actual.TenantId != tc.claims.TenantId || actual.ActingUserId != tc.claims.ActingUserId || actual.IsImpersonation != tc.claims.IsImpersonation || actual.ActingAsTenant != tc.claims.ActingAsTenant || actual.ClientType != tc.claims.ClientType.String() {
				t.Fatal("authenticated discovery did not preserve authoritative session context")
			}
		})
	}
}
