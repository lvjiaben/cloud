// Isolated PostgreSQL tests verify content visibility, lifecycle, file fences, and numbered administration.

package contenthub

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"

	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const contentHubIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"
const contentHubIntegrationDatabase = "linapro_idc_codegen"

func TestDeleteCategoryMigratesPublishedContentWithoutChangingVisibility(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_category (tenant_id,category_no,content_type,name,sort_order,status_code,version,created_by,updated_by) VALUES (0,'cat_news','news','News',1,'active',1,7,7)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item (tenant_id,content_no,content_type,category_no,visibility_code,title,body,public_path,status_code,validity_version,version,created_by,updated_by,published_at) VALUES (0,'cnt_news','news','cat_news','all_logged_in','Title','Body','/pages/news/cnt_news','published',4,3,7,7,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteCategory(ctx, CategoryDeleteInput{CategoryNo: "cat_news", Version: 1, Confirmed: true}); err != nil {
		t.Fatalf("delete category: %v", err)
	}
	var row struct {
		CategoryNo               *string `orm:"category_no"`
		Status, Visibility       string
		ValidityVersion, Version int64
	}
	if err := database.GetScan(ctx, &row, `SELECT category_no,status_code AS status,visibility_code AS visibility,validity_version,version FROM plugin_idc_core_content_item WHERE content_no='cnt_news'`); err != nil {
		t.Fatal(err)
	}
	if row.CategoryNo != nil || row.Status != "published" || row.Visibility != "all_logged_in" || row.ValidityVersion != 4 || row.Version != 4 {
		t.Fatalf("category migration changed unrelated facts: %+v", row)
	}
	categories, err := database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_core_content_category WHERE category_no='cat_news'`)
	if err != nil || categories.Int() != 0 {
		t.Fatalf("category count=%d err=%v", categories.Int(), err)
	}
}

func TestSearchCategoriesUsesRuntimeTranslationForUncategorized(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	page, err := service.SearchCategories(ctx, CategorySearchInput{Type: "news", Locale: "zh-CN", Limit: 10})
	if err != nil {
		t.Fatalf("search categories: %v", err)
	}
	if len(page.Items) != 1 || !page.Items[0].Synthetic || page.Items[0].Name != "translated:dict.idc_content_category_group.uncategorized.label" {
		t.Fatalf("unexpected synthetic category: %+v", page.Items)
	}
}

func TestCreateDownloadValidatesCategoryAudienceAndStartsUploadPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	service.storage = &contentHubStorageStub{}
	for _, statement := range []string{
		`INSERT INTO plugin_idc_core_content_category (tenant_id,category_no,content_type,name,sort_order,status_code,version,created_by,updated_by) VALUES (0,'cat_download','download','Downloads',1,'active',1,7,7)`,
		`INSERT INTO plugin_idc_core_customer_group (tenant_id,group_no,name_zh_cn,name_en_us,status_code) VALUES (0,'grp_download','下载客户','Download customers','active')`,
	} {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	item, err := service.Create(ctx, WriteInput{
		Type: "download", CategoryNo: "cat_download", Visibility: "customer_group",
		Title: "Guide", Body: "Guide body", TargetCustomerGroupNos: []string{"grp_download"},
	})
	if err != nil {
		t.Fatalf("create targeted download: %v", err)
	}
	if item.CategoryNo != "cat_download" || len(item.TargetCustomerGroupNos) != 1 || item.TargetCustomerGroupNos[0] != "grp_download" {
		t.Fatalf("created download projection=%+v", item)
	}
	upload, err := service.CreateUpload(ctx, UploadCreateInput{
		ContentNo: item.ContentNo, FileName: "guide.pdf", SizeBytes: 10, ContentVersion: item.Version,
	})
	if err != nil {
		t.Fatalf("start download upload: %v", err)
	}
	if upload.UploadNo == "" || upload.FileName != "guide.pdf" || upload.SizeBytes != 10 || upload.Version != 1 {
		t.Fatalf("upload session=%+v", upload)
	}
}

func TestUnpublishInvalidatesExistingPublicDownloadTicketPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item (tenant_id,content_no,content_type,visibility_code,title,body,public_path,status_code,validity_version,current_file_no,version,created_by,updated_by,published_at) VALUES (0,'cnt_download','download','public','Download','Body','/pages/downloads/cnt_download','published',3,'file_download',7,7,7,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_upload_session (tenant_id,upload_no,content_no,uploader_user_id,file_name,declared_size_bytes,persisted_size_bytes,temporary_object_ref,transfer_mode,chunk_size_bytes,status_code,completed_at,expires_at) VALUES (0,'upload_download','cnt_download',7,'guide.pdf',10,10,'tmp/guide','chunk_objects',5242880,'completed',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 hour')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_file (tenant_id,file_no,content_no,upload_no,candidate_generation,role_code,status_code,file_name,media_type,content_kind,size_bytes,sha256_hex,object_ref,scan_required,scan_status,gate_completed_at,enabled_at,created_by) VALUES (0,'file_download','cnt_download','upload_download',1,'current','active','guide.pdf','application/pdf','pdf',10,repeat('a',64),'downloads/guide.pdf',FALSE,'not_configured',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,7)`); err != nil {
		t.Fatal(err)
	}
	ticket, err := service.IssuePublicDownload(ctx, "cnt_download")
	if err != nil || ticket == nil || ticket.Token == "" {
		t.Fatalf("issue public download ticket: ticket=%+v err=%v", ticket, err)
	}
	if _, err = service.Unpublish(ctx, "cnt_download", 7); err != nil {
		t.Fatalf("unpublish download content: %v", err)
	}
	if _, err = service.OpenPublicDownload(ctx, ticket.Token); !bizerr.Is(err, CodeNotFound) {
		t.Fatalf("old public download ticket remained usable after unpublish: %v", err)
	}
	var row struct {
		Status          string `orm:"status_code"`
		ValidityVersion int64  `orm:"validity_version"`
		Version         int64  `orm:"version"`
	}
	if err = database.GetScan(ctx, &row, `SELECT status_code,validity_version,version FROM plugin_idc_core_content_item WHERE content_no='cnt_download'`); err != nil {
		t.Fatal(err)
	}
	if row.Status != "offline" || row.ValidityVersion != 4 || row.Version != 8 {
		t.Fatalf("unexpected content fences after unpublish: %+v", row)
	}
}

