// This file proves execution rechecks authoritative customer identity after a
// page grant, preserving revocation, account rebinding and request context.

package serverprojection

import (
	"context"
	"errors"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serverdirectory"
)

type mutableCustomerReader struct {
	subject  *customerauthcap.Subject
	err      error
	contexts []context.Context
}

func (reader *mutableCustomerReader) CurrentCustomer(ctx context.Context) (*customerauthcap.Subject, error) {
	reader.contexts = append(reader.contexts, ctx)
	return reader.subject, reader.err
}

type ownedDirectory struct {
	calls int
	item  serverdirectory.Item
}

func (directory *ownedDirectory) List(ctx context.Context, request serverdirectory.ListRequest) (*serverdirectory.Page, error) {
	directory.calls++
	if request.Scope.CustomerNo != "customer-a" {
		return &serverdirectory.Page{Items: []serverdirectory.Item{}}, nil
	}
	return &serverdirectory.Page{Items: []serverdirectory.Item{directory.item}}, nil
}
func (directory *ownedDirectory) Detail(ctx context.Context, request serverdirectory.DetailRequest) (*serverdirectory.Item, error) {
	directory.calls++
	if request.Scope.CustomerNo != "customer-a" || request.ServerNo != directory.item.ServerNo {
		return nil, serverdirectory.ErrNotFound
	}
	return &directory.item, nil
}

func TestServerProjectionRechecksIdentityAfterPageGrant(t *testing.T) {
	t.Parallel()
	var (
		reader    = &mutableCustomerReader{subject: &customerauthcap.Subject{CustomerNo: "customer-a", ActorUserID: 42}}
		directory = &ownedDirectory{item: serverdirectory.Item{ServerNo: "SVR-0123456789ABCDEF0123456789ABCDEF", ProductNameZhCN: "测试云", ProductNameEnUS: "Test cloud", ChannelName: "Channel", PoolName: "Pool", Kind: serverdirectory.ServerKindCloud, State: serverdirectory.ResourceStateReady, LatestOperation: serverdirectory.OperationStateIdle, BoundAt: time.Unix(1700000000, 0), UpdatedAt: time.Unix(1700000000, 0)}}
		list      = resourcecap.CurrentCustomerServerListRequest{Limit: 20}
		detail    = resourcecap.CurrentCustomerServerGetRequest{ServerNo: directory.item.ServerNo}
	)
	service, err := New(reader, directory)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if page, err := service.ListCurrentCustomerServers(ctx, list); err != nil || len(page.Items) != 1 {
		t.Fatalf("owned list=%+v err=%v", page, err)
	}
	if result, err := service.GetCurrentCustomerServer(ctx, detail); err != nil || result.Item.ServerNo != detail.ServerNo {
		t.Fatalf("owned detail=%+v err=%v", result, err)
	}
	if reader.contexts[0] != ctx || reader.contexts[1] != ctx {
		t.Fatal("trusted execution context replaced")
	}
	for _, reason := range []string{"anonymous", "customer_disabled", "binding_revoked", "assistance_expired", "assistance_revoked"} {
		t.Run(reason, func(t *testing.T) {
			before := directory.calls
			reader.subject = nil
			reader.err = errors.New(reason)
			if result, err := service.ListCurrentCustomerServers(ctx, list); result != nil || !errors.Is(err, reader.err) {
				t.Fatalf("rejected identity listed: %+v %v", result, err)
			}
			if result, err := service.GetCurrentCustomerServer(ctx, detail); result != nil || !errors.Is(err, reader.err) {
				t.Fatalf("rejected identity read detail: %+v %v", result, err)
			}
			if directory.calls != before {
				t.Fatal("rejected identity queried resource existence")
			}
		})
	}
	reader.subject = &customerauthcap.Subject{CustomerNo: "customer-b", ActorUserID: 84}
	reader.err = nil
	if page, err := service.ListCurrentCustomerServers(ctx, list); err != nil || len(page.Items) != 0 {
		t.Fatalf("cross-customer list=%+v %v", page, err)
	}
	_, foreign := service.GetCurrentCustomerServer(ctx, detail)
	_, missing := service.GetCurrentCustomerServer(ctx, resourcecap.CurrentCustomerServerGetRequest{ServerNo: "SVR-FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"})
	if !errors.Is(foreign, serverdirectory.ErrNotFound) || !errors.Is(missing, serverdirectory.ErrNotFound) {
		t.Fatalf("foreign/missing existence differs: %v / %v", foreign, missing)
	}
}
