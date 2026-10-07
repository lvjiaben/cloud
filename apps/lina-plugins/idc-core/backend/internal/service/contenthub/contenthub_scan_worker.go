// contenthub_scan_worker.go leases scans and durable upload-object cleanup work.
package contenthub

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/storagecap"
)

const (
	maximumScanBatch        = 10
	maximumCleanupBatch     = 100
	cleanCandidateRetention = 7 * 24 * time.Hour
	uploadCleanupLease      = time.Minute
)

type ScanWorker interface {
	Run(context.Context, int) (int, error)
}

type scanStorage interface {
	Get(context.Context, storagecap.GetInput) (*storagecap.GetOutput, error)
	Delete(context.Context, storagecap.DeleteInput) error
	DeleteMany(context.Context, storagecap.DeleteManyInput) error
	AbortMultipart(context.Context, storagecap.MultipartAbortInput) error
}

type scanWorker struct {
	database gdb.DB
	storage  scanStorage
	scanner  Scanner
}

type scanClaim struct {
	AttemptNo           string
	FileNo              string
	ObjectRef           string
	CandidateGeneration int64
	SizeBytes           int64
	AttemptVersion      int64
}

type uploadCleanupClaim struct {
	UploadNo, ObjectRef, TransferMode, ProviderUploadID, Status string
	PartRefs                                                    []string
	Version                                                     int64
}

func NewScanWorker(database gdb.DB, storage scanStorage, scanner Scanner) (ScanWorker, error) {
	if database == nil || storage == nil || scanner == nil || scanner.Timeout() <= 0 {
		return nil, gerror.New("content scan worker requires database, storage, and scanner boundaries")
	}
	return &scanWorker{database: database, storage: storage, scanner: scanner}, nil
}

func (w *scanWorker) Run(ctx context.Context, limit int) (int, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > maximumScanBatch {
		return 0, gerror.New("content scan worker request is invalid")
	}
	var runErrors []error
	if err := w.expireUploadSessions(ctx, maximumCleanupBatch); err != nil {
		runErrors = append(runErrors, err)
	}
	if err := w.cleanupUploadSessions(ctx, maximumScanBatch); err != nil {
		runErrors = append(runErrors, err)
	}
	if err := w.failExpiredLeases(ctx, maximumCleanupBatch); err != nil {
		runErrors = append(runErrors, err)
	}
	if !w.scanner.Ready() {
		if err := w.failUnconfiguredAttempts(ctx, maximumCleanupBatch); err != nil {
			runErrors = append(runErrors, err)
		}
	}
	if err := w.expireCandidates(ctx, maximumCleanupBatch); err != nil {
		runErrors = append(runErrors, err)
	}
	if err := w.deleteTerminalObjects(ctx, maximumCleanupBatch); err != nil {
		runErrors = append(runErrors, err)
	}
	processed := 0
	if w.scanner.Ready() {
		for processed < limit {
			claim, err := w.claim(ctx)
			if err != nil {
				runErrors = append(runErrors, err)
				break
			}
			if claim == nil {
				break
			}
			processed++
			if err = w.scan(ctx, claim); err != nil {
				runErrors = append(runErrors, err)
			}
		}
	}
	return processed, errors.Join(runErrors...)
}

func (w *scanWorker) expireUploadSessions(ctx context.Context, limit int) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := contentDatabaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		_, err = w.database.Exec(ctx, `WITH locked AS MATERIALIZED (
SELECT id FROM plugin_idc_core_content_upload_session
WHERE tenant_id=0 AND status_code='uploading' AND expires_at<=?
ORDER BY expires_at,id FOR UPDATE SKIP LOCKED LIMIT ?
)
UPDATE plugin_idc_core_content_upload_session s
SET status_code='expired',cleanup_lease_expires_at=NULL,version=s.version+1,updated_at=?
FROM locked l WHERE s.id=l.id AND s.status_code='uploading'`, now, limit, now)
		return err
	})
}

