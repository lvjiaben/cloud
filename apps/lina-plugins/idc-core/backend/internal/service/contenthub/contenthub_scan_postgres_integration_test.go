// contenthub_scan_postgres_integration_test.go verifies scanner and cleanup fences on PostgreSQL.
package contenthub

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/storagecap"
)

type scanStorageFake struct {
	objects   map[string][]byte
	deleted   map[string]int
	deleteErr error
}

func (s *scanStorageFake) Get(_ context.Context, input storagecap.GetInput) (*storagecap.GetOutput, error) {
	content, found := s.objects[input.Path]
	if !found {
		return &storagecap.GetOutput{}, nil
	}
	return &storagecap.GetOutput{Found: true, Body: io.NopCloser(bytes.NewReader(content))}, nil
}

func (s *scanStorageFake) Delete(_ context.Context, input storagecap.DeleteInput) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.objects, input.Path)
	s.deleted[input.Path]++
	return nil
}

func (s *scanStorageFake) DeleteMany(ctx context.Context, input storagecap.DeleteManyInput) error {
	for _, objectPath := range input.Paths {
		if err := s.Delete(ctx, storagecap.DeleteInput{Path: objectPath}); err != nil {
			return err
		}
	}
	return nil
}

func (s *scanStorageFake) AbortMultipart(context.Context, storagecap.MultipartAbortInput) error {
	return s.deleteErr
}

type scannerFake struct {
	configured     bool
	notReady       bool
	timeout        time.Duration
	dangerousLimit int64
	verdict        string
	err            error
	calls          int
}

func (s *scannerFake) Configured() bool { return s.configured }
func (s *scannerFake) Ready() bool      { return s.configured && !s.notReady }
func (s *scannerFake) DangerousLimitBytes() int64 {
	if s.dangerousLimit > 0 {
		return s.dangerousLimit
	}
	return maximumDangerousDownloadBytes
}
func (s *scannerFake) Timeout() time.Duration         { return s.timeout }
func (s *scannerFake) SelfTest(context.Context) error { return s.err }
func (s *scannerFake) Scan(_ context.Context, body io.Reader, expected int64) (string, error) {
	s.calls++
	content, err := io.ReadAll(body)
	if err != nil || int64(len(content)) != expected {
		return "", io.ErrUnexpectedEOF
	}
	return s.verdict, s.err
}

func TestContentScanWorkerPassesCleanCandidateWithSevenDayGatePostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	seedScanCandidate(t, ctx, database, "clean", "pending", "scanning", time.Now().Add(24*time.Hour), nil)
	storage := newScanStorage("clean")
	scanner := &scannerFake{configured: true, timeout: time.Minute, verdict: ScanVerdictClean}
	worker, err := NewScanWorker(database, storage, scanner)
	if err != nil {
		t.Fatal(err)
	}
	processed, err := worker.Run(ctx, 1)
	if err != nil || processed != 1 || scanner.calls != 1 {
		t.Fatalf("processed=%d calls=%d err=%v", processed, scanner.calls, err)
	}
	var row struct {
		Status, ScanStatus, AttemptStatus   string
		GateCompletedAt, RetentionExpiresAt *time.Time
	}
	if err = database.GetScan(ctx, &row, `SELECT f.status_code AS status,f.scan_status,a.status_code AS attempt_status,f.gate_completed_at,f.retention_expires_at FROM plugin_idc_core_content_file f JOIN plugin_idc_core_content_scan_attempt a ON a.file_no=f.file_no WHERE f.file_no='file_clean'`); err != nil {
		t.Fatal(err)
	}
	if row.Status != "ready" || row.ScanStatus != "passed" || row.AttemptStatus != "passed" || row.GateCompletedAt == nil || row.RetentionExpiresAt == nil || row.RetentionExpiresAt.Sub(*row.GateCompletedAt) != 7*24*time.Hour {
		t.Fatalf("unexpected clean result: %+v", row)
	}
}

