package zjmfcloud

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestQueryAccessoriesProjectsOnlySafeFields(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var reads atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/administrator/v1/login":
			login(writer, request)
		case "/administrator/v1/clouds/41":
			reads.Add(1)
			writeJSON(t, writer, http.StatusOK, `{"data":{"ip":[{"id":1,"ipaddress":"203.0.113.9","password":"must-not-leak"}],"disk":[{"id":12,"size":80,"type":"system","secret":"must-not-leak"}],"traffic_quota":1000,"tmp_traffic":20,"user_id":9,"type":"host","security":77,"node_id":6,"iso":88,"rootpassword":"must-not-leak"}}`)
		case "/administrator/v1/clouds/41/snapshots":
			writeJSON(t, writer, http.StatusOK, `{"data":{"data":[{"id":20,"type":"snap","name":"daily","status":"ready","password":"must-not-leak"},{"id":21,"type":"backup","name":"weekly"}]}}`)
		case "/administrator/v1/clouds/41/nat_acl":
			writeJSON(t, writer, http.StatusOK, `{"data":[{"id":30,"name":"ssh-alt","ext_port":10022,"int_port":22,"protocol":1,"secret":"must-not-leak"}]}`)
		case "/administrator/v1/security_groups":
			writeJSON(t, writer, http.StatusOK, `{"data":[{"id":77,"type":"host","name":"default","secret":"must-not-leak","links":[41,42],"rules":[{"address":"0.0.0.0/0"}]},{"id":78,"type":"host","name":"web","used_by":[99]}]}`)
		case "/administrator/v1/node_isos":
			writeJSON(t, writer, http.StatusOK, `{"data":[{"name":"linux","info":[{"id":88,"name":"rescue.iso","path":"must-not-leak"},{"id":89,"name":"install.iso"}]}]}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	snapshot, err := fixture.provider.QueryAccessories(context.Background(), resourcecap.AccessoryQuery{ResourceRef: "41"})
	if err != nil {
		t.Fatal(err)
	}
	if reads.Load() != 1 || len(snapshot.Items) != 10 {
		t.Fatalf("detail reads=%d items=%d", reads.Load(), len(snapshot.Items))
	}
	if err := snapshot.ValidateAt(fixture.clock.Now()); err != nil {
		t.Fatal(err)
	}
	for _, item := range snapshot.Items {
		projection := string(item.Data)
		for _, unsafe := range []string{"must-not-leak", "password", "secret", "rootpassword", "path", "links", "rules", "used_by", "0.0.0.0/0"} {
			if strings.Contains(projection, unsafe) {
				t.Fatalf("unsafe accessory projection: %+v", item)
			}
		}
	}
	assertAccessoryProjection(t, snapshot.Items, resourcecap.AccessoryKindTraffic,
		`{"quota_gb":1000,"temporary_gb":20,"total_gb":1020}`)
}

func assertAccessoryProjection(t *testing.T, items []resourcecap.AccessoryItem,
	kind resourcecap.AccessoryKind, expected string) {
	t.Helper()
	for _, item := range items {
		if item.Kind == kind {
			if string(item.Data) != expected {
				t.Fatalf("unexpected %s projection: %s", kind, item.Data)
			}
			return
		}
	}
	t.Fatalf("missing %s accessory projection", kind)
}

func TestQueryAccessoriesRejectsMalformedDetailBeforeSecondaryReads(t *testing.T) {
	t.Parallel()
	var loginCount int
	var loginMu sync.Mutex
	login := loginHandler(t, &loginCount, &loginMu, "token-one")
	var requests atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/administrator/v1/login" {
			login(writer, request)
			return
		}
		requests.Add(1)
		writeJSON(t, writer, http.StatusOK, `{"data":{"ip":[{"id":1,"ipaddress":"not-an-ip"}]}}`)
	}))
	if _, err := fixture.provider.QueryAccessories(context.Background(), resourcecap.AccessoryQuery{ResourceRef: "41"}); err == nil {
		t.Fatal("malformed accessory detail was accepted")
	}
	if requests.Load() != 1 {
		t.Fatalf("network requests=%d, want one detail read", requests.Load())
	}
}

func TestQueryAccessoriesRejectsInvalidResourceBeforeNetwork(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	fixture := newProviderFixture(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	if _, err := fixture.provider.QueryAccessories(context.Background(), resourcecap.AccessoryQuery{ResourceRef: "cloud-41"}); err == nil {
		t.Fatal("invalid resource was accepted")
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid resource made %d network requests", requests.Load())
	}
}
