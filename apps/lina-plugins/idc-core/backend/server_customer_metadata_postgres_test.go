package backend

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

type serverOperationAuditStub struct {
	transitions []idccap.BusinessAuditTransition
	failure     error
}

func (s *serverOperationAuditStub) AppendBusinessTransition(_ context.Context, input idccap.BusinessAuditTransition) error {
	if s.failure != nil {
		return s.failure
	}
	s.transitions = append(s.transitions, input)
	return nil
}

func (*serverOperationAuditStub) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, errors.New("unexpected batch server operation audit")
}

func TestServerCustomerMetadataPostgreSQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL server metadata tests", catalogIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newBillingIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_billing_test_") {
			_, _ = adminDB.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE")
		}
		_ = adminDB.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(
		dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, catalogDictionaryFixtureSQL); err != nil {
		t.Fatal(err)
	}
	for _, path := range append(billingMigrationPaths(t), serviceRenewalMigrationPaths(t)...) {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", filepath.Base(path), readErr)
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute %s: %v", filepath.Base(path), err)
		}
	}
	seedCommerceFoundation(t, ctx, database)
	seedServiceRenewalFoundation(t, ctx, database)

	audit := &serverOperationAuditStub{}
	service, err := serveroperation.NewService(database, audit)
	if err != nil {
		t.Fatal(err)
	}
	version, err := database.GetValue(ctx, `SELECT version FROM plugin_idc_core_service_instance
WHERE service_no='service-commerce-1'`)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.UpdateCustomerMetadata(ctx, serveroperation.CustomerMetadataCommand{
		CustomerNo: "cus_commerce_test", ServiceNo: "service-commerce-1",
		Alias: "  Production database  ", Remark: "  Reporting workload  ",
		ExpectedVersion: version.Int64(),
	})
	if err != nil || updated.Alias != "Production database" ||
		updated.Remark != "Reporting workload" || updated.Version != version.Int64()+1 {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if len(audit.transitions) != 1 || audit.transitions[0].CustomerNo != "cus_commerce_test" {
		t.Fatalf("audit=%+v", audit.transitions)
	}
	replayed, err := service.UpdateCustomerMetadata(ctx, serveroperation.CustomerMetadataCommand{
		CustomerNo: "cus_commerce_test", ServiceNo: "service-commerce-1",
		Alias: "Production database", Remark: "Reporting workload",
		ExpectedVersion: version.Int64(),
	})
	if err != nil || replayed.Version != updated.Version || len(audit.transitions) != 1 {
		t.Fatalf("safe replay=%+v audit=%d err=%v", replayed, len(audit.transitions), err)
	}
	if _, err = service.UpdateCustomerMetadata(ctx, serveroperation.CustomerMetadataCommand{
		CustomerNo: "another-customer", ServiceNo: "service-commerce-1",
		ExpectedVersion: updated.Version,
	}); !errors.Is(err, serveroperation.ErrServiceUnavailable) {
		t.Fatalf("cross-customer update returned %v", err)
	}
	if _, err = service.UpdateCustomerMetadata(ctx, serveroperation.CustomerMetadataCommand{
		CustomerNo: "cus_commerce_test", ServiceNo: "service-commerce-1",
		ExpectedVersion: version.Int64(),
	}); !errors.Is(err, serveroperation.ErrOperationConflict) {
		t.Fatalf("stale update returned %v", err)
	}

	audit.failure = errors.New("audit unavailable")
	if _, err = service.UpdateCustomerMetadata(ctx, serveroperation.CustomerMetadataCommand{
		CustomerNo: "cus_commerce_test", ServiceNo: "service-commerce-1",
		Alias: "Should roll back", Remark: "", ExpectedVersion: updated.Version,
	}); err == nil {
		t.Fatal("audit failure should roll back customer metadata")
	}
	var persisted struct {
		Alias   string `orm:"customer_alias"`
		Version int64  `orm:"version"`
	}
	if err = database.GetScan(ctx, &persisted, `SELECT customer_alias,version
FROM plugin_idc_core_service_instance WHERE service_no='service-commerce-1'`); err != nil {
		t.Fatal(err)
	}
	if persisted.Alias != updated.Alias || persisted.Version != updated.Version {
		t.Fatalf("audit rollback failed: %+v", persisted)
	}
}
