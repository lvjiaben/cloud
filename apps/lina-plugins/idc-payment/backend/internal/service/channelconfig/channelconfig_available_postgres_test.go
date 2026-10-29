// These tests use a disposable schema to prove empty currency results avoid
// discovery, while real channel results retain current provider authorization.
package channelconfig

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

type availableProviderDirectory struct {
	calls   int
	failure error
	items   []*capmodel.BusinessExtensionProviderEvidence
}

func (r *availableProviderDirectory) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	r.calls++
	if r.failure != nil {
		return nil, r.failure
	}
	return &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{Items: r.items, Total: len(r.items)}, nil
}

type availableAdapter struct{ paymentcap.Adapter }

func TestEmptyAvailableChannelsSkipDiscoveryPostgreSQL(t *testing.T) {
	database := availableChannelDatabase(t)
	ctx := context.Background()
	if _, err := database.Exec(ctx, `CREATE TABLE plugin_idc_payment_channel(channel_no TEXT,provider_code TEXT,display_name TEXT,supported_modes JSONB,supported_currencies JSONB,status_code TEXT,tenant_id INT,deleted_at TIMESTAMPTZ)`); err != nil {
		t.Fatal(err)
	}
	directory := &availableProviderDirectory{failure: errors.New("discovery unavailable")}
	service := &Service{database: database, extensions: directory, adapters: map[paymentcap.ProviderCode]paymentcap.Adapter{paymentcap.ProviderEPay: &availableAdapter{}}}
	empty, err := service.ListAvailable(ctx, "CNY")
	if err != nil || len(empty) != 0 || directory.calls != 0 {
		t.Fatalf("empty discovered providers: %+v %v calls=%d", empty, err, directory.calls)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_payment_channel VALUES('usd','epay','USD','["epay_cashier"]','["USD"]','enabled',0,NULL),('disabled','epay','Disabled','["epay_cashier"]','["CNY"]','disabled',0,NULL)`); err != nil {
		t.Fatal(err)
	}
	if empty, err = service.ListAvailable(ctx, "CNY"); err != nil || len(empty) != 0 || directory.calls != 0 {
		t.Fatalf("other currency triggered discovery: %+v %v calls=%d", empty, err, directory.calls)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_payment_channel VALUES('cny','epay','CNY','["epay_cashier"]','["CNY"]','enabled',0,NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ListAvailable(ctx, "CNY"); err == nil || directory.calls != 1 {
		t.Fatal("nonempty discovery failure was hidden", err, directory.calls)
	}
	directory.failure = nil
	if empty, err = service.ListAvailable(ctx, "CNY"); err != nil || len(empty) != 0 || directory.calls != 2 {
		t.Fatal("disabled payment provider remained available", err)
	}
	directory.items = []*capmodel.BusinessExtensionProviderEvidence{{PluginID: "gateway-epay", ReleaseVersion: "v0.1.0", InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{}, ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Type: paymentcap.DynamicProviderMetadataType, Version: paymentcap.DynamicProviderContractVersion, Values: map[string]any{"providerCode": "epay", "endpointOrigin": "https://example.test/submit.php", "modes": []any{"epay_cashier"}, "currencies": []any{"CNY", "USD"}}}}}
	found, err := service.ListAvailable(ctx, "CNY")
	if err != nil || len(found) != 1 || found[0].ChannelNo != "cny" || directory.calls != 3 {
		t.Fatalf("enabled result changed: %+v %v calls=%d", found, err, directory.calls)
	}
	all, err := service.ListAvailable(ctx, "")
	if err != nil || len(all) != 2 || directory.calls != 4 {
		t.Fatalf("all currency results changed: %+v %v", all, err)
	}
}

func availableChannelDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "payment_available_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); cleanupErr != nil {
			t.Error(cleanupErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	return database
}
