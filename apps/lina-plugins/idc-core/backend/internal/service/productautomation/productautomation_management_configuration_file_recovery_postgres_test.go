package productautomation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/storagecap"
)

type managementRecoveryStorage struct {
	storagecap.Service
	objects map[string][]byte
}

func (s *managementRecoveryStorage) Get(_ context.Context, input storagecap.GetInput) (*storagecap.GetOutput, error) {
	value, ok := s.objects[input.Path]
	if !ok {
		return &storagecap.GetOutput{Found: false}, nil
	}
	return &storagecap.GetOutput{Found: true, Object: &storagecap.Object{Path: input.Path, Size: int64(len(value)), ContentType: "application/pdf", Visibility: storagecap.VisibilityPrivate}, Body: io.NopCloser(bytes.NewReader(value))}, nil
}
func (s *managementRecoveryStorage) Delete(_ context.Context, input storagecap.DeleteInput) error {
	delete(s.objects, input.Path)
	return nil
}

func TestManagementConfigurationFilePostgresRecoversUnknownStorageOutcome(t *testing.T) {
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
	content := []byte("%PDF governed evidence")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	storage := &managementRecoveryStorage{objects: map[string][]byte{"management-configuration/one": content, "management-configuration/two": content}}
	service := &serviceImpl{database: database, storage: storage}
	insert := func(token, path string) {
		t.Helper()
		_, insertErr := database.Exec(ctx, `INSERT INTO plugin_idc_core_management_configuration_upload (tenant_id,upload_no,surface_code,target_no,target_revision,uploader_user_id,field_key,policy_id,policy_snapshot,file_name,declared_size_bytes,actual_size_bytes,media_type,sha256_hex,object_ref,status_code,scan_status,expires_at,version,created_at,updated_at) VALUES (0,?,'product','ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',0,101,'document','document-v1','{}','proof.pdf',?,?, 'application/pdf',?,?,'uploading','not_required',CURRENT_TIMESTAMP+INTERVAL '1 hour',2,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, token, len(content), len(content), digest, path)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
	}
	insert("MCF_11111111111111111111111111111111", "management-configuration/one")
	row := &managementFileRow{UploadNo: "MCF_11111111111111111111111111111111", FileName: "proof.pdf", MediaType: "application/pdf", ActualSize: int64(len(content)), SHA256Hex: digest, ObjectRef: "management-configuration/one", Status: "uploading", ScanStatus: "not_required", UploaderUserID: 101, Version: 2}
	item, found, err := service.reconcileManagementFileObject(ctx, row)
	if err != nil || !found || item.Status != "ready" {
		t.Fatalf("stored unknown outcome must converge ready: %#v %v", item, err)
	}
	insert("MCF_22222222222222222222222222222222", "management-configuration/two")
	if _, err = database.Exec(ctx, `CREATE FUNCTION reject_ready_once() RETURNS TRIGGER AS $$ BEGIN IF NEW.status_code='ready' THEN RAISE EXCEPTION 'injected finalization failure'; END IF; RETURN NEW; END; $$ LANGUAGE plpgsql; CREATE TRIGGER reject_ready_once BEFORE UPDATE ON plugin_idc_core_management_configuration_upload FOR EACH ROW EXECUTE FUNCTION reject_ready_once();`); err != nil {
		t.Fatal(err)
	}
	row.UploadNo = "MCF_22222222222222222222222222222222"
	row.ObjectRef = "management-configuration/two"
	if _, _, err = service.reconcileManagementFileObject(ctx, row); err == nil {
		t.Fatal("expected injected database finalization failure")
	}
	if _, err = database.Exec(ctx, `DROP TRIGGER reject_ready_once ON plugin_idc_core_management_configuration_upload`); err != nil {
		t.Fatal(err)
	}
	item, found, err = service.reconcileManagementFileObject(ctx, row)
	if err != nil || !found || item.ExpiresAt.Before(time.Now()) {
		t.Fatalf("restart recovery must finish the same object: %#v %v", item, err)
	}
}