func TestRetryCandidateScanUsesLockedContentVersionAndCreatesOneAttemptPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	seedScanCandidate(t, ctx, database, "rescan", "", "failed", time.Now().Add(24*time.Hour), nil)
	service := newAuthorizedContentHubTestService(database)
	service.scanner = &scannerFake{configured: true, timeout: time.Minute}
	input := CandidateActionInput{
		ContentNo: "content_rescan", ContentVersion: 1,
		FileNo: "file_rescan", FileVersion: 1, Confirmed: true,
	}
	if _, err := service.RetryCandidateScan(ctx, input); err != nil {
		t.Fatalf("retry candidate scan: %v", err)
	}
	if _, err := service.RetryCandidateScan(ctx, input); err == nil {
		t.Fatal("stale retry candidate scan was accepted")
	}
	var row struct {
		ScanStatus, AttemptStatus string
		FileVersion, Attempts     int64
	}
	if err := database.GetScan(ctx, &row, `SELECT f.scan_status,f.version AS file_version,(SELECT COUNT(1) FROM plugin_idc_core_content_scan_attempt a WHERE a.file_no=f.file_no) AS attempts,(SELECT status_code FROM plugin_idc_core_content_scan_attempt a WHERE a.file_no=f.file_no LIMIT 1) AS attempt_status FROM plugin_idc_core_content_file f WHERE f.file_no='file_rescan'`); err != nil {
		t.Fatal(err)
	}
	if row.ScanStatus != "scanning" || row.AttemptStatus != "pending" || row.FileVersion != 2 || row.Attempts != 1 {
		t.Fatalf("unexpected rescan state: %+v", row)
	}
}

func TestContentScanWorkerDeletesRiskAndNeverStoresScannerOutputPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	seedScanCandidate(t, ctx, database, "risk", "pending", "scanning", time.Now().Add(24*time.Hour), nil)
	storage := newScanStorage("risk")
	scanner := &scannerFake{configured: true, timeout: time.Minute, verdict: ScanVerdictRisk}
	worker, err := NewScanWorker(database, storage, scanner)
	if err != nil {
		t.Fatal(err)
	}
	if processed, runErr := worker.Run(ctx, 1); runErr != nil || processed != 1 {
		t.Fatalf("processed=%d err=%v", processed, runErr)
	}
	var row struct {
		Status, ScanStatus, RiskCode, ObjectRef, TemporaryObjectRef string
		ObjectDeletedAt                                             *time.Time
	}
	if err = database.GetScan(ctx, &row, `SELECT f.status_code AS status,f.scan_status,COALESCE(a.risk_code,'') AS risk_code,COALESCE(f.object_ref,'') AS object_ref,u.temporary_object_ref,f.object_deleted_at FROM plugin_idc_core_content_file f JOIN plugin_idc_core_content_scan_attempt a ON a.file_no=f.file_no JOIN plugin_idc_core_content_upload_session u ON u.upload_no=f.upload_no WHERE f.file_no='file_risk'`); err != nil {
		t.Fatal(err)
	}
	if row.Status != "risk_rejected" || row.ScanStatus != "risk_found" || row.RiskCode != "malware_detected" || row.ObjectRef != "" || row.TemporaryObjectRef != "" || row.ObjectDeletedAt == nil || storage.deleted["object_risk"] != 1 {
		t.Fatalf("unexpected risk result: row=%+v deleted=%v", row, storage.deleted)
	}
}

func TestContentScanWorkerRetriesRiskObjectDeletionWithoutRestoringCandidatePostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	seedScanCandidate(t, ctx, database, "risk_delete_retry", "pending", "scanning", time.Now().Add(24*time.Hour), nil)
	storage := newScanStorage("risk_delete_retry")
	storage.deleteErr = errors.New("storage unavailable")
	scanner := &scannerFake{configured: true, timeout: time.Minute, verdict: ScanVerdictRisk}
	worker, err := NewScanWorker(database, storage, scanner)
	if err != nil {
		t.Fatal(err)
	}
	if processed, runErr := worker.Run(ctx, 1); processed != 1 || runErr == nil {
		t.Fatalf("processed=%d err=%v", processed, runErr)
	}
	var row struct {
		Status, ObjectRef string
		ObjectDeletedAt   *time.Time
	}
	if err = database.GetScan(ctx, &row, `SELECT status_code AS status,COALESCE(object_ref,'') AS object_ref,object_deleted_at FROM plugin_idc_core_content_file WHERE file_no='file_risk_delete_retry'`); err != nil {
		t.Fatal(err)
	}
	if row.Status != "risk_rejected" || row.ObjectRef != "" || row.ObjectDeletedAt != nil {
		t.Fatalf("risk candidate did not remain terminal while delete awaited retry: %+v", row)
	}
	storage.deleteErr = nil
	if processed, runErr := worker.Run(ctx, 1); processed != 0 || runErr != nil {
		t.Fatalf("cleanup processed=%d err=%v", processed, runErr)
	}
	var deletedRow struct {
		TemporaryObjectRef string
		ObjectDeletedAt    *time.Time
	}
	if err = database.GetScan(ctx, &deletedRow, `SELECT u.temporary_object_ref,f.object_deleted_at FROM plugin_idc_core_content_file f JOIN plugin_idc_core_content_upload_session u ON u.upload_no=f.upload_no WHERE f.file_no='file_risk_delete_retry'`); err != nil || deletedRow.ObjectDeletedAt == nil || deletedRow.TemporaryObjectRef != "" || storage.deleted["object_risk_delete_retry"] != 1 {
		t.Fatalf("risk object deletion was not retried: row=%+v deleted=%v err=%v", deletedRow, storage.deleted, err)
	}
}