func TestOpenDownloadStopsAtNextControlledChunkAfterUnpublishPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	seedPublishedDownload(t, ctx, database, "cnt_stream", "file_stream", "upload_stream", 3, 7)
	ticket, err := service.IssuePublicDownload(ctx, "cnt_stream")
	if err != nil {
		t.Fatalf("issue public download ticket: %v", err)
	}
	digest := sha256.Sum256([]byte(ticket.Token))
	body := &controlledDownloadBody{
		body:      io.NopCloser(bytes.NewReader(make([]byte, 2*downloadAuthorizationChunkBytes))),
		remaining: downloadAuthorizationChunkBytes,
		revalidate: func() error {
			return service.revalidateDownload(ctx, hex.EncodeToString(digest[:]), 0, "")
		},
	}
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close controlled download: %v", closeErr)
		}
	})
	first := make([]byte, downloadAuthorizationChunkBytes)
	if _, err = io.ReadFull(body, first); err != nil {
		t.Fatalf("read first controlled chunk: %v", err)
	}
	if _, err = service.Unpublish(ctx, "cnt_stream", 7); err != nil {
		t.Fatalf("unpublish open download: %v", err)
	}
	if _, err = body.Read(make([]byte, 1)); !bizerr.Is(err, CodeNotFound) {
		t.Fatalf("open stream continued past revocation boundary: %v", err)
	}
}

