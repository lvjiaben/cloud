// This file verifies the shared route and object-ownership access boundary.

package customer

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestNewAccessGuardRejectsMissingAndTypedNilDependencies(t *testing.T) {
	principalSvc := activeTestPrincipal()
	if _, err := NewAccessGuard(nil, principalSvc); err == nil {
		t.Fatal("missing host bizctx service must fail")
	}
	if _, err := NewAccessGuard(staticBizCtx{}, nil); err == nil {
		t.Fatal("missing principal service must fail")
	}
	var typedNil *staticPrincipal
	if _, err := NewAccessGuard(staticBizCtx{}, typedNil); err == nil {
		t.Fatal("typed-nil principal service must fail")
	}
}

func TestAccessGuardMiddlewareInjectsOwnedSubjectOnce(t *testing.T) {
	principalSvc := activeTestPrincipal()
	guard, err := NewAccessGuard(
		staticBizCtx{current: bizctxcap.CurrentContext{UserID: 81}},
		principalSvc,
	)
	if err != nil {
		t.Fatalf("create access guard: %v", err)
	}

	var handlerErr string
	called, observed, body := runAccessMiddlewareRequest(t, guard, func(request *ghttp.Request) {
		subject, ok := AccessSubjectFromContext(request.Context())
		if !ok {
			handlerErr = "customer access subject was not injected"
			return
		}
		owned, requireErr := guard.RequireOwned(request.Context(), testCustomerNo)
		if requireErr != nil {
			handlerErr = requireErr.Error()
			return
		}
		if owned != subject {
			handlerErr = "middleware and ownership subjects differ"
			return
		}
		request.Response.Write(subject.CustomerNo)
	})
	if !called || observed != nil || handlerErr != "" {
		t.Fatalf("called=%t observed=%v handlerErr=%q", called, observed, handlerErr)
	}
	if strings.TrimSpace(body) != testCustomerNo {
		t.Fatalf("response body=%q, want owned customer number", body)
	}
	if principalSvc.calls != 1 {
		t.Fatalf("principal lookups=%d, want one middleware lookup", principalSvc.calls)
	}
}

func TestAccessGuardMiddlewareRejectsAdministrativeContextBeforePrincipal(t *testing.T) {
	principalSvc := activeTestPrincipal()
	guard, err := NewAccessGuard(
		staticBizCtx{current: bizctxcap.CurrentContext{
			UserID: 81, Permissions: []string{"idc:admin:customer:list"},
		}},
		principalSvc,
	)
	if err != nil {
		t.Fatalf("create access guard: %v", err)
	}

	called, observed, _ := runAccessMiddlewareRequest(t, guard, func(*ghttp.Request) {})
	if called {
		t.Fatal("administrative identity reached customer handler")
	}
	if !bizerr.Is(observed, CodeNotAccessible) {
		t.Fatalf("middleware error=%v, want inaccessible", observed)
	}
	if principalSvc.calls != 0 {
		t.Fatalf("administrative identity crossed context guard: calls=%d", principalSvc.calls)
	}
}

func TestAccessGuardRequireOwnedCollapsesForeignAndMalformedTargets(t *testing.T) {
	principalSvc := activeTestPrincipal()
	guard, err := NewAccessGuard(
		staticBizCtx{current: bizctxcap.CurrentContext{UserID: 81}},
		principalSvc,
	)
	if err != nil {
		t.Fatalf("create access guard: %v", err)
	}
	ctx := withAccessSubject(context.Background(), principalSvc.audience)
	for _, target := range []string{
		"CUS_ffffffffffffffffffffffffffffffff",
		"malformed",
		"",
	} {
		if _, requireErr := guard.RequireOwned(ctx, target); !bizerr.Is(requireErr, CodeNotAccessible) {
			t.Fatalf("target %q returned %v, want inaccessible", target, requireErr)
		}
	}
	if principalSvc.calls != 0 {
		t.Fatalf("trusted request subject should avoid repeated principal lookups: %d", principalSvc.calls)
	}
}

// TestAccessGuardRejectsRevokedCustomerOnNextBusinessRequest proves that a
// prior identity proof (for example, grant issuance) cannot bypass the next
// actual customer route when the owning audience is disabled or unbound.
func TestAccessGuardRejectsRevokedCustomerOnNextBusinessRequest(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		revoke func(*staticPrincipal)
	}{
		{name: "revoked", revoke: func(reader *staticPrincipal) { reader.audience.Status = principal.AudienceStatusRevoked }},
		{name: "unbound", revoke: func(reader *staticPrincipal) { reader.audience.CustomerNo = "" }},
		{name: "staff", revoke: func(reader *staticPrincipal) { reader.audience.Code = principal.AudienceStaff }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			reader := activeTestPrincipal()
			guard, err := NewAccessGuard(staticBizCtx{current: bizctxcap.CurrentContext{UserID: 81}}, reader)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = guard.Resolve(context.Background()); err != nil {
				t.Fatal(err)
			}
			testCase.revoke(reader)
			called, observed, _ := runAccessMiddlewareRequest(t, guard, func(*ghttp.Request) {})
			if called || !bizerr.Is(observed, CodeNotAccessible) || reader.calls != 2 {
				t.Fatalf("prior proof survived revocation: called=%t error=%v lookups=%d", called, observed, reader.calls)
			}
		})
	}
}

func runAccessMiddlewareRequest(
	t *testing.T,
	guard *AccessGuard,
	handler func(request *ghttp.Request),
) (called bool, observed error, body string) {
	t.Helper()
	server := g.Server("customer-access-" + guid.S())
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	server.Group("/", func(group *ghttp.RouterGroup) {
		group.Middleware(
			func(request *ghttp.Request) {
				request.Middleware.Next()
				observed = request.GetError()
			},
			guard.Middleware,
		)
		group.GET("/owned", func(request *ghttp.Request) {
			called = true
			handler(request)
		})
	})
	if err := server.Start(); err != nil {
		t.Fatalf("start customer access middleware server: %v", err)
	}
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Errorf("shutdown customer access middleware server: %v", err)
		}
	})
	time.Sleep(100 * time.Millisecond)

	response, err := http.Get("http://" + server.GetListenedAddress() + "/owned")
	if err != nil {
		t.Fatalf("send customer access middleware request: %v", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("close customer access middleware response: %v", closeErr)
		}
	}()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read customer access middleware response: %v", err)
	}
	return called, observed, string(contents)
}
