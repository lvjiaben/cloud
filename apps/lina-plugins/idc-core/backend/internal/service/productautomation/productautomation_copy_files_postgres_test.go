// This file verifies copied files retain bytes but get independent authorization
// scopes, and rejected or rolled-back copies leave no uploads or references.

package productautomation

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestProductCopyFilesPostgresScopeAndRollback(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	schemaName := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteAutomationIdentifier(schemaName)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schemaName)+` CASCADE`); dropErr != nil {
			t.Error(dropErr)
		}
	})
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
	declaration := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", UploadPolicies: []*capmodel.ConfigurationUploadPolicyDeclaration{policy}, Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "document", Type: capmodel.ConfigurationFieldFile, UploadPolicy: policy.ID, Label: &capmodel.LocalizedText{EnUS: "Document", ZhCN: "文件"}}}}
	raw, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	sourceNo := "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	targetNo := "ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	rollbackNo := "ver_cccccccccccccccccccccccccccccccc"
	token := "MCF_0123456789ABCDEF0123456789ABCDEF"
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_management_configuration_upload(tenant_id,upload_no,surface_code,target_no,target_revision,uploader_user_id,field_key,policy_id,policy_snapshot,file_name,declared_size_bytes,actual_size_bytes,media_type,sha256_hex,object_ref,status_code,scan_status,expires_at,completed_at,bound_revision,bound_at) VALUES(0,?,'product',?,1,101,'document','document-v1',?,'proof.pdf',10,10,'application/pdf',?,'private/immutable','bound','clean',CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,1,CURRENT_TIMESTAMP)`, token, sourceNo, raw, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_management_configuration_file_reference(tenant_id,surface_code,target_no,configuration_revision,upload_no,field_key,policy_id,created_by) VALUES(0,'product',?,1,?,'document','document-v1',101)`, sourceNo, token); err != nil {
		t.Fatal(err)
	}
	declarationJSON, err := json.Marshal(declaration)
	if err != nil {
		t.Fatal(err)
	}
	valueJSON := `{"document":["` + token + `"]}`
	source := &entity.CatalogProductVersionConfiguration{VersionNo: sourceNo, Version: 1, ProductDeclarationSnapshot: string(declarationJSON), MerchantValueSnapshot: valueJSON, MerchantValueDigest: configurationDigest([]byte(valueJSON))}
	service := &serviceImpl{database: database, random: rand.Reader}
	var copied, digest string
	if err = database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var copyErr error
		copied, digest, copyErr = service.copyProductConfigurationFiles(txCtx, source, targetNo, 202)
		return copyErr
	}); err != nil {
		t.Fatal(err)
	}
	var values map[string][]string
	if err = json.Unmarshal([]byte(copied), &values); err != nil {
		t.Fatal(err)
	}
	newToken := values["document"][0]
	if newToken == token || !validManagementFileToken(newToken) || digest != configurationDigest([]byte(copied)) {
		t.Fatal("copy did not regenerate token and digest")
	}
	var stored struct {
		ObjectRef, TargetNo   string
		UploaderUserID, Count int
	}
	if err = database.GetScan(ctx, &stored, `SELECT object_ref,target_no,uploader_user_id FROM plugin_idc_core_management_configuration_upload WHERE tenant_id=0 AND upload_no=?`, newToken); err != nil {
		t.Fatal(err)
	}
	if stored.ObjectRef != "private/immutable" || stored.TargetNo != targetNo || stored.UploaderUserID != 202 {
		t.Fatalf("wrong copied scope: %+v", stored)
	}
	if err = database.GetScan(ctx, &stored, `SELECT COUNT(*) AS count FROM plugin_idc_core_management_configuration_file_reference WHERE tenant_id=0 AND target_no=? AND upload_no=?`, targetNo, newToken); err != nil || stored.Count != 1 {
		t.Fatalf("missing independent reference: %+v %v", stored, err)
	}
	rollbackErr := gerror.New("injected owner failure")
	err = database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, _, copyErr := service.copyProductConfigurationFiles(txCtx, source, rollbackNo, 202); copyErr != nil {
			return copyErr
		}
		return rollbackErr
	})
	if err == nil {
		t.Fatal("injected copy failure accepted")
	}
	if err = database.GetScan(ctx, &stored, `SELECT COUNT(*) AS count FROM plugin_idc_core_management_configuration_upload WHERE target_no=?`, rollbackNo); err != nil || stored.Count != 0 {
		t.Fatalf("partial copy survived rollback: %v %+v", err, stored)
	}
	forged := *source
	forged.Version = 2
	err = database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		_, _, copyErr := service.copyProductConfigurationFiles(txCtx, &forged, rollbackNo, 202)
		return copyErr
	})
	if !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("out-of-scope revision accepted: %v", err)
	}
	if _, _, err = service.copyProductConfigurationFiles(ctx, source, rollbackNo, 202); !bizerr.Is(err, catalog.CodeManagementInvalid) {
		t.Fatalf("nontransactional copy accepted: %v", err)
	}
	if err = database.GetScan(ctx, &stored, `SELECT COUNT(*) AS count FROM plugin_idc_core_management_configuration_upload WHERE upload_no=? AND target_no=? AND uploader_user_id=101`, token, sourceNo); err != nil || stored.Count != 1 {
		t.Fatalf("source evidence changed: %v %+v", err, stored)
	}
}