func TestOrdinaryTicketExpiryRejectsNewRequestsWithoutInterruptingOpenDownloadPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	seedPublishedDownload(t, ctx, database, "cnt_expiring_stream", "file_expiring_stream", "upload_expiring_stream", 3, 7)
	ticket, err := service.IssuePublicDownload(ctx, "cnt_expiring_stream")
	if err != nil {
		t.Fatalf("issue public download ticket: %v", err)
	}
	digest := sha256.Sum256([]byte(ticket.Token))
	body := &controlledDownloadBody{
		body:      io.NopCloser(bytes.NewReader(make([]byte, 2*downloadAuthorizationChunkBytes))),
		remaining: downloadAuthorizationChunkBytes,
		revalidate: func() error {
			return service.revalidateDownload(ctx, hex.EncodeToString(digest[:]), 0, "")
		},
	}
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close controlled download: %v", closeErr)
		}
	})
	first := make([]byte, downloadAuthorizationChunkBytes)
	if _, err = io.ReadFull(body, first); err != nil {
		t.Fatalf("read first controlled chunk: %v", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_content_download_ticket SET issued_at=CURRENT_TIMESTAMP-INTERVAL '11 minutes',expires_at=CURRENT_TIMESTAMP-INTERVAL '1 minute' WHERE ticket_no=?`, ticketNoByDigest(t, ctx, database, digest)); err != nil {
		t.Fatalf("expire public download ticket: %v", err)
	}
	if _, err = service.OpenPublicDownload(ctx, ticket.Token); !bizerr.Is(err, CodeNotFound) {
		t.Fatalf("expired ticket started a new request: %v", err)
	}
	if read, readErr := body.Read(make([]byte, 1)); readErr != nil || read != 1 {
		t.Fatalf("ordinary expiry interrupted an already-authorized connection: read=%d err=%v", read, readErr)
	}
}

func TestEnableCandidateStopsOpenOldFileAtNextControlledChunkPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	seedPublishedDownload(t, ctx, database, "cnt_replace_stream", "file_replace_old", "upload_replace_old", 3, 7)
	for _, statement := range []string{
		`INSERT INTO plugin_idc_core_content_upload_session
         (tenant_id,upload_no,content_no,uploader_user_id,file_name,declared_size_bytes,
          persisted_size_bytes,temporary_object_ref,transfer_mode,chunk_size_bytes,status_code,completed_at,expires_at)
         VALUES (0,'upload_replace_new','cnt_replace_stream',7,'new.pdf',10,10,
                 'downloads/file_replace_new','chunk_objects',5242880,'completed',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 day')`,
		`INSERT INTO plugin_idc_core_content_file
         (tenant_id,file_no,content_no,upload_no,candidate_generation,role_code,status_code,
          file_name,media_type,content_kind,size_bytes,sha256_hex,object_ref,scan_required,scan_status,
          gate_completed_at,retention_expires_at,version,created_by)
         VALUES (0,'file_replace_new','cnt_replace_stream','upload_replace_new',2,'candidate','ready',
                 'new.pdf','application/pdf','pdf',10,repeat('b',64),'downloads/file_replace_new',FALSE,'passed',
                 CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '7 days',1,7)`,
	} {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	ticket, err := service.IssuePublicDownload(ctx, "cnt_replace_stream")
	if err != nil {
		t.Fatalf("issue old-file download ticket: %v", err)
	}
	digest := sha256.Sum256([]byte(ticket.Token))
	body := &controlledDownloadBody{
		body:      io.NopCloser(bytes.NewReader(make([]byte, 2*downloadAuthorizationChunkBytes))),
		remaining: downloadAuthorizationChunkBytes,
		revalidate: func() error {
			return service.revalidateDownload(ctx, hex.EncodeToString(digest[:]), 0, "")
		},
	}
	t.Cleanup(func() {
		if closeErr := body.Close(); closeErr != nil {
			t.Errorf("close controlled download: %v", closeErr)
		}
	})
	if _, err = io.CopyN(io.Discard, body, downloadAuthorizationChunkBytes); err != nil {
		t.Fatalf("read first controlled chunk: %v", err)
	}
	if _, err = service.EnableCandidate(ctx, CandidateActionInput{
		ContentNo: "cnt_replace_stream", ContentVersion: 7,
		FileNo: "file_replace_new", FileVersion: 1, Confirmed: true,
	}); err != nil {
		t.Fatalf("enable replacement candidate: %v", err)
	}
	if _, err = body.Read(make([]byte, 1)); !bizerr.Is(err, CodeNotFound) {
		t.Fatalf("old-file stream continued after replacement fence: %v", err)
	}
	var state struct {
		CurrentFileNo   string `orm:"current_file_no"`
		ValidityVersion int64  `orm:"validity_version"`
	}
	if err = database.GetScan(ctx, &state, `SELECT current_file_no,validity_version FROM plugin_idc_core_content_item WHERE content_no='cnt_replace_stream'`); err != nil {
		t.Fatal(err)
	}
	if state.CurrentFileNo != "file_replace_new" || state.ValidityVersion != 4 {
		t.Fatalf("replacement fence state=%+v", state)
	}
}

func ticketNoByDigest(t *testing.T, ctx context.Context, database gdb.DB, digest [sha256.Size]byte) string {
	t.Helper()
	value, err := database.GetValue(ctx, `SELECT ticket_no FROM plugin_idc_core_content_download_ticket WHERE token_digest=?`, hex.EncodeToString(digest[:]))
	if err != nil || value.String() == "" {
		t.Fatalf("load download ticket number: value=%q err=%v", value.String(), err)
	}
	return value.String()
}

func TestVisibilityChangeAdvancesDownloadValidityFencePostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	seedPublishedDownload(t, ctx, database, "cnt_visibility", "file_visibility", "upload_visibility", 5, 9)
	ticket, err := service.IssuePublicDownload(ctx, "cnt_visibility")
	if err != nil {
		t.Fatalf("issue public download ticket: %v", err)
	}
	if _, err = service.Update(ctx, WriteInput{
		ContentNo: "cnt_visibility", Type: "download", Visibility: "all_logged_in",
		Title: "Download", Body: "Body", Version: 9,
	}); err != nil {
		t.Fatalf("change content visibility: %v", err)
	}
	validity, err := database.GetValue(ctx, `SELECT validity_version FROM plugin_idc_core_content_item WHERE content_no='cnt_visibility'`)
	if err != nil {
		t.Fatal(err)
	}
	if validity.Int64() != 6 {
		t.Fatalf("visibility change validity=%d, want 6", validity.Int64())
	}
	if _, err = service.OpenPublicDownload(ctx, ticket.Token); !bizerr.Is(err, CodeNotFound) {
		t.Fatalf("public ticket survived visibility removal: %v", err)
	}
}

func TestContentManagerCanCancelAnotherManagersUnfinishedUploadPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	storage := &contentHubStorageStub{}
	service.storage = storage
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item
        (tenant_id,content_no,content_type,visibility_code,title,body,public_path,status_code,version,created_by,updated_by)
        VALUES (0,'cnt_cancel_upload','download','internal','Download','Body','/pages/downloads/cnt_cancel_upload','draft',1,99,99)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_upload_session
        (tenant_id,upload_no,content_no,uploader_user_id,file_name,declared_size_bytes,
         persisted_size_bytes,temporary_object_ref,transfer_mode,chunk_size_bytes,status_code,expires_at,version)
        VALUES (0,'upload_other_manager','cnt_cancel_upload',99,'guide.pdf',10,0,
                'tmp/other-manager','chunk_objects',5242880,'uploading',CURRENT_TIMESTAMP+INTERVAL '1 hour',4)`); err != nil {
		t.Fatal(err)
	}
	if err := service.CancelUpload(ctx, "upload_other_manager", 4, true); err != nil {
		t.Fatalf("cancel another manager upload: %v", err)
	}
	var row struct {
		Status    string `orm:"status_code"`
		ObjectRef string `orm:"temporary_object_ref"`
		Cleaned   bool   `orm:"cleaned"`
	}
	if err := database.GetScan(ctx, &row, `SELECT status_code,temporary_object_ref,
        cleanup_completed_at IS NOT NULL AS cleaned
        FROM plugin_idc_core_content_upload_session WHERE upload_no='upload_other_manager'`); err != nil {
		t.Fatal(err)
	}
	if row.Status != "cancelled" || row.ObjectRef != "" || !row.Cleaned || len(storage.deleted) != 1 || storage.deleted[0] != "tmp/other-manager" {
		t.Fatalf("unexpected cancelled upload state row=%+v deleted=%v", row, storage.deleted)
	}
}

func TestDeleteContentCancelsFilesUploadsAndLateScanPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item
        (tenant_id,content_no,content_type,visibility_code,title,body,public_path,status_code,
         validity_version,current_file_no,version,created_by,updated_by,published_at)
        VALUES (0,'cnt_delete_files','download','public','Download','Body',
                '/pages/downloads/cnt_delete_files','published',3,'file_delete_current',1,7,7,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO plugin_idc_core_content_upload_session
         (tenant_id,upload_no,content_no,uploader_user_id,file_name,declared_size_bytes,
          persisted_size_bytes,temporary_object_ref,transfer_mode,chunk_size_bytes,status_code,completed_at,expires_at)
         VALUES (0,'upload_delete_current','cnt_delete_files',7,'current.pdf',4,4,'object/current','chunk_objects',5242880,'completed',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 day')`,
		`INSERT INTO plugin_idc_core_content_upload_session
         (tenant_id,upload_no,content_no,uploader_user_id,file_name,declared_size_bytes,
          persisted_size_bytes,temporary_object_ref,transfer_mode,chunk_size_bytes,status_code,completed_at,expires_at)
         VALUES (0,'upload_delete_candidate','cnt_delete_files',7,'candidate.zip',4,4,'object/candidate','chunk_objects',5242880,'completed',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 day')`,
		`INSERT INTO plugin_idc_core_content_upload_session
         (tenant_id,upload_no,content_no,uploader_user_id,file_name,declared_size_bytes,
          persisted_size_bytes,temporary_object_ref,transfer_mode,chunk_size_bytes,status_code,expires_at)
         VALUES (0,'upload_delete_active','cnt_delete_files',99,'later.pdf',4,0,'object/active','chunk_objects',5242880,'uploading',CURRENT_TIMESTAMP+INTERVAL '1 day')`,
		`INSERT INTO plugin_idc_core_content_file
         (tenant_id,file_no,content_no,upload_no,candidate_generation,role_code,status_code,
          file_name,media_type,content_kind,size_bytes,sha256_hex,object_ref,scan_required,scan_status,enabled_at,created_by)
         VALUES (0,'file_delete_current','cnt_delete_files','upload_delete_current',1,'current','active',
                 'current.pdf','application/pdf','pdf',4,repeat('a',64),'object/current',FALSE,'not_configured',CURRENT_TIMESTAMP,7)`,
		`INSERT INTO plugin_idc_core_content_file
         (tenant_id,file_no,content_no,upload_no,candidate_generation,role_code,status_code,
          file_name,media_type,content_kind,size_bytes,sha256_hex,object_ref,scan_required,scan_status,retention_expires_at,created_by)
         VALUES (0,'file_delete_candidate','cnt_delete_files','upload_delete_candidate',2,'candidate','checking',
                 'candidate.zip','application/zip','zip',4,repeat('b',64),'object/candidate',TRUE,'scanning',CURRENT_TIMESTAMP+INTERVAL '1 day',7)`,
		`INSERT INTO plugin_idc_core_content_scan_attempt
         (tenant_id,attempt_no,file_no,candidate_generation,status_code,queued_at,started_at,lease_expires_at,created_by,version)
         VALUES (0,'attempt_delete','file_delete_candidate',2,'scanning',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,
                 CURRENT_TIMESTAMP+INTERVAL '1 minute',7,1)`,
	} {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	deleted, err := service.Delete(ctx, "cnt_delete_files", 1)
	if err != nil {
		t.Fatalf("delete content with file workflows: %v", err)
	}
	if deleted == nil || deleted.Status != "deleted" || deleted.Version != 2 || deleted.Body != "" {
		t.Fatalf("invalid deletion acknowledgement: %+v", deleted)
	}
	var files []struct{ FileNo, Role, Status, ObjectRef string }
	if err := database.GetScan(ctx, &files, `SELECT file_no,role_code AS role,status_code AS status,
        COALESCE(object_ref,'') AS object_ref FROM plugin_idc_core_content_file
        WHERE content_no='cnt_delete_files' ORDER BY file_no`); err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Status != "cancelled" || files[0].ObjectRef != "" ||
		files[1].Role != "retired" || files[1].Status != "replaced" || files[1].ObjectRef != "object/current" {
		t.Fatalf("unexpected terminal file states: %+v", files)
	}
	attemptStatus, err := database.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_content_scan_attempt WHERE attempt_no='attempt_delete'`)
	if err != nil {
		t.Fatal(err)
	}
	uploadStatus, err := database.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_content_upload_session WHERE upload_no='upload_delete_active'`)
	if err != nil {
		t.Fatal(err)
	}
	if attemptStatus.String() != "failed" || uploadStatus.String() != "cancelled" {
		t.Fatalf("attempt/upload states=%s/%s", attemptStatus.String(), uploadStatus.String())
	}
	worker := &scanWorker{database: database}
	if err := worker.finishClean(ctx, &scanClaim{AttemptNo: "attempt_delete", FileNo: "file_delete_candidate", CandidateGeneration: 2, AttemptVersion: 1}); err == nil {
		t.Fatal("late scanner completion restored a deleted content candidate")
	}
}