func TestContentScanWorkerFailsWithoutAutomaticRetryPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	seedScanCandidate(t, ctx, database, "failed", "pending", "scanning", time.Now().Add(24*time.Hour), nil)
	storage := newScanStorage("failed")
	scanner := &scannerFake{configured: true, timeout: time.Minute, err: errors.New("clamd unavailable")}
	worker, err := NewScanWorker(database, storage, scanner)
	if err != nil {
		t.Fatal(err)
	}
	processed, runErr := worker.Run(ctx, 1)
	if processed != 1 || runErr == nil {
		t.Fatalf("processed=%d err=%v", processed, runErr)
	}
	processed, runErr = worker.Run(ctx, 1)
	if processed != 0 || runErr != nil || scanner.calls != 1 {
		t.Fatalf("failed scan retried automatically: processed=%d calls=%d err=%v", processed, scanner.calls, runErr)
	}
	scanStatus, err := database.GetValue(ctx, `SELECT scan_status FROM plugin_idc_core_content_file WHERE file_no='file_failed'`)
	if err != nil || scanStatus.String() != "failed" {
		t.Fatalf("scan status=%q err=%v", scanStatus.String(), err)
	}
}

func TestContentScanWorkerExpiresUnconfiguredCandidateAndFencesLateResultPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	past := time.Now().Add(-time.Minute)
	seedScanCandidate(t, ctx, database, "expired", "", "not_configured", past, nil)
	storage := newScanStorage("expired")
	disabled := &scannerFake{timeout: time.Minute}
	worker, err := NewScanWorker(database, storage, disabled)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = worker.Run(ctx, 1); err != nil {
		t.Fatal(err)
	}
	var row struct {
		Status, ObjectRef string
		ObjectDeletedAt   *time.Time
	}
	if err = database.GetScan(ctx, &row, `SELECT status_code AS status,COALESCE(object_ref,'') AS object_ref,object_deleted_at FROM plugin_idc_core_content_file WHERE file_no='file_expired'`); err != nil {
		t.Fatal(err)
	}
	if row.Status != "expired" || row.ObjectRef != "" || row.ObjectDeletedAt == nil || storage.deleted["object_expired"] != 1 {
		t.Fatalf("unexpected expired candidate: row=%+v deleted=%v", row, storage.deleted)
	}

	seedScanCandidate(t, ctx, database, "late", "pending", "scanning", time.Now().Add(time.Hour), nil)
	active := &scanWorker{database: database, storage: newScanStorage("late"), scanner: &scannerFake{configured: true, timeout: time.Minute, verdict: ScanVerdictClean}}
	claim, err := active.claim(ctx)
	if err != nil || claim == nil {
		t.Fatalf("claim=%+v err=%v", claim, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET status_code='cancelled',object_ref=NULL,version=version+1 WHERE file_no='file_late'`); err != nil {
		t.Fatal(err)
	}
	if err = active.finishClean(ctx, claim); err == nil {
		t.Fatal("late scan result crossed cancelled candidate fence")
	}
	lateStatus, statusErr := database.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_content_file WHERE file_no='file_late'`)
	if statusErr != nil || lateStatus.String() != "cancelled" {
		t.Fatalf("late result restored candidate: status=%q err=%v", lateStatus.String(), statusErr)
	}
}

