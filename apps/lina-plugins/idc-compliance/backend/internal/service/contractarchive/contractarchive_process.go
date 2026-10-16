package contractarchive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-core/backend/cap/idccap"
)

var safeReasonPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,63}$`)

func (s *serviceImpl) archivePDF(
	ctx context.Context,
	job *claimedJob,
	artifact *localgatewayclient.TencentContractArtifact,
) error {
	expectedKind := job.ArtifactType
	if expectedKind == "termination_pdf" {
		expectedKind = "final_pdf"
	}
	if artifact == nil || len(artifact.Body) < 5 || string(artifact.Body[:5]) != "%PDF-" ||
		artifact.Kind != expectedKind || artifact.SHA256 != sha256Hex(artifact.Body) ||
		artifact.SupplierObservedAt.IsZero() {
		return s.reschedule(ctx, job, "artifact_integrity_invalid", time.Time{})
	}
	path := contractObjectPath(job.ContractNo, job.FlowNo, job.ArtifactType)
	confirmed, err := s.confirmObject(ctx, path, artifact.Body, artifact.SHA256)
	if err != nil {
		return s.reschedule(ctx, job, "private_storage_unavailable", time.Time{})
	}
	if !confirmed {
		if _, err = s.storage.Put(ctx, storagecap.PutInput{
			Path: path, Body: bytes.NewReader(artifact.Body), Size: int64(len(artifact.Body)),
			ContentType: "application/pdf", Overwrite: false,
		}); err != nil {
			confirmed, confirmErr := s.confirmObject(ctx, path, artifact.Body, artifact.SHA256)
			if confirmErr != nil || !confirmed {
				return s.reschedule(ctx, job, "private_storage_unavailable", time.Time{})
			}
		}
		confirmed, err = s.confirmObject(ctx, path, artifact.Body, artifact.SHA256)
		if err != nil || !confirmed {
			return s.reschedule(ctx, job, "private_storage_unavailable", time.Time{})
		}
	}
	return s.finalizePDF(ctx, job, path, artifact)
}

func (s *serviceImpl) confirmObject(
	ctx context.Context,
	path string,
	expected []byte,
	digest string,
) (bool, error) {
	result, err := s.storage.Get(ctx, storagecap.GetInput{Path: path})
	if err != nil {
		return false, err
	}
	if result == nil || !result.Found {
		return false, nil
	}
	if result.Object == nil || result.Body == nil || result.Object.Path != path ||
		result.Object.Visibility != storagecap.VisibilityPrivate ||
		result.Object.Size != int64(len(expected)) || result.Object.ContentType != "application/pdf" {
		if result.Body != nil {
			if closeErr := result.Body.Close(); closeErr != nil {
				return false, gerror.Wrap(closeErr, "close inconsistent contract archive object")
			}
		}
		return false, gerror.New("contract archive private object metadata is inconsistent")
	}
	actual, readErr := io.ReadAll(io.LimitReader(result.Body, int64(len(expected))+1))
	closeErr := result.Body.Close()
	defer clear(actual)
	if readErr != nil || closeErr != nil || len(actual) != len(expected) ||
		subtle.ConstantTimeCompare(actual, expected) != 1 || sha256Hex(actual) != digest {
		return false, gerror.New("contract archive private object integrity check failed")
	}
	return true, nil
}

func (s *serviceImpl) finalizePDF(
	ctx context.Context,
	job *claimedJob,
	path string,
	artifact *localgatewayclient.TencentContractArtifact,
) error {
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		var existing *struct {
			FileNo             string     `orm:"file_no"`
			PrivateObjectRef   string     `orm:"private_object_ref"`
			ContentSHA256      string     `orm:"content_sha256"`
			MediaType          string     `orm:"media_type"`
			ByteSize           int64      `orm:"byte_size"`
			StatusCode         string     `orm:"status_code"`
			Immutable          bool       `orm:"immutable"`
			SupplierObservedAt *time.Time `orm:"supplier_observed_at"`
		}
		if err = tx.GetScan(&existing, `
SELECT file_no, private_object_ref, content_sha256, media_type, byte_size,
       status_code, immutable, supplier_observed_at
FROM plugin_idc_compliance_contract_file
WHERE tenant_id = ? AND flow_no = ? AND file_type_code = ? AND status_code = 'archived'
LIMIT 2`, idccap.DefaultTenantID, job.FlowNo, job.ArtifactType); err != nil {
			return gerror.Wrap(err, "read archived contract file")
		}
		fileNo := contractFileNo(job.FlowNo, job.ArtifactType, artifact.SHA256)
		if existing == nil {
			if _, err = tx.Exec(`