func (w *scanWorker) cleanupUploadSessions(ctx context.Context, limit int) error {
	var cleanupErrors []error
	for cleaned := 0; cleaned < limit; cleaned++ {
		claim, err := w.claimUploadCleanup(ctx)
		if err != nil {
			cleanupErrors = append(cleanupErrors, err)
			break
		}
		if claim == nil {
			break
		}
		if err = cleanupUploadObjects(ctx, w.storage, claim); err != nil {
			cleanupErrors = append(cleanupErrors, err)
			continue
		}
		if err = markUploadCleanupComplete(ctx, w.database, claim.UploadNo, claim.Version); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	return errors.Join(cleanupErrors...)
}

func (w *scanWorker) claimUploadCleanup(ctx context.Context) (*uploadCleanupClaim, error) {
	var claim *uploadCleanupClaim
	err := w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var rows []struct {
			UploadNo, ObjectRef, TransferMode, ProviderUploadID, Status, PartRefsJSON string
			Version                                                                   int64
		}
		if err := w.database.GetScan(ctx, &rows, `WITH locked AS MATERIALIZED (
SELECT id,upload_no,temporary_object_ref,transfer_mode,COALESCE(provider_upload_id,'') AS provider_upload_id,status_code,version
FROM plugin_idc_core_content_upload_session
WHERE tenant_id=0 AND cleanup_completed_at IS NULL
  AND (cleanup_lease_expires_at IS NULL OR cleanup_lease_expires_at<=CURRENT_TIMESTAMP)
  AND (status_code IN ('cancelled','expired') OR (status_code='completed' AND transfer_mode='chunk_objects'))
ORDER BY expires_at,id FOR UPDATE SKIP LOCKED LIMIT 1
)
SELECT l.upload_no,l.temporary_object_ref AS object_ref,l.transfer_mode,l.provider_upload_id,l.status_code AS status,l.version,
       COALESCE(jsonb_agg(p.object_ref ORDER BY p.part_number) FILTER (WHERE p.object_ref IS NOT NULL),'[]'::jsonb)::text AS part_refs_json
FROM locked l LEFT JOIN plugin_idc_core_content_upload_part p ON p.tenant_id=0 AND p.upload_no=l.upload_no
GROUP BY l.upload_no,l.temporary_object_ref,l.transfer_mode,l.provider_upload_id,l.status_code,l.version`); err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		row := rows[0]
		partRefs := make([]string, 0)
		if err := json.Unmarshal([]byte(row.PartRefsJSON), &partRefs); err != nil {
			return gerror.Wrap(err, "decode upload cleanup parts")
		}
		now, err := contentDatabaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		result, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_content_upload_session SET cleanup_lease_expires_at=?,version=version+1,updated_at=? WHERE tenant_id=0 AND upload_no=? AND version=? AND cleanup_completed_at IS NULL AND (cleanup_lease_expires_at IS NULL OR cleanup_lease_expires_at<=?)`, now.Add(uploadCleanupLease), now, row.UploadNo, row.Version, now)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return gerror.New("content upload cleanup lost its PostgreSQL lease fence")
		}
		claim = &uploadCleanupClaim{UploadNo: row.UploadNo, ObjectRef: row.ObjectRef, TransferMode: row.TransferMode, ProviderUploadID: row.ProviderUploadID, Status: row.Status, PartRefs: partRefs, Version: row.Version + 1}
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "claim content upload cleanup")
	}
	return claim, nil
}

func (w *scanWorker) failUnconfiguredAttempts(ctx context.Context, limit int) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := contentDatabaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		_, err = w.database.Exec(ctx, `WITH locked AS MATERIALIZED (
SELECT a.id AS attempt_id,a.file_no,a.candidate_generation,u.completed_at
FROM plugin_idc_core_content_scan_attempt a
JOIN plugin_idc_core_content_file f ON f.tenant_id=a.tenant_id AND f.file_no=a.file_no AND f.candidate_generation=a.candidate_generation
JOIN plugin_idc_core_content_upload_session u ON u.tenant_id=f.tenant_id AND u.upload_no=f.upload_no
WHERE a.tenant_id=0 AND a.status_code IN ('pending','scanning') AND f.role_code='candidate' AND f.status_code='checking' AND f.scan_status='scanning'
ORDER BY a.queued_at,a.id FOR UPDATE OF a,f SKIP LOCKED LIMIT ?
), finished AS (
UPDATE plugin_idc_core_content_scan_attempt a
SET status_code='failed',started_at=COALESCE(a.started_at,?),finished_at=?,lease_expires_at=NULL,version=a.version+1
FROM locked l WHERE a.id=l.attempt_id AND a.status_code IN ('pending','scanning')
RETURNING a.file_no,a.candidate_generation
)
UPDATE plugin_idc_core_content_file f
SET scan_status='not_configured',retention_expires_at=LEAST(COALESCE(f.retention_expires_at,l.completed_at+INTERVAL '24 hours'),l.completed_at+INTERVAL '24 hours'),version=f.version+1,updated_at=?
FROM locked l JOIN finished x ON x.file_no=l.file_no AND x.candidate_generation=l.candidate_generation
WHERE f.tenant_id=0 AND f.file_no=l.file_no AND f.candidate_generation=l.candidate_generation AND f.role_code='candidate' AND f.status_code='checking' AND f.scan_status='scanning'`, limit, now, now, now)
		return err
	})
}

func (w *scanWorker) claim(ctx context.Context) (*scanClaim, error) {
	var claim *scanClaim
	err := w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var rows []struct {
			AttemptNo                                      string
			FileNo, ObjectRef                              string
			CandidateGeneration, SizeBytes, AttemptVersion int64
		}
		if err := w.database.GetScan(ctx, &rows, `SELECT a.attempt_no,a.file_no,a.candidate_generation,a.version AS attempt_version,f.object_ref,f.size_bytes FROM plugin_idc_core_content_scan_attempt a JOIN plugin_idc_core_content_file f ON f.tenant_id=a.tenant_id AND f.file_no=a.file_no AND f.candidate_generation=a.candidate_generation WHERE a.tenant_id=0 AND a.status_code='pending' AND f.role_code='candidate' AND f.status_code='checking' AND f.scan_required=TRUE AND f.scan_status='scanning' AND f.object_ref IS NOT NULL ORDER BY a.queued_at,a.id FOR UPDATE OF a,f SKIP LOCKED LIMIT 1`); err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		now, err := contentDatabaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		row := rows[0]
		result, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_content_scan_attempt SET status_code='scanning',started_at=?,lease_expires_at=?,version=version+1 WHERE tenant_id=0 AND attempt_no=? AND status_code='pending' AND version=?`, now, now.Add(w.scanner.Timeout()), row.AttemptNo, row.AttemptVersion)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return gerror.New("content scan claim lost its PostgreSQL fence")
		}
		claim = &scanClaim{AttemptNo: row.AttemptNo, FileNo: row.FileNo, ObjectRef: row.ObjectRef, CandidateGeneration: row.CandidateGeneration, SizeBytes: row.SizeBytes, AttemptVersion: row.AttemptVersion + 1}
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "claim content scan attempt")
	}
	return claim, nil
}