func TestContentScanWorkerExpiresLeaseWithoutAutomaticRetryPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	expiredLease := time.Now().Add(-time.Minute)
	seedScanCandidate(t, ctx, database, "lease_expired", "scanning", "scanning", time.Now().Add(24*time.Hour), &expiredLease)
	scanner := &scannerFake{timeout: time.Minute}
	worker, err := NewScanWorker(database, newScanStorage("lease_expired"), scanner)
	if err != nil {
		t.Fatal(err)
	}
	if processed, runErr := worker.Run(ctx, 1); processed != 0 || runErr != nil {
		t.Fatalf("processed=%d err=%v", processed, runErr)
	}
	var row struct{ FileStatus, AttemptStatus string }
	if err = database.GetScan(ctx, &row, `SELECT f.scan_status AS file_status,a.status_code AS attempt_status FROM plugin_idc_core_content_file f JOIN plugin_idc_core_content_scan_attempt a ON a.file_no=f.file_no WHERE f.file_no='file_lease_expired'`); err != nil {
		t.Fatal(err)
	}
	if row.FileStatus != "failed" || row.AttemptStatus != "failed" || scanner.calls != 0 {
		t.Fatalf("expired lease was retried or left active: row=%+v calls=%d", row, scanner.calls)
	}
}

func TestContentScanWorkerReturnsPendingCandidateToUnconfiguredRetentionPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	seedScanCandidate(t, ctx, database, "became_unconfigured", "pending", "scanning", nil, nil)
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_content_upload_session SET completed_at=CURRENT_TIMESTAMP-INTERVAL '2 hours' WHERE upload_no='upload_became_unconfigured'`); err != nil {
		t.Fatal(err)
	}
	scanner := &scannerFake{timeout: time.Minute}
	worker, err := NewScanWorker(database, newScanStorage("became_unconfigured"), scanner)
	if err != nil {
		t.Fatal(err)
	}
	if processed, runErr := worker.Run(ctx, 1); processed != 0 || runErr != nil {
		t.Fatalf("processed=%d err=%v", processed, runErr)
	}
	var row struct {
		ScanStatus, AttemptStatus       string
		CompletedAt, RetentionExpiresAt time.Time
	}
	if err = database.GetScan(ctx, &row, `SELECT f.scan_status,a.status_code AS attempt_status,u.completed_at,f.retention_expires_at FROM plugin_idc_core_content_file f JOIN plugin_idc_core_content_scan_attempt a ON a.file_no=f.file_no JOIN plugin_idc_core_content_upload_session u ON u.upload_no=f.upload_no WHERE f.file_no='file_became_unconfigured'`); err != nil {
		t.Fatal(err)
	}
	if row.ScanStatus != "not_configured" || row.AttemptStatus != "failed" || row.RetentionExpiresAt.Sub(row.CompletedAt) != 24*time.Hour || scanner.calls != 0 {
		t.Fatalf("pending scan did not fail closed after configuration removal: row=%+v calls=%d", row, scanner.calls)
	}
}

func TestContentScanWorkerNeverDeletesCandidateForRiskResultAfterLeaseExpiryPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	seedScanCandidate(t, ctx, database, "risk_late", "pending", "scanning", time.Now().Add(24*time.Hour), nil)
	storage := newScanStorage("risk_late")
	worker := &scanWorker{database: database, storage: storage, scanner: &scannerFake{configured: true, timeout: time.Minute, verdict: ScanVerdictRisk}}
	claim, err := worker.claim(ctx)
	if err != nil || claim == nil {
		t.Fatalf("claim=%+v err=%v", claim, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_content_scan_attempt SET queued_at=CURRENT_TIMESTAMP-INTERVAL '3 minutes',started_at=CURRENT_TIMESTAMP-INTERVAL '2 minutes',lease_expires_at=CURRENT_TIMESTAMP-INTERVAL '1 minute' WHERE attempt_no='attempt_risk_late'`); err != nil {
		t.Fatal(err)
	}
	if err = worker.finishRisk(ctx, claim); err == nil {
		t.Fatal("late risk result crossed the expired attempt lease")
	}
	var row struct{ Status, ObjectRef string }
	if err = database.GetScan(ctx, &row, `SELECT status_code AS status,COALESCE(object_ref,'') AS object_ref FROM plugin_idc_core_content_file WHERE file_no='file_risk_late'`); err != nil {
		t.Fatal(err)
	}
	if row.Status != "checking" || row.ObjectRef != "object_risk_late" || storage.deleted["object_risk_late"] != 0 {
		t.Fatalf("late risk result deleted or changed the candidate: row=%+v deleted=%v", row, storage.deleted)
	}
}