type contentHubStorageStub struct {
	storagecap.Service
	deleted []string
}

func (s *contentHubStorageStub) SupportsMultipart(context.Context) (bool, error) {
	return false, nil
}

func (s *contentHubStorageStub) DeleteMany(_ context.Context, input storagecap.DeleteManyInput) error {
	s.deleted = append(s.deleted, input.Paths...)
	return nil
}

func seedPublishedDownload(t *testing.T, ctx context.Context, database gdb.DB, contentNo, fileNo, uploadNo string, validity, version int64) {
	t.Helper()
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item
        (tenant_id,content_no,content_type,visibility_code,title,body,public_path,status_code,
         validity_version,current_file_no,version,created_by,updated_by,published_at)
        VALUES (0,?,'download','public','Download','Body',?,'published',?,?,?,7,7,CURRENT_TIMESTAMP)`,
		contentNo, "/pages/downloads/"+contentNo, validity, fileNo, version); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_upload_session
        (tenant_id,upload_no,content_no,uploader_user_id,file_name,declared_size_bytes,
         persisted_size_bytes,temporary_object_ref,transfer_mode,chunk_size_bytes,status_code,completed_at,expires_at)
        VALUES (0,?,?,7,'guide.pdf',10,10,?,'chunk_objects',5242880,'completed',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 hour')`,
		uploadNo, contentNo, "tmp/"+uploadNo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_file
        (tenant_id,file_no,content_no,upload_no,candidate_generation,role_code,status_code,
         file_name,media_type,content_kind,size_bytes,sha256_hex,object_ref,scan_required,scan_status,gate_completed_at,enabled_at,created_by)
        VALUES (0,?,?,?,1,'current','active','guide.pdf','application/pdf','pdf',10,
                repeat('a',64),?,FALSE,'not_configured',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,7)`,
		fileNo, contentNo, uploadNo, "downloads/"+fileNo); err != nil {
		t.Fatal(err)
	}
}

type contentHubBizCtxStub struct{}

func (contentHubBizCtxStub) Current(context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2}
}

type contentHubPrincipalStub struct{}

func (contentHubPrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, nil
}

type contentHubAuthzStub struct{}

func (contentHubAuthzStub) BatchGetPermissions(context.Context, []authz.PermissionKey) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}
func (contentHubAuthzStub) BatchHasPermissions(_ context.Context, keys []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	result := make(map[authz.PermissionKey]bool, len(keys))
	for _, key := range keys {
		result[key] = key == authz.PermissionKey(PermissionManage)
	}
	return result, nil
}
func (contentHubAuthzStub) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return true, nil
}
func (contentHubAuthzStub) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return false, nil
}
func (contentHubAuthzStub) ReplaceRolePermissions(context.Context, authz.RoleID, []authz.PermissionKey) error {
	return nil
}

type contentHubI18nStub struct{}

func (contentHubI18nStub) GetLocale(context.Context) string { return "zh-CN" }
func (contentHubI18nStub) Translate(_ context.Context, key, _ string) string {
	return "translated:" + key
}

func newAuthorizedContentHubTestService(database gdb.DB) *serviceImpl {
	return &serviceImpl{database: database, bizCtx: contentHubBizCtxStub{}, principal: contentHubPrincipalStub{}, authorization: contentHubAuthzStub{}, i18n: contentHubI18nStub{}, random: rand.Reader}
}

func newContentHubTestDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(contentHubIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run content hub PostgreSQL tests", contentHubIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var adminIdentity struct {
		DatabaseName string `orm:"database_name"`
		SchemaName   string `orm:"schema_name"`
	}
	if err = admin.GetScan(ctx, &adminIdentity, `SELECT current_database() AS database_name,current_schema() AS schema_name`); err != nil {
		t.Fatal(err)
	}
	if adminIdentity.DatabaseName != contentHubIntegrationDatabase || adminIdentity.SchemaName != "public" {
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close refused PostgreSQL admin connection: %v", closeErr)
		}
		t.Fatalf("refusing content hub integration database %q schema %q", adminIdentity.DatabaseName, adminIdentity.SchemaName)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_contenthub_" + hex.EncodeToString(suffix[:])
	if !validContentHubTestSchema(schema) {
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close PostgreSQL admin connection after invalid schema: %v", closeErr)
		}
		t.Fatalf("refusing invalid content hub test schema %q", schema)
	}
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close PostgreSQL admin connection after schema failure: %v", closeErr)
		}
		t.Fatal(err)
	}
	t.Logf("created isolated PostgreSQL schema database=%s schema=%s", adminIdentity.DatabaseName, schema)
	var database gdb.DB
	t.Cleanup(func() {
		if database != nil {
			if closeErr := database.Close(ctx); closeErr != nil {
				t.Errorf("close isolated PostgreSQL connection: %v", closeErr)
			}
		}
		if !validContentHubTestSchema(schema) {
			t.Errorf("refusing cleanup for invalid content hub test schema %q", schema)
			return
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop isolated PostgreSQL schema %q: %v", schema, dropErr)
		} else {
			t.Logf("deleted isolated PostgreSQL schema database=%s schema=%s", adminIdentity.DatabaseName, schema)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close PostgreSQL admin connection: %v", closeErr)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err = gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	var scopedIdentity struct {
		DatabaseName string `orm:"database_name"`
		SchemaName   string `orm:"schema_name"`
		SearchPath   string `orm:"search_path"`
	}
	if err = database.GetScan(ctx, &scopedIdentity, `SELECT current_database() AS database_name,current_schema() AS schema_name,current_setting('search_path') AS search_path`); err != nil {
		t.Fatal(err)
	}
	if scopedIdentity.DatabaseName != contentHubIntegrationDatabase || scopedIdentity.SchemaName != schema || !strings.Contains(scopedIdentity.SearchPath, schema) {
		t.Fatalf("isolated PostgreSQL identity mismatch: database=%q schema=%q search_path=%q", scopedIdentity.DatabaseName, scopedIdentity.SchemaName, scopedIdentity.SearchPath)
	}
	t.Logf("verified isolated PostgreSQL connection database=%s schema=%s search_path=%s", scopedIdentity.DatabaseName, scopedIdentity.SchemaName, scopedIdentity.SearchPath)
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_customer_subject (tenant_id BIGINT NOT NULL DEFAULT 0,customer_no VARCHAR(40) PRIMARY KEY,display_name VARCHAR(128) NOT NULL DEFAULT '')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_customer_group (id BIGSERIAL PRIMARY KEY,tenant_id BIGINT NOT NULL DEFAULT 0,group_no VARCHAR(40) UNIQUE NOT NULL,name_zh_cn VARCHAR(128) NOT NULL DEFAULT '',name_en_us VARCHAR(128) NOT NULL DEFAULT '',status_code VARCHAR(16) NOT NULL DEFAULT 'active')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_product_fulfillment_intent (tenant_id BIGINT NOT NULL DEFAULT 0,intent_no VARCHAR(40) UNIQUE NOT NULL,product_name_zh_cn VARCHAR(256) NOT NULL,product_name_en_us VARCHAR(256) NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_service_entitlement (id BIGSERIAL PRIMARY KEY,tenant_id BIGINT NOT NULL DEFAULT 0,entitlement_no VARCHAR(40) UNIQUE NOT NULL,source_intent_no VARCHAR(40) NOT NULL,customer_no VARCHAR(40) NOT NULL,status_code VARCHAR(24) NOT NULL DEFAULT 'active',version BIGINT NOT NULL DEFAULT 1)`); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "manifest", "sql"))
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "042-support-content.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(content)); err != nil {
		t.Fatalf("execute support/content migration: %v", err)
	}
	return ctx, database
}