func (w *scanWorker) scan(ctx context.Context, claim *scanClaim) error {
	object, err := w.storage.Get(ctx, storagecap.GetInput{Path: claim.ObjectRef})
	if err != nil || object == nil || !object.Found || object.Body == nil {
		failure := err
		if failure == nil {
			failure = gerror.New("candidate object is unavailable")
		}
		if markErr := w.finishFailed(ctx, claim); markErr != nil {
			return errors.Join(gerror.Wrap(failure, "open candidate for local scan"), markErr)
		}
		return gerror.Wrap(failure, "open candidate for local scan")
	}
	body := newCancellationReadCloser(ctx, object.Body)
	verdict, scanErr := w.scanner.Scan(ctx, body, claim.SizeBytes)
	closeErr := body.Close()
	if closeErr != nil {
		scanErr = errors.Join(scanErr, closeErr)
	}
	if scanErr != nil {
		if markErr := w.finishFailed(ctx, claim); markErr != nil {
			return errors.Join(gerror.Wrap(scanErr, "scan candidate with local clamd"), markErr)
		}
		return gerror.Wrap(scanErr, "scan candidate with local clamd")
	}
	switch verdict {
	case ScanVerdictClean:
		return w.finishClean(ctx, claim)
	case ScanVerdictRisk:
		return w.finishRisk(ctx, claim)
	default:
		if markErr := w.finishFailed(ctx, claim); markErr != nil {
			return markErr
		}
		return gerror.New("local clamd returned an unknown verdict")
	}
}

