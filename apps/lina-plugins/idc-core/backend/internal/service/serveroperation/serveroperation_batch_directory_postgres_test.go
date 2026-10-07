// This file isolates the staff server directory SQL and authorization boundary.
// It does not create billable services, execute operations, or contact providers.

package serveroperation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type batchDirectoryStaff struct{ current bizctxcap.CurrentContext }

func (staff batchDirectoryStaff) Current(context.Context) bizctxcap.CurrentContext {
	return staff.current
}

type batchDirectoryPrincipal struct{}

func (batchDirectoryPrincipal) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{UserID: 42, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, nil
}

type batchDirectoryAuthorization struct {
	authz.Service
	allowed bool
}

func (authorization batchDirectoryAuthorization) BatchHasPermissions(context.Context, []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	return map[authz.PermissionKey]bool{authz.PermissionKey(PermissionServerBatchView): authorization.allowed}, nil
}
func (batchDirectoryAuthorization) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return false, nil
}

func TestStaffServerNumericPagesPostgreSQL(t *testing.T) {
	ctx, database := newBatchDirectoryDatabase(t)
	for _, statement := range []string{
		`CREATE TABLE plugin_idc_core_catalog_product (tenant_id int NOT NULL,product_no text NOT NULL,name_zh_cn text,name_en_us text,PRIMARY KEY(tenant_id,product_no))`,
		`CREATE TABLE plugin_idc_core_service_instance (tenant_id int NOT NULL,service_no text NOT NULL,customer_no text NOT NULL,product_no text NOT NULL,entitlement_status_code text,runtime_status_code text,provider_task_status_code text,entitlement_expires_at timestamptz,PRIMARY KEY(tenant_id,service_no))`,
		`INSERT INTO plugin_idc_core_catalog_product VALUES(0,'prd_directory','测试服务器','Directory server'),(1,'prd_directory','另一个租户','Other tenant')`,
		`INSERT INTO plugin_idc_core_service_instance SELECT 0,'SVC_directory_'||lpad(n::text,4,'0'),CASE WHEN n<=101 THEN 'CUS_directory_A' ELSE 'CUS_directory_B' END,'prd_directory',CASE WHEN n<=101 THEN 'active' ELSE 'terminated' END,'running','none',NULL FROM generate_series(1,121) n`,
		`INSERT INTO plugin_idc_core_service_instance VALUES(1,'SVC_private','CUS_other','prd_directory','active','running','none',NULL)`,
	} {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	manager := &managementServiceImpl{bizCtx: batchDirectoryStaff{bizctxcap.CurrentContext{UserID: 42, DataScope: 1}},
		principal: batchDirectoryPrincipal{}, authorization: batchDirectoryAuthorization{allowed: true}, directory: &directoryImpl{database: database}}
	service := &batchManagementImpl{management: manager}
	for _, size := range []int{10, 20, 50, 100} {
		page, err := service.ListServices(ctx, BatchServiceListInput{Page: 2, Limit: size})
		if err != nil || page.Total != 121 || len(page.Items) != min(size, 121-size) || page.Items[0].ServiceNo != fmt.Sprintf("SVC_directory_%04d", 121-size) {
			t.Fatalf("size %d: %+v / %v", size, page, err)
		}
	}
	filtered, err := service.ListServices(ctx, BatchServiceListInput{Page: 3, Limit: 10, Keyword: "CUS_directory_A", EntitlementStatus: "active", RuntimeStatus: "running"})
	if err != nil || filtered.Total != 101 || len(filtered.Items) != 10 || filtered.Items[0].ServiceNo != "SVC_directory_0081" {
		t.Fatalf("filtered %+v / %v", filtered, err)
	}
	empty, err := service.ListServices(ctx, BatchServiceListInput{Page: 99, Limit: 10})
	if err != nil || empty.Total != 121 || len(empty.Items) != 0 {
		t.Fatalf("tail %+v / %v", empty, err)
	}
	for _, input := range []BatchServiceListInput{{Page: -1}, {Page: 1000001}, {Page: 2, Cursor: "SVC_directory_0001"}, {Page: 1, Limit: 101}} {
		if page, failure := service.ListServices(ctx, input); failure == nil || page != nil {
			t.Fatalf("accepted invalid page %+v", input)
		}
	}
	manager.authorization = batchDirectoryAuthorization{allowed: false}
	if page, denial := service.ListServices(ctx, BatchServiceListInput{Page: 1, Limit: 10}); denial == nil || page != nil {
		t.Fatal("permission denial leaked directory")
	}
	manager.authorization = batchDirectoryAuthorization{allowed: true}
	manager.bizCtx = batchDirectoryStaff{bizctxcap.CurrentContext{UserID: 42, DataScope: 3}}
	if page, denial := service.ListServices(ctx, BatchServiceListInput{Page: 1, Limit: 10}); denial == nil || page != nil {
		t.Fatal("unsupported data scope leaked directory")
	}
}

// newBatchDirectoryDatabase creates only the two tables read by this directory.
func newBatchDirectoryDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("LINAPRO_IDC_CORE_TEST_DSN is required")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_batch_directory_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	var database gdb.DB
	t.Cleanup(func() {
		if database != nil {
			if closeErr := database.Close(ctx); closeErr != nil {
				t.Error(closeErr)
			}
		}
		if !strings.HasPrefix(schema, "linapro_batch_directory_") {
			t.Error("unexpected schema")
			return
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Error(dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err = gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	return ctx, database
}
