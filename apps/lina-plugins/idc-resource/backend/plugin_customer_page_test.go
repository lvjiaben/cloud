// This file verifies the plugin's actual customer-page registration, middleware
// order and per-request owner proof without starting or modifying the host.

package backend

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/gogf/gf/v2/net/ghttp"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	accountv1 "lina-plugin-idc-resource/backend/api/account/v1"
	pagev1 "lina-plugin-idc-resource/backend/api/serverpage/v1"
)

// pageRouteCapture implements only the registrar operations this binding uses.
type pageRouteCapture struct {
	pluginhost.RouteGroup
	handlers []pluginhost.RouteMiddleware
	proof    func(context.Context) (int, error)
	bound    []any
}

func (group *pageRouteCapture) Group(prefix string, register func(pluginhost.RouteGroup)) {
	register(group)
}
func (group *pageRouteCapture) Middleware(handlers ...pluginhost.RouteMiddleware) {
	group.handlers = append(group.handlers, handlers...)
}
func (group *pageRouteCapture) BindCustomerPageBridge(proof func(context.Context) (int, error), bound ...any) {
	group.proof, group.bound = proof, bound
}

// pageMiddlewareCapture adds the real host's separate verifier slot to a test directory.
type pageMiddlewareCapture struct {
	pluginhost.RouteMiddlewares
	verifier pluginhost.RouteMiddleware
}

func (m pageMiddlewareCapture) PageBridge() pluginhost.RouteMiddleware { return m.verifier }

// pageCustomerReader holds test-controlled authoritative state and records context identity.
type pageCustomerReader struct {
	subject  *customerauthcap.Subject
	err      error
	contexts []context.Context
}

func (reader *pageCustomerReader) CurrentCustomer(ctx context.Context) (*customerauthcap.Subject, error) {
	reader.contexts = append(reader.contexts, ctx)
	return reader.subject, reader.err
}

func TestCustomerServerPageBindingUsesFreshOwnerProofAndVerifier(t *testing.T) {
	t.Parallel()
	var (
		group       = &pageRouteCapture{}
		reader      = &pageCustomerReader{subject: &customerauthcap.Subject{CustomerNo: "customer-a", ActorUserID: 42}}
		passthrough = func(*ghttp.Request) {}
		verifier    = func(*ghttp.Request) {}
		middlewares = pageMiddlewareCapture{RouteMiddlewares: pluginhost.NewRouteMiddlewares(passthrough, passthrough, passthrough, passthrough, passthrough, passthrough, passthrough, passthrough), verifier: verifier}
	)
	bindCustomerServerPageRoutes(group, middlewares, reader, nil)
	if group.proof == nil || len(group.handlers) != 8 || reflect.ValueOf(group.handlers[7]).Pointer() != reflect.ValueOf(verifier).Pointer() {
		t.Fatal("customer page binding omitted the ticket verifier after authentication and tenancy")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if actor, err := group.proof(ctx); err != nil || actor != 42 || reader.contexts[0] != ctx {
		t.Fatalf("owner proof did not retain original request context: actor=%d err=%v", actor, err)
	}
	denied := errors.New("test customer or assistance revoked")
	reader.subject, reader.err = nil, denied
	if actor, err := group.proof(ctx); actor != 0 || !errors.Is(err, denied) || len(reader.contexts) != 2 {
		t.Fatalf("owner proof cached prior customer or swallowed revocation: actor=%d err=%v", actor, err)
	}
	reader.err = nil
	if actor, err := group.proof(context.Background()); actor != 0 || err != nil {
		t.Fatalf("nil subject must provide no customer identity: actor=%d err=%v", actor, err)
	}
	reader.subject = &customerauthcap.Subject{CustomerNo: "customer-b", ActorUserID: 84}
	if actor, err := group.proof(context.Background()); actor != 84 || err != nil {
		t.Fatalf("proof leaked previous request identity: actor=%d err=%v", actor, err)
	}
}

func TestCustomerServerPageRoutesKeepDirectAccountRoutesSeparate(t *testing.T) {
	t.Parallel()
	for _, request := range []any{accountv1.ServerListReq{}, accountv1.ServerDetailReq{}} {
		if tag := reflect.TypeOf(request).Field(0).Tag; tag.Get("pageBridgeCapability") != "" || tag.Get("pageBridgeHost") != "" {
			t.Fatal("direct account route unexpectedly requires a page ticket")
		}
	}
	for _, request := range []any{pagev1.ServerListReq{}, pagev1.ServerDetailReq{}} {
		kind := reflect.TypeOf(request)
		tag := kind.Field(0).Tag
		if tag.Get("pageBridgeCapability") != "customer-servers.read" || tag.Get("pageBridgeHost") != "customer" || tag.Get("method") != "get" {
			t.Fatalf("unsafe page route metadata: %s", tag)
		}
		if _, found := kind.FieldByName("CustomerNo"); found {
			t.Fatal("browser can select customer owner")
		}
	}
}