func TestContentScanWorkerExpiresSevenDayReadyCandidatePostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	past := time.Now().Add(-time.Minute)
	seedScanCandidate(t, ctx, database, "ready_expired", "", "passed", past, nil)
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET status_code='ready',gate_completed_at=CURRENT_TIMESTAMP-INTERVAL '7 days 1 minute' WHERE file_no='file_ready_expired'`); err != nil {
		t.Fatal(err)
	}
	storage := newScanStorage("ready_expired")
	worker, err := NewScanWorker(database, storage, &scannerFake{timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if processed, runErr := worker.Run(ctx, 1); processed != 0 || runErr != nil {
		t.Fatalf("processed=%d err=%v", processed, runErr)
	}
	var row struct {
		Status          string
		ObjectDeletedAt *time.Time
	}
	if err = database.GetScan(ctx, &row, `SELECT status_code AS status,object_deleted_at FROM plugin_idc_core_content_file WHERE file_no='file_ready_expired'`); err != nil {
		t.Fatal(err)
	}
	if row.Status != "expired" || row.ObjectDeletedAt == nil || storage.deleted["object_ready_expired"] != 1 {
		t.Fatalf("seven-day ready candidate was not expired and deleted: row=%+v deleted=%v", row, storage.deleted)
	}
}

func TestContentScanWorkerPersistsExpiredUploadCleanupRetryPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item (tenant_id,content_no,content_type,visibility_code,title,body,public_path,status_code,version,created_by,updated_by) VALUES (0,'content_cleanup','download','public','Title','Body','/pages/downloads/content_cleanup','draft',1,7,7)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_upload_session (tenant_id,upload_no,content_no,uploader_user_id,file_name,declared_size_bytes,persisted_size_bytes,temporary_object_ref,transfer_mode,chunk_size_bytes,status_code,expires_at) VALUES (0,'upload_cleanup','content_cleanup',7,'file.pdf',4,4,'object_cleanup','chunk_objects',8388608,'uploading',CURRENT_TIMESTAMP-INTERVAL '1 minute')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_upload_part (tenant_id,upload_no,part_number,offset_bytes,size_bytes,object_ref,sha256_hex) VALUES (0,'upload_cleanup',1,0,4,'part_cleanup',repeat('a',64))`); err != nil {
		t.Fatal(err)
	}
	storage := &scanStorageFake{
		objects: map[string][]byte{"object_cleanup": []byte("data"), "part_cleanup": []byte("data")},
		deleted: make(map[string]int), deleteErr: errors.New("storage unavailable"),
	}
	worker, err := NewScanWorker(database, storage, &scannerFake{timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if _, runErr := worker.Run(ctx, 1); runErr == nil {
		t.Fatal("failed upload cleanup was reported as complete")
	}
	var failed struct {
		Status             string
		CleanupLease       *time.Time
		CleanupCompletedAt *time.Time
	}
	if err = database.GetScan(ctx, &failed, `SELECT status_code AS status,cleanup_lease_expires_at AS cleanup_lease,cleanup_completed_at FROM plugin_idc_core_content_upload_session WHERE upload_no='upload_cleanup'`); err != nil {
		t.Fatal(err)
	}
	if failed.Status != "expired" || failed.CleanupLease == nil || failed.CleanupCompletedAt != nil {
		t.Fatalf("failed cleanup was not persisted for retry: %+v", failed)
	}
	storage.deleteErr = nil
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_content_upload_session SET cleanup_lease_expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE upload_no='upload_cleanup'`); err != nil {
		t.Fatal(err)
	}
	if _, runErr := worker.Run(ctx, 1); runErr != nil {
		t.Fatal(runErr)
	}
	var completed struct {
		ObjectRef          string
		CleanupCompletedAt *time.Time
		Parts              int
	}
	if err = database.GetScan(ctx, &completed, `SELECT temporary_object_ref AS object_ref,cleanup_completed_at,(SELECT COUNT(1) FROM plugin_idc_core_content_upload_part WHERE upload_no='upload_cleanup') AS parts FROM plugin_idc_core_content_upload_session WHERE upload_no='upload_cleanup'`); err != nil {
		t.Fatal(err)
	}
	if completed.ObjectRef != "" || completed.CleanupCompletedAt == nil || completed.Parts != 0 || storage.deleted["object_cleanup"] != 1 || storage.deleted["part_cleanup"] != 1 {
		t.Fatalf("upload cleanup was not retried safely: row=%+v deleted=%v", completed, storage.deleted)
	}
}

func TestContentScanWorkerCompletedUploadCleanupKeepsCandidateObjectPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item (tenant_id,content_no,content_type,visibility_code,title,body,public_path,status_code,version,created_by,updated_by) VALUES (0,'content_part_cleanup','download','public','Title','Body','/pages/downloads/content_part_cleanup','draft',1,7,7)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_upload_session (tenant_id,upload_no,content_no,uploader_user_id,file_name,declared_size_bytes,persisted_size_bytes,temporary_object_ref,transfer_mode,chunk_size_bytes,status_code,completed_at,expires_at) VALUES (0,'upload_part_cleanup','content_part_cleanup',7,'file.pdf',4,4,'candidate_object','chunk_objects',8388608,'completed',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 day')`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_upload_part (tenant_id,upload_no,part_number,offset_bytes,size_bytes,object_ref,sha256_hex) VALUES (0,'upload_part_cleanup',1,0,4,'candidate_part',repeat('a',64))`); err != nil {
		t.Fatal(err)
	}
	storage := &scanStorageFake{
		objects: map[string][]byte{"candidate_object": []byte("data"), "candidate_part": []byte("data")},
		deleted: make(map[string]int),
	}
	worker, err := NewScanWorker(database, storage, &scannerFake{timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if _, runErr := worker.Run(ctx, 1); runErr != nil {
		t.Fatal(runErr)
	}
	cleanupAt, err := database.GetValue(ctx, `SELECT cleanup_completed_at FROM plugin_idc_core_content_upload_session WHERE upload_no='upload_part_cleanup'`)
	if err != nil || cleanupAt.IsNil() || storage.deleted["candidate_part"] != 1 || storage.deleted["candidate_object"] != 0 {
		t.Fatalf("completed upload cleanup changed the candidate object: cleanup=%v deleted=%v err=%v", cleanupAt, storage.deleted, err)
	}
}

func seedScanCandidate(t *testing.T, ctx context.Context, database gdb.DB, suffix, attemptStatus, scanStatus string, retention any, lease *time.Time) {
	t.Helper()
	contentNo, uploadNo, fileNo := "content_"+suffix, "upload_"+suffix, "file_"+suffix
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item (tenant_id,content_no,content_type,visibility_code,title,body,public_path,status_code,version,created_by,updated_by) VALUES (0,?,'download','public','Title','Body',?,'draft',1,7,7)`, contentNo, "/pages/downloads/"+contentNo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_upload_session (tenant_id,upload_no,content_no,uploader_user_id,file_name,declared_size_bytes,persisted_size_bytes,temporary_object_ref,transfer_mode,chunk_size_bytes,status_code,completed_at,expires_at) VALUES (0,?,?,7,'file.zip',4,4,?,'chunk_objects',8388608,'completed',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 day')`, uploadNo, contentNo, "object_"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_file (tenant_id,file_no,content_no,upload_no,candidate_generation,role_code,status_code,file_name,media_type,content_kind,size_bytes,sha256_hex,object_ref,scan_required,scan_status,retention_expires_at,version,created_by) VALUES (0,?,?,?,1,'candidate','checking','file.zip','application/zip','zip',4,repeat('a',64),?,TRUE,?,?,1,7)`, fileNo, contentNo, uploadNo, "object_"+suffix, scanStatus, retention); err != nil {
		t.Fatal(err)
	}
	if attemptStatus == "" {
		return
	}
	if attemptStatus == "pending" {
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_scan_attempt (tenant_id,attempt_no,file_no,candidate_generation,status_code,queued_at,created_by,version) VALUES (0,?, ?,1,'pending',CURRENT_TIMESTAMP,7,1)`, "attempt_"+suffix, fileNo); err != nil {
			t.Fatal(err)
		}
		return
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_scan_attempt (tenant_id,attempt_no,file_no,candidate_generation,status_code,queued_at,started_at,lease_expires_at,created_by,version) VALUES (0,?, ?,1,'scanning',CURRENT_TIMESTAMP-INTERVAL '2 minutes',CURRENT_TIMESTAMP-INTERVAL '2 minutes',?,7,1)`, "attempt_"+suffix, fileNo, lease); err != nil {
		t.Fatal(err)
	}
}

func newScanStorage(suffix string) *scanStorageFake {
	return &scanStorageFake{objects: map[string][]byte{"object_" + suffix: []byte("data")}, deleted: make(map[string]int)}
}