func (w *scanWorker) finishClean(ctx context.Context, claim *scanClaim) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := contentDatabaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		changed, err := w.finishAttempt(ctx, claim, "passed", "", now)
		if err != nil {
			return err
		}
		if !changed {
			return gerror.New("content scan completion lost its attempt lease fence")
		}
		result, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET status_code='ready',scan_status='passed',gate_completed_at=?,retention_expires_at=?,version=version+1,updated_at=? WHERE tenant_id=0 AND file_no=? AND candidate_generation=? AND role_code='candidate' AND status_code='checking' AND scan_status='scanning' AND object_ref IS NOT NULL AND (retention_expires_at IS NULL OR retention_expires_at>?)`, now, now.Add(cleanCandidateRetention), now, claim.FileNo, claim.CandidateGeneration, now)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil || rows != 1 {
			return gerror.New("content scan result lost its candidate generation fence")
		}
		return nil
	})
}

func (w *scanWorker) finishRisk(ctx context.Context, claim *scanClaim) error {
	err := w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := contentDatabaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		changed, err := w.finishAttempt(ctx, claim, "risk_found", "malware_detected", now)
		if err != nil {
			return err
		}
		if !changed {
			return gerror.New("content risk result lost its attempt lease fence")
		}
		result, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET status_code='risk_rejected',scan_status='risk_found',gate_completed_at=?,retention_expires_at=NULL,object_ref=NULL,version=version+1,updated_at=? WHERE tenant_id=0 AND file_no=? AND candidate_generation=? AND role_code='candidate' AND status_code='checking' AND scan_status='scanning' AND object_ref IS NOT NULL`, now, now, claim.FileNo, claim.CandidateGeneration)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil || rows != 1 {
			return gerror.New("content risk result lost its candidate generation fence")
		}
		return nil
	})
	if err != nil {
		return gerror.Wrap(err, "persist content risk result")
	}
	if err = w.storage.Delete(ctx, storagecap.DeleteInput{Path: claim.ObjectRef}); err != nil {
		return gerror.Wrap(err, "delete rejected candidate object")
	}
	return w.markObjectDeleted(ctx, claim.FileNo)
}

func (w *scanWorker) finishFailed(ctx context.Context, claim *scanClaim) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := contentDatabaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		changed, err := w.finishAttempt(ctx, claim, "failed", "", now)
		if err != nil {
			return err
		}
		if !changed {
			return gerror.New("content scan failure lost its attempt lease fence")
		}
		_, err = w.database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET scan_status='failed',version=version+1,updated_at=? WHERE tenant_id=0 AND file_no=? AND candidate_generation=? AND role_code='candidate' AND status_code='checking' AND scan_status='scanning'`, now, claim.FileNo, claim.CandidateGeneration)
		return err
	})
}

func (w *scanWorker) finishAttempt(ctx context.Context, claim *scanClaim, status, risk string, now time.Time) (bool, error) {
	result, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_content_scan_attempt SET status_code=?,finished_at=?,lease_expires_at=NULL,risk_code=?,version=version+1 WHERE tenant_id=0 AND attempt_no=? AND file_no=? AND candidate_generation=? AND status_code='scanning' AND version=? AND lease_expires_at>=?`, status, now, contentNullable(risk), claim.AttemptNo, claim.FileNo, claim.CandidateGeneration, claim.AttemptVersion, now)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	return changed == 1, err
}