INSERT INTO plugin_idc_compliance_contract_file
    (tenant_id, file_no, contract_no, flow_no, file_type_code,
     private_object_ref, content_sha256, media_type, byte_size, status_code,
     immutable, archived_at, supplier_observed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 'application/pdf', ?, 'archived', TRUE, ?, ?)`,
				idccap.DefaultTenantID, fileNo, job.ContractNo, job.FlowNo, job.ArtifactType,
				path, artifact.SHA256, len(artifact.Body), now, artifact.SupplierObservedAt.UTC(),
			); err != nil {
				return gerror.Wrap(err, "insert immutable contract archive file")
			}
		} else if existing.FileNo != fileNo || existing.PrivateObjectRef != path ||
			existing.ContentSHA256 != artifact.SHA256 || existing.MediaType != "application/pdf" ||
			existing.ByteSize != int64(len(artifact.Body)) || existing.StatusCode != "archived" ||
			!existing.Immutable || existing.SupplierObservedAt == nil {
			return gerror.New("existing contract archive file conflicts with verified content")
		}
		if err = s.completeJob(
			ctx, tx, job, "archived", "artifact_archived", artifact.SupplierObservedAt.UTC(), now,
		); err != nil {
			return err
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract.archive",
			IdempotencyKey:   job.JobNo, OperationCode: "compliance.contract.archive-file",
			TargetType: "contract", TargetNo: job.ContractNo, CustomerNo: job.CustomerNo,
			ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: "artifact_archived", RiskCode: "high",
			CorrelationNo:  job.FlowNo,
			BeforeSnapshot: map[string]any{"archive_status": "processing"},
			AfterSnapshot: map[string]any{
				"archive_status": "archived", "file_type": job.ArtifactType,
				"content_sha256": artifact.SHA256, "byte_size": len(artifact.Body),
				"supplier_observed_at": artifact.SupplierObservedAt.UTC(),
			},
		})
	})
}

func (s *serviceImpl) archiveBlockchainMetadata(ctx context.Context, job *claimedJob) error {
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		occurredAt := now
		if job.SupplierObservedAt != nil && !job.SupplierObservedAt.IsZero() {
			occurredAt = job.SupplierObservedAt.UTC()
		}
		payloadDigest := sha256.Sum256([]byte(
			"tencent-blockchain-short-lived-only/v1\x00" + job.GatewayRouteNo,
		))
		sourceRef := job.GatewayRouteNo + ":blockchain-short-lived-only"
		if _, err = tx.Exec(`
INSERT INTO plugin_idc_compliance_external_evidence
    (tenant_id, evidence_no, aggregate_type_code, aggregate_no,
     source_system_code, source_event_ref, evidence_type_code, payload_sha256,
     disposition_code, occurred_at, received_at)
VALUES (?, ?, 'contract', ?, 'tencent_essbasic', ?,
        'blockchain_short_lived_metadata', ?, 'accepted', ?, ?)
ON CONFLICT (tenant_id, source_system_code, source_event_ref) DO NOTHING`,
			idccap.DefaultTenantID, blockchainEvidenceNo(job.FlowNo), job.ContractNo,
			sourceRef, hex.EncodeToString(payloadDigest[:]), occurredAt, now,
		); err != nil {
			return gerror.Wrap(err, "append Tencent blockchain limitation metadata")
		}
		var stored struct {
			EvidenceNo       string  `orm:"evidence_no"`
			AggregateNo      string  `orm:"aggregate_no"`
			EvidenceTypeCode string  `orm:"evidence_type_code"`
			PayloadSHA256    string  `orm:"payload_sha256"`
			PrivateObjectRef *string `orm:"private_object_ref"`
		}
		if err = tx.GetScan(&stored, `