func validContentHubTestSchema(schema string) bool {
	const prefix = "linapro_contenthub_"
	if !strings.HasPrefix(schema, prefix) || len(schema) != len(prefix)+16 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(schema, prefix))
	return err == nil
}

type audienceQueryCounter struct {
	gdb.DB
	writes int
}

func (c *audienceQueryCounter) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	c.writes++
	return c.DB.Exec(ctx, query, args...)
}

// Batch replacement must retain exact audience membership and rollback atomically.
func TestReplaceAudienceBatchAndRollbackPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	counter := &audienceQueryCounter{DB: database}
	service.database = counter
	input := WriteInput{}
	for i := 0; i < 500; i++ {
		input.TargetCustomerGroupNos = append(input.TargetCustomerGroupNos, fmt.Sprintf("GRP_%03d", i))
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item (tenant_id,content_no,content_type,visibility_code,title,body,public_path,status_code,version,created_by,updated_by) VALUES (0,'cnt_batch','news','customer_group','Batch','Body','/pages/cnt_batch','draft',1,7,7),(0,'cnt_other','news','customer_group','Other','Body','/pages/cnt_other','draft',1,7,7)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_group_scope(tenant_id,content_no,group_no) VALUES(0,'cnt_other','GRP_keep')`); err != nil {
		t.Fatal(err)
	}
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return service.replaceTargets(ctx, "cnt_batch", input) }); err != nil {
		t.Fatal(err)
	}
	if counter.writes > 2 {
		t.Fatalf("audience save performed %d writes for 500 groups", counter.writes)
	}
	for _, check := range []struct {
		table string
		want  int
	}{{"group", 500}} {
		count, err := database.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_content_"+check.table+"_scope WHERE tenant_id=0 AND content_no='cnt_batch'")
		if err != nil || count.Int() != check.want {
			t.Fatalf("%s audience count=%v err=%v", check.table, count, err)
		}
	}
	// Duplicate membership violates uniqueness after deletes, so the transaction
	// must restore all original target sets rather than partially removing access.
	invalid := WriteInput{TargetCustomerGroupNos: []string{"GRP_dup", "GRP_dup"}}
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return service.replaceTargets(ctx, "cnt_batch", invalid) }); err == nil {
		t.Fatal("duplicate audience committed")
	}
	count, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_content_group_scope WHERE tenant_id=0 AND content_no='cnt_batch'`)
	if err != nil || count.Int() != 500 {
		t.Fatal("failed replacement did not restore audience", err)
	}
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return service.replaceTargets(ctx, "cnt_batch", WriteInput{})
	}); err != nil {
		t.Fatal(err)
	}
	count, err = database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_content_group_scope`)
	if err != nil || count.Int() != 1 {
		t.Fatal("empty replacement changed another content", err)
	}
}

// TestAdminNumberedContentPages checks exact totals, page boundaries and deleted-item isolation.
func TestAdminNumberedContentPages(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	for index := 1; index <= 4; index++ {
		status := "draft"
		if index == 4 {
			status = "deleted"
		}
		_, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item (tenant_id,content_no,content_type,visibility_code,title,summary,body,seo_title,seo_description,public_path,status_code,created_by,updated_by) VALUES (0,?,'news','public',?,'Description','<p>Body</p>','Old metadata','Old description',?,?,7,7)`, fmt.Sprintf("CNT_PAGE_%d", index), fmt.Sprintf("Page %d", index), fmt.Sprintf("/news/page-%d", index), status)
		if err != nil {
			t.Fatal(err)
		}
	}
	first, err := service.SearchAdmin(ctx, SearchInput{Type: "news", Query: "Page", Page: 1, Limit: 2})
	if err != nil || first.Total != 3 || len(first.Items) != 2 || first.Items[0].ContentNo != "CNT_PAGE_3" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := service.SearchAdmin(ctx, SearchInput{Type: "news", Query: "Page", Page: 2, Limit: 2})
	if err != nil || second.Total != 3 || len(second.Items) != 1 || second.Items[0].ContentNo != "CNT_PAGE_1" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	empty, err := service.SearchAdmin(ctx, SearchInput{Type: "download", Page: 1, Limit: 2})
	if err != nil || empty.Total != 0 || len(empty.Items) != 0 {
		t.Fatalf("empty=%+v err=%v", empty, err)
	}
	if _, err = service.SearchAdmin(ctx, SearchInput{Page: 1, Cursor: first.NextCursor, Limit: 2}); err == nil {
		t.Fatal("numbered page accepted a cursor")
	}
	if _, err = service.DetailAdmin(ctx, "CNT_PAGE_4"); err == nil {
		t.Fatal("deleted content remained readable in ordinary administration")
	}
	item, err := service.DetailAdmin(ctx, "CNT_PAGE_1")
	if err != nil || item.SEOTitle != "Page 1" || item.SEODescription != "Description" {
		t.Fatalf("metadata=%+v err=%v", item, err)
	}
}
