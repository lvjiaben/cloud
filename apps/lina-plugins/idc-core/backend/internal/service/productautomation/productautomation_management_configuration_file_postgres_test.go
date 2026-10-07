package productautomation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestManagementConfigurationFilePostgresRevisionInheritanceAndReadyOwnership(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	schemaName := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteAutomationIdentifier(schemaName)); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schemaName)+` CASCADE`)
	database := openAutomationDatabase(t, ctx, dsn, schemaName)
	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "077-management-configuration-files.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	policy := &capmodel.ConfigurationUploadPolicyDeclaration{ID: "document-v1", MaximumFiles: 1, MaximumFileBytes: 1024, MaximumTotalBytes: 1024, AllowedMIMETypes: []string{"application/pdf"}, AllowedExtensions: []string{".pdf"}}
	raw, _ := json.Marshal(policy)
	token := "MCF_0123456789ABCDEF0123456789ABCDEF"
	target := "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_management_configuration_upload (tenant_id,upload_no,surface_code,target_no,target_revision,uploader_user_id,field_key,policy_id,policy_snapshot,file_name,declared_size_bytes,actual_size_bytes,media_type,sha256_hex,object_ref,status_code,scan_status,expires_at,completed_at,version,created_at,updated_at) VALUES (0,?,'product',?,0,101,'document','document-v1',?,'proof.pdf',10,10,'application/pdf',?,'private/object','ready','not_required',CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, token, target, raw, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	service := &serviceImpl{database: database}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", UploadPolicies: []*capmodel.ConfigurationUploadPolicyDeclaration{policy}, Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "document", Type: capmodel.ConfigurationFieldFile, UploadPolicy: policy.ID, Label: &capmodel.LocalizedText{EnUS: "Document", ZhCN: "文件"}}}}
	values := []ConfigurationValue{{FieldKey: "document", FileTokens: []string{token}}}
	if err = database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		return service.bindManagementConfigurationFiles(txCtx, configurationSurfaceProduct, target, 0, 1, 101, schema, values)
	}); err != nil {
		t.Fatal(err)
	}
	if err = database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		return service.bindManagementConfigurationFiles(txCtx, configurationSurfaceProduct, target, 1, 2, 202, schema, values)
	}); err != nil {
		t.Fatalf("authorized colleague should inherit bound evidence: %v", err)
	}
	var referenceCount struct{ Count int }
	if err = database.GetScan(ctx, &referenceCount, `SELECT COUNT(*) AS count FROM plugin_idc_core_management_configuration_file_reference WHERE target_no=? AND upload_no=?`, target, token); err != nil || referenceCount.Count != 2 {
		t.Fatalf("expected two immutable revision references, got %d: %v", referenceCount.Count, err)
	}
	foreign := "MCF_FEDCBA9876543210FEDCBA9876543210"
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_management_configuration_upload (tenant_id,upload_no,surface_code,target_no,target_revision,uploader_user_id,field_key,policy_id,policy_snapshot,file_name,declared_size_bytes,actual_size_bytes,media_type,sha256_hex,object_ref,status_code,scan_status,expires_at,completed_at,version,created_at,updated_at) VALUES (0,?,'product',?,2,101,'document','document-v1',?,'foreign.pdf',10,10,'application/pdf',?,'private/foreign','ready','not_required',CURRENT_TIMESTAMP+INTERVAL '1 day',CURRENT_TIMESTAMP,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, foreign, target, raw, strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	err = database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		return service.bindManagementConfigurationFiles(txCtx, configurationSurfaceProduct, target, 2, 3, 202, schema, []ConfigurationValue{{FieldKey: "document", FileTokens: []string{foreign}}})
	})
	if !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("foreign ready token must fail closed, got %v", err)
	}
}

func TestManagementConfigurationFileReferencePostgresRejectsForgedEvidence(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	schemaName := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteAutomationIdentifier(schemaName)); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schemaName)+` CASCADE`)
	database := openAutomationDatabase(t, ctx, dsn, schemaName)
	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "077-management-configuration-files.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("replay management configuration file migration: %v", err)
	}

	// IDC business data is deliberately owned by the single merchant tenant 0;
	// this is the current idccap.DefaultTenantID contract, not a compatibility
	// fallback. The reference trigger must still compare tenant ownership exactly.
	boundUpload := "MCF_11111111111111111111111111111111"
	readyUpload := "MCF_22222222222222222222222222222222"
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_management_configuration_upload
    (tenant_id,upload_no,surface_code,target_no,target_revision,uploader_user_id,
     field_key,policy_id,policy_snapshot,file_name,declared_size_bytes,
     actual_size_bytes,media_type,sha256_hex,object_ref,status_code,scan_status,
     expires_at,completed_at,bound_revision,bound_at,version,created_at,updated_at)
VALUES
    (0,?,'product','ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',1,101,
     'document','document-v1','{}','proof.pdf',10,10,'application/pdf',?,
     'private/bound','bound','clean',CURRENT_TIMESTAMP+INTERVAL '1 day',
     CURRENT_TIMESTAMP,1,CURRENT_TIMESTAMP,2,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,?,'product','ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',1,101,
     'document','document-v1','{}','ready.pdf',10,10,'application/pdf',?,
     'private/ready','ready','clean',CURRENT_TIMESTAMP+INTERVAL '1 day',
     CURRENT_TIMESTAMP,NULL,NULL,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		boundUpload, strings.Repeat("a", 64), readyUpload, strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}

	insertReference := func(tenant int, surface, target, uploadNo, field, policy string, createdBy int) error {
		_, insertErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_management_configuration_file_reference
    (tenant_id,surface_code,target_no,configuration_revision,upload_no,field_key,policy_id,created_by)
VALUES (?,?,?,?,?,?,?,?)`, tenant, surface, target, 2, uploadNo, field, policy, createdBy)
		return insertErr
	}
	tests := []struct {
		name      string
		tenant    int
		surface   string
		target    string
		uploadNo  string
		field     string
		policy    string
		createdBy int
	}{
		{name: "tenant mismatch", tenant: 1, surface: "product", target: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", uploadNo: boundUpload, field: "document", policy: "document-v1", createdBy: 202},
		{name: "surface mismatch", tenant: 0, surface: "interface", target: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", uploadNo: boundUpload, field: "document", policy: "document-v1", createdBy: 202},
		{name: "target mismatch", tenant: 0, surface: "product", target: "ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", uploadNo: boundUpload, field: "document", policy: "document-v1", createdBy: 202},
		{name: "field mismatch", tenant: 0, surface: "product", target: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", uploadNo: boundUpload, field: "contract", policy: "document-v1", createdBy: 202},
		{name: "policy mismatch", tenant: 0, surface: "product", target: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", uploadNo: boundUpload, field: "document", policy: "document-v2", createdBy: 202},
		{name: "upload not bound", tenant: 0, surface: "product", target: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", uploadNo: readyUpload, field: "document", policy: "document-v1", createdBy: 202},
		{name: "missing staff actor", tenant: 0, surface: "product", target: "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", uploadNo: boundUpload, field: "document", policy: "document-v1", createdBy: 0},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if insertErr := insertReference(testCase.tenant, testCase.surface, testCase.target, testCase.uploadNo, testCase.field, testCase.policy, testCase.createdBy); insertErr == nil {
				t.Fatal("forged management configuration file reference was accepted")
			}
		})
	}
	if err = insertReference(0, "product", "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", boundUpload, "document", "document-v1", 202); err != nil {
		t.Fatalf("valid later revision must inherit immutable bound evidence: %v", err)
	}
}