SELECT evidence_no, aggregate_no, evidence_type_code, payload_sha256, private_object_ref
FROM plugin_idc_compliance_external_evidence
WHERE tenant_id = ? AND source_system_code = 'tencent_essbasic' AND source_event_ref = ?`,
			idccap.DefaultTenantID, sourceRef,
		); err != nil {
			return gerror.Wrap(err, "verify Tencent blockchain limitation metadata")
		}
		if stored.EvidenceNo != blockchainEvidenceNo(job.FlowNo) || stored.AggregateNo != job.ContractNo ||
			stored.EvidenceTypeCode != "blockchain_short_lived_metadata" ||
			stored.PayloadSHA256 != hex.EncodeToString(payloadDigest[:]) || stored.PrivateObjectRef != nil {
			return gerror.New("Tencent blockchain limitation metadata conflicts with stored evidence")
		}
		if err = s.completeJob(ctx, tx, job, "archived", "short_lived_metadata_archived", occurredAt, now); err != nil {
			return err
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract.archive",
			IdempotencyKey:   job.JobNo, OperationCode: "compliance.contract.archive-evidence-metadata",
			TargetType: "contract", TargetNo: job.ContractNo, CustomerNo: job.CustomerNo,
			ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: "short_lived_metadata_archived", RiskCode: "high",
			CorrelationNo:  job.FlowNo,
			BeforeSnapshot: map[string]any{"archive_status": "processing"},
			AfterSnapshot: map[string]any{
				"archive_status": "archived", "evidence_type": "blockchain_short_lived_metadata",
				"durable_file_available": false, "temporary_link_persisted": false,
			},
		})
	})
}

func (s *serviceImpl) finishWithoutFile(
	ctx context.Context,
	job *claimedJob,
	status string,
	reason string,
) error {
	reason = safeReasonCode(reason)
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		if err = s.completeJob(ctx, tx, job, status, reason, time.Time{}, now); err != nil {
			return err
		}
		auditResult := "failed"
		if status == "result_unknown" {
			auditResult = "result_unknown"
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract.archive",
			IdempotencyKey:   job.JobNo, OperationCode: "compliance.contract.archive-file",
			TargetType: "contract", TargetNo: job.ContractNo, CustomerNo: job.CustomerNo,
			ResultCode: auditResult, ReasonCode: reason, RiskCode: "high", CorrelationNo: job.FlowNo,
			BeforeSnapshot: map[string]any{"archive_status": "processing"},
			AfterSnapshot: map[string]any{
				"archive_status": status, "file_type": job.ArtifactType, "reason": reason,
			},
		})
	})
}

func (s *serviceImpl) completeJob(
	ctx context.Context,
	tx gdb.TX,
	job *claimedJob,
	status string,
	reason string,
	supplierObservedAt time.Time,
	now time.Time,
) error {
	var observed any
	if !supplierObservedAt.IsZero() {
		observed = supplierObservedAt.UTC()
	}
	result, err := tx.Exec(`
UPDATE plugin_idc_compliance_contract_archive_job
SET status_code = ?, next_attempt_at = NULL, lease_sha256 = NULL,
    lease_expires_at = NULL, last_reason_code = ?,
    supplier_observed_at = COALESCE(?, supplier_observed_at),
    completed_at = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND job_no = ? AND status_code = 'processing'
  AND lease_sha256 = ? AND lease_expires_at > ? AND version = ?`,
		status, reason, observed, now, now, idccap.DefaultTenantID, job.JobNo,
		job.LeaseSHA256, now, job.Version,
	)
	if err != nil {
		return gerror.Wrap(err, "complete contract archive job")
	}
	affected, affectedErr := result.RowsAffected()
	if affectedErr != nil {
		return gerror.Wrap(affectedErr, "read completed contract archive row count")
	}
	if affected != 1 {
		return gerror.New("contract archive completion lease was lost")
	}
	return nil
}

func (s *serviceImpl) reschedule(
	ctx context.Context,
	job *claimedJob,
	reason string,
	next time.Time,
) error {
	reason = safeReasonCode(reason)
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		minimum := now.Add(retryDelay(job.AttemptCount))
		if next.IsZero() || next.Before(minimum) {
			next = minimum
		}
		if next.After(now.Add(7 * 24 * time.Hour)) {
			next = now.Add(7 * 24 * time.Hour)
		}
		result, err := tx.Exec(`
UPDATE plugin_idc_compliance_contract_archive_job
SET status_code = 'pending', next_attempt_at = ?, lease_sha256 = NULL,
    lease_expires_at = NULL, last_reason_code = ?, version = version + 1,
    updated_at = ?
WHERE tenant_id = ? AND job_no = ? AND status_code = 'processing'
  AND lease_sha256 = ? AND lease_expires_at > ? AND version = ?`,
			next.UTC(), reason, now, idccap.DefaultTenantID, job.JobNo,
			job.LeaseSHA256, now, job.Version,
		)
		if err != nil {
			return gerror.Wrap(err, "reschedule contract archive job")
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return gerror.Wrap(affectedErr, "read rescheduled contract archive row count")
		}
		if affected != 1 {
			return gerror.New("contract archive reschedule lease was lost")
		}
		return nil
	})
}

func retryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	case 3:
		return 30 * time.Minute
	default:
		return time.Hour
	}
}

func safeReason(err error) string {
	var gatewayError *localgatewayclient.TencentContractFlowError
	if errors.As(err, &gatewayError) {
		return safeReasonCode(gatewayError.Code)
	}
	return "artifact_unavailable"
}

func safeReasonCode(code string) string {
	if safeReasonPattern.MatchString(code) {
		return code
	}
	return "artifact_unavailable"
}

func contractObjectPath(contractNo, flowNo, artifactType string) string {
	return "contracts/" + contractNo + "/flows/" + flowNo + "/" + artifactType + ".pdf"
}

func contractFileNo(flowNo, artifactType, digest string) string {
	value := sha256.Sum256([]byte(
		"contract-file/v1\x00" + flowNo + "\x00" + artifactType + "\x00" + digest,
	))
	return "cfi_" + hex.EncodeToString(value[:16])
}

func blockchainEvidenceNo(flowNo string) string {
	value := sha256.Sum256([]byte("contract-blockchain-metadata/v1\x00" + flowNo))
	return "cev_" + hex.EncodeToString(value[:16])
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
