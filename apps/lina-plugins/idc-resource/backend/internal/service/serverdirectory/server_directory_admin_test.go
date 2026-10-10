// This file verifies staff authorization precedes bounded fixed-provider reads.
package serverdirectory

import (
	"context"
	"errors"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"reflect"
	"strings"
	"testing"
	"time"
)

type staffDirectoryAuthorizer struct {
	denied bool
	calls  int
}

func (a *staffDirectoryAuthorizer) Authorize(_ context.Context, request managementcap.AuthorizationRequest) (int, error) {
	a.calls++
	if a.denied || request.Permission != cloudAdminViewPermission {
		return 0, errors.New("denied")
	}
	return 7, nil
}

type staffDirectoryScanner struct {
	calls  int
	absent bool
}

func (s *staffDirectoryScanner) GetScan(_ context.Context, target any, query string, args ...any) error {
	s.calls++
	if !strings.Contains(query, "ownership.tenant_id=0") || !strings.Contains(query, "channel.provider_plugin_id=?") || len(args) < 1 || args[0] != cloudConnectorPluginID {
		return errors.New("missing fixed owner/provider scope")
	}
	value := reflect.ValueOf(target).Elem()
	if value.Kind() == reflect.Struct {
		value.FieldByName("Total").SetInt(1)
		return nil
	}
	if s.absent {
		return nil
	}
	rows := target.(*[]directoryRow)
	*rows = []directoryRow{{ServerNo: "SVR-" + strings.Repeat("A", 32), ActionReference: "srv_" + strings.Repeat("a", 32), ProductNameZhCN: "云服务器", ProductNameEnUS: "Cloud server", ChannelName: "授权接口", PoolName: "资源池", ResourceKind: "virtual_server", MappingStatus: "active", Capabilities: "[]", BoundAt: time.Unix(1, 0), UpdatedAt: time.Unix(2, 0)}}
	return nil
}
func TestAdminDirectoryAuthorizesBeforeFixedProviderQueries(t *testing.T) {
	for _, denied := range []bool{true, false} {
		t.Run(map[bool]string{true: "denied", false: "allowed"}[denied], func(t *testing.T) {
			scanner := &staffDirectoryScanner{}
			directory, _ := newWithScanner(scanner)
			auth := &staffDirectoryAuthorizer{denied: denied}
			service, _ := NewAdmin(directory, auth)
			page, err := service.List(context.Background(), 1, 20)
			if denied {
				if err == nil || scanner.calls != 0 {
					t.Fatal("denied staff reached storage")
				}
				return
			}
			if err != nil || len(page.Items) != 1 || page.Total != 1 || scanner.calls != 2 {
				t.Fatalf("invalid bounded page: %#v %v calls=%d", page, err, scanner.calls)
			}
			detail, err := service.Detail(context.Background(), page.Items[0].ServerNo)
			if err != nil || detail.ServerNo != page.Items[0].ServerNo {
				t.Fatal(err)
			}
			scanner.absent = true
			if _, err = service.Detail(context.Background(), page.Items[0].ServerNo); err == nil {
				t.Fatal("foreign or missing provider server accepted")
			}
			before := scanner.calls
			for _, input := range [][2]int{{0, 100}, {1, 101}, {1000001, 20}} {
				if _, err = service.List(context.Background(), input[0], input[1]); err == nil || scanner.calls != before {
					t.Fatal("unbounded request reached database")
				}
			}
		})
	}
}