func (w *scanWorker) failExpiredLeases(ctx context.Context, limit int) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := contentDatabaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		var rows []struct {
			AttemptNo           string
			FileNo              string
			CandidateGeneration int64
		}
		if err = w.database.GetScan(ctx, &rows, `SELECT attempt_no,file_no,candidate_generation FROM plugin_idc_core_content_scan_attempt WHERE tenant_id=0 AND status_code='scanning' AND lease_expires_at<=? ORDER BY lease_expires_at,id FOR UPDATE SKIP LOCKED LIMIT ?`, now, limit); err != nil {
			return err
		}
		for _, row := range rows {
			if _, err = w.database.Exec(ctx, `UPDATE plugin_idc_core_content_scan_attempt SET status_code='failed',finished_at=?,lease_expires_at=NULL,version=version+1 WHERE tenant_id=0 AND attempt_no=? AND status_code='scanning'`, now, row.AttemptNo); err != nil {
				return err
			}
			if _, err = w.database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET scan_status='failed',version=version+1,updated_at=? WHERE tenant_id=0 AND file_no=? AND candidate_generation=? AND role_code='candidate' AND status_code='checking' AND scan_status='scanning'`, now, row.FileNo, row.CandidateGeneration); err != nil {
				return err
			}
		}
		return nil
	})
}

func (w *scanWorker) expireCandidates(ctx context.Context, limit int) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := contentDatabaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		var rows []struct {
			FileNo              string
			CandidateGeneration int64
		}
		if err = w.database.GetScan(ctx, &rows, `SELECT file_no,candidate_generation FROM plugin_idc_core_content_file WHERE tenant_id=0 AND role_code='candidate' AND status_code IN ('checking','ready') AND retention_expires_at IS NOT NULL AND retention_expires_at<=? ORDER BY retention_expires_at,id FOR UPDATE SKIP LOCKED LIMIT ?`, now, limit); err != nil {
			return err
		}
		for _, row := range rows {
			if _, err = w.database.Exec(ctx, `UPDATE plugin_idc_core_content_scan_attempt SET status_code='failed',started_at=COALESCE(started_at,?),finished_at=?,lease_expires_at=NULL,version=version+1 WHERE tenant_id=0 AND file_no=? AND candidate_generation=? AND status_code IN ('pending','scanning')`, now, now, row.FileNo, row.CandidateGeneration); err != nil {
				return err
			}
			if _, err = w.database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET status_code='expired',object_ref=NULL,version=version+1,updated_at=? WHERE tenant_id=0 AND file_no=? AND candidate_generation=? AND role_code='candidate' AND status_code IN ('checking','ready')`, now, row.FileNo, row.CandidateGeneration); err != nil {
				return err
			}
		}
		return nil
	})
}

func (w *scanWorker) deleteTerminalObjects(ctx context.Context, limit int) error {
	var rows []struct {
		FileNo, ObjectRef string
	}
	if err := w.database.GetScan(ctx, &rows, `SELECT f.file_no,u.temporary_object_ref AS object_ref FROM plugin_idc_core_content_file f JOIN plugin_idc_core_content_upload_session u ON u.tenant_id=f.tenant_id AND u.upload_no=f.upload_no WHERE f.tenant_id=0 AND f.status_code IN ('risk_rejected','cancelled','expired','replaced') AND f.object_deleted_at IS NULL ORDER BY f.id LIMIT ?`, limit); err != nil {
		return gerror.Wrap(err, "load content objects pending deletion")
	}
	var deleteErrors []error
	for _, row := range rows {
		if err := w.storage.Delete(ctx, storagecap.DeleteInput{Path: row.ObjectRef}); err != nil {
			deleteErrors = append(deleteErrors, gerror.Wrap(err, "delete terminal content object"))
			continue
		}
		if err := w.markObjectDeleted(ctx, row.FileNo); err != nil {
			deleteErrors = append(deleteErrors, err)
		}
	}
	return errors.Join(deleteErrors...)
}

func (w *scanWorker) markObjectDeleted(ctx context.Context, fileNo string) error {
	err := w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var rows []struct{ UploadNo string }
		if err := w.database.GetScan(ctx, &rows, `SELECT upload_no FROM plugin_idc_core_content_file WHERE tenant_id=0 AND file_no=? AND status_code IN ('risk_rejected','cancelled','expired','replaced') AND object_deleted_at IS NULL FOR UPDATE`, fileNo); err != nil {
			return err
		}
		if len(rows) != 1 {
			return gerror.New("terminal content object deletion lost its file fence")
		}
		if _, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_content_upload_session SET temporary_object_ref='',updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND upload_no=?`, rows[0].UploadNo); err != nil {
			return err
		}
		result, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_content_file SET object_ref=NULL,object_deleted_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND file_no=? AND status_code IN ('risk_rejected','cancelled','expired','replaced') AND object_deleted_at IS NULL`, fileNo)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return gerror.New("terminal content object deletion lost its update fence")
		}
		return nil
	})
	if err != nil {
		return gerror.Wrap(err, "mark terminal content object deleted")
	}
	return nil
}
