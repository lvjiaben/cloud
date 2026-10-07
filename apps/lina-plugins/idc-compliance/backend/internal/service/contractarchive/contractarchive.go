// Package contractarchive runs the managed, installation-local contract archive queue.
package contractarchive

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"reflect"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	MaximumBatchSize = 8
	leaseDuration    = 2 * time.Minute
	maximumAttempts  = 1000
	providerCode     = "tencent_essbasic"
)

var (
	archiveRoutePattern = regexp.MustCompile(`^lwr_[0-9a-f]{32}$`)
	archiveJobPattern   = regexp.MustCompile(`^caj_[0-9a-f]{32}$`)
	archiveNamePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
)

// Gateway is the narrow loopback seam used by the archive worker.
type Gateway interface {
	FetchTencentContractFinalPDF(context.Context, string) (*localgatewayclient.TencentContractArtifact, error)
	EnsureTencentContractSignReport(
		context.Context,
		string,
		string,
	) (*localgatewayclient.TencentContractArtifact, localgatewayclient.TencentContractArtifactState, error)
}

// RunResult is one bounded managed-job pass.
type RunResult struct {
	Enqueued  int
	Processed int
}

// Service owns the private object archive queue.
type Service interface {
	Run(context.Context, int) (RunResult, error)
}

type serviceImpl struct {
	database gdb.DB
	gateway  Gateway
	storage  storagecap.Service
	audit    idccap.BusinessAuditWriter
	random   io.Reader
}

var _ Service = (*serviceImpl)(nil)

// New constructs the primary-node archive worker.
func New(
	database gdb.DB,
	gateway Gateway,
	storage storagecap.Service,
	audit idccap.BusinessAuditWriter,
) (Service, error) {
	if nilDependency(database) || nilDependency(gateway) || nilDependency(storage) || nilDependency(audit) {
		return nil, gerror.New("contract archive dependencies are invalid")
	}
	return &serviceImpl{
		database: database, gateway: gateway, storage: storage, audit: audit, random: rand.Reader,
	}, nil
}

// Run enqueues missing terminal artifacts and processes a bounded set.
func (s *serviceImpl) Run(ctx context.Context, limit int) (RunResult, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || limit < 1 || limit > MaximumBatchSize {
		return RunResult{}, gerror.New("contract archive batch is invalid")
	}
	enqueued, err := s.enqueueCompleted(ctx, limit)
	if err != nil {
		return RunResult{}, err
	}
	result := RunResult{Enqueued: enqueued}
	for result.Processed < limit {
		job, claimErr := s.claimOne(ctx)
		if claimErr != nil {
			return result, claimErr
		}
		if job == nil {
			break
		}
		if processErr := s.process(ctx, job); processErr != nil {
			return result, processErr
		}
		result.Processed++
	}
	return result, nil
}

type completedFlow struct {
	ContractNo       string    `orm:"contract_no"`
	FlowNo           string    `orm:"flow_no"`
	PurposeCode      string    `orm:"purpose_code"`
	GatewayRouteNo   string    `orm:"gateway_route_no"`
	SupplierObserved time.Time `orm:"supplier_observed_at"`
}

func (s *serviceImpl) enqueueCompleted(ctx context.Context, limit int) (int, error) {
	var rows []completedFlow
	err := s.database.GetScan(ctx, &rows, `
SELECT f.contract_no, f.flow_no, f.purpose_code, f.gateway_route_no,
       f.completed_at AS supplier_observed_at
FROM plugin_idc_compliance_contract_flow AS f
WHERE f.tenant_id = ? AND f.provider_code = ? AND f.status_code = 'completed'
  AND f.gateway_route_no IS NOT NULL
  AND EXISTS (
      SELECT 1
      FROM (VALUES ('final_pdf'), ('sign_report'), ('termination_pdf'), ('blockchain_metadata')) AS expected(type_code)
      WHERE (
          (f.purpose_code = 'termination' AND expected.type_code IN ('termination_pdf', 'blockchain_metadata'))
          OR (f.purpose_code <> 'termination' AND expected.type_code IN ('final_pdf', 'sign_report', 'blockchain_metadata'))
      ) AND NOT EXISTS (
          SELECT 1
          FROM plugin_idc_compliance_contract_archive_job AS j
          WHERE j.tenant_id = f.tenant_id AND j.flow_no = f.flow_no
            AND j.artifact_type_code = expected.type_code
      )
  )
ORDER BY f.completed_at, f.id
LIMIT ?`, idccap.DefaultTenantID, providerCode, limit)
	if err != nil {
		return 0, gerror.Wrap(err, "list completed contract flows for archive")
	}
	enqueued := 0
	for _, row := range rows {
		for _, artifactType := range artifactTypes(row.PurposeCode) {
			result, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_archive_job
    (tenant_id, job_no, contract_no, flow_no, gateway_route_no,
     artifact_type_code, status_code, next_attempt_at, last_reason_code,
     supplier_observed_at)
VALUES (?, ?, ?, ?, ?, ?, 'queued', CURRENT_TIMESTAMP, 'archive_queued', ?)
ON CONFLICT (tenant_id, flow_no, artifact_type_code) DO NOTHING`,
				idccap.DefaultTenantID, archiveJobNo(row.FlowNo, artifactType), row.ContractNo,
				row.FlowNo, row.GatewayRouteNo, artifactType, row.SupplierObserved,
			)
			if insertErr != nil {
				return enqueued, gerror.Wrap(insertErr, "enqueue completed contract artifact")
			}
			affected, affectedErr := result.RowsAffected()
			if affectedErr != nil {
				return enqueued, gerror.Wrap(affectedErr, "read enqueued contract artifact count")
			}
			if affected == 1 {
				enqueued++
			}
		}
	}
	return enqueued, nil
}

func artifactTypes(purpose string) []string {
	if purpose == "termination" {
		return []string{"termination_pdf", "blockchain_metadata"}
	}
	return []string{"final_pdf", "sign_report", "blockchain_metadata"}
}

type claimedJob struct {
	ID                 int64      `orm:"id"`
	JobNo              string     `orm:"job_no"`
	ContractNo         string     `orm:"contract_no"`
	CustomerNo         string     `orm:"customer_no"`
	FlowNo             string     `orm:"flow_no"`
	GatewayRouteNo     string     `orm:"gateway_route_no"`
	ArtifactType       string     `orm:"artifact_type_code"`
	AttemptCount       int        `orm:"attempt_count"`
	LeaseSHA256        string     `orm:"lease_sha256"`
	LeaseExpiresAt     time.Time  `orm:"lease_expires_at"`
	SupplierObservedAt *time.Time `orm:"supplier_observed_at"`
	Version            int64      `orm:"version"`
}

func (s *serviceImpl) claimOne(ctx context.Context) (*claimedJob, error) {
	var claimed *claimedJob
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`
WITH expired AS (
    SELECT id
    FROM plugin_idc_compliance_contract_archive_job
    WHERE tenant_id = ? AND status_code = 'processing' AND lease_expires_at <= ?
    ORDER BY lease_expires_at, id
    LIMIT 32 FOR UPDATE SKIP LOCKED
)
UPDATE plugin_idc_compliance_contract_archive_job
SET status_code = 'pending', next_attempt_at = ?, lease_sha256 = NULL,
    lease_expires_at = NULL, last_reason_code = 'archive_lease_expired',
    version = version + 1, updated_at = ?
WHERE id IN (SELECT id FROM expired)`, idccap.DefaultTenantID, now, now, now); err != nil {
			return gerror.Wrap(err, "recover expired contract archive leases")
		}
		var row *struct {
			ID                 int64      `orm:"id"`
			JobNo              string     `orm:"job_no"`
			ContractNo         string     `orm:"contract_no"`
			CustomerNo         string     `orm:"customer_no"`
			FlowNo             string     `orm:"flow_no"`
			GatewayRouteNo     string     `orm:"gateway_route_no"`
			ArtifactType       string     `orm:"artifact_type_code"`
			AttemptCount       int        `orm:"attempt_count"`
			SupplierObservedAt *time.Time `orm:"supplier_observed_at"`
			Version            int64      `orm:"version"`
		}
		if err = tx.GetScan(&row, `
SELECT j.id, j.job_no, j.contract_no, c.customer_no, j.flow_no,
       j.gateway_route_no, j.artifact_type_code, j.attempt_count,
       j.supplier_observed_at, j.version
FROM plugin_idc_compliance_contract_archive_job AS j
JOIN plugin_idc_compliance_contract AS c
  ON c.tenant_id = j.tenant_id AND c.contract_no = j.contract_no
WHERE j.tenant_id = ? AND j.status_code IN ('queued', 'pending')
  AND COALESCE(j.next_attempt_at, ?) <= ?
ORDER BY COALESCE(j.next_attempt_at, j.created_at), j.id
		LIMIT 1 FOR UPDATE OF j SKIP LOCKED`, idccap.DefaultTenantID, now, now); err != nil || row == nil {
			return err
		}
		if !archiveJobPattern.MatchString(row.JobNo) ||
			!archiveNamePattern.MatchString(row.ContractNo) ||
			!archiveNamePattern.MatchString(row.CustomerNo) ||
			!archiveNamePattern.MatchString(row.FlowNo) ||
			!archiveRoutePattern.MatchString(row.GatewayRouteNo) ||
			!validArtifactType(row.ArtifactType) {
			return gerror.New("contract archive job contains invalid protected identifiers")
		}
		leaseBytes := make([]byte, 32)
		if _, err = io.ReadFull(s.random, leaseBytes); err != nil {
			clear(leaseBytes)
			return gerror.Wrap(err, "generate contract archive lease")
		}
		digest := sha256.Sum256(leaseBytes)
		clear(leaseBytes)
		leaseSHA := hex.EncodeToString(digest[:])
		leaseExpiresAt := now.Add(leaseDuration)
		result, err := tx.Exec(`
UPDATE plugin_idc_compliance_contract_archive_job
SET status_code = 'processing', attempt_count = LEAST(attempt_count + 1, ?),
    lease_sha256 = ?, lease_expires_at = ?, next_attempt_at = NULL,
    last_reason_code = 'archive_processing', version = version + 1, updated_at = ?
WHERE id = ? AND tenant_id = ? AND version = ? AND status_code IN ('queued', 'pending')`,
			maximumAttempts, leaseSHA, leaseExpiresAt, now, row.ID,
			idccap.DefaultTenantID, row.Version,
		)
		if err != nil {
			return gerror.Wrap(err, "claim contract archive job")
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return gerror.Wrap(affectedErr, "read claimed contract archive row count")
		}
		if affected != 1 {
			return gerror.New("contract archive job lease was lost")
		}
		claimed = &claimedJob{
			ID: row.ID, JobNo: row.JobNo, ContractNo: row.ContractNo,
			CustomerNo: row.CustomerNo, FlowNo: row.FlowNo, GatewayRouteNo: row.GatewayRouteNo,
			ArtifactType: row.ArtifactType, AttemptCount: min(row.AttemptCount+1, maximumAttempts),
			LeaseSHA256: leaseSHA, LeaseExpiresAt: leaseExpiresAt,
			SupplierObservedAt: row.SupplierObservedAt, Version: row.Version + 1,
		}
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "claim contract archive job")
	}
	return claimed, nil
}

func validArtifactType(value string) bool {
	switch value {
	case "final_pdf", "sign_report", "termination_pdf", "blockchain_metadata":
		return true
	default:
		return false
	}
}

func (s *serviceImpl) process(ctx context.Context, job *claimedJob) error {
	switch job.ArtifactType {
	case "blockchain_metadata":
		return s.archiveBlockchainMetadata(ctx, job)
	case "final_pdf", "termination_pdf":
		artifact, err := s.gateway.FetchTencentContractFinalPDF(ctx, job.GatewayRouteNo)
		if err != nil {
			return s.reschedule(ctx, job, safeReason(err), time.Time{})
		}
		if artifact == nil {
			return s.reschedule(ctx, job, "artifact_unavailable", time.Time{})
		}
		defer clear(artifact.Body)
		return s.archivePDF(ctx, job, artifact)
	case "sign_report":
		artifact, state, err := s.gateway.EnsureTencentContractSignReport(
			ctx, job.GatewayRouteNo, reportBusinessKey(job.JobNo),
		)
		if err != nil {
			return s.reschedule(ctx, job, safeReason(err), time.Time{})
		}
		if artifact != nil {
			defer clear(artifact.Body)
			return s.archivePDF(ctx, job, artifact)
		}
		switch state.State {
		case "pending":
			return s.reschedule(ctx, job, safeReasonCode(state.Code), state.NextCheckAt)
		case "skipped":
			return s.finishWithoutFile(ctx, job, "skipped", safeReasonCode(state.Code))
		case "result_unknown":
			return s.finishWithoutFile(ctx, job, "result_unknown", safeReasonCode(state.Code))
		default:
			return s.reschedule(ctx, job, "artifact_unavailable", time.Time{})
		}
	default:
		return s.finishWithoutFile(ctx, job, "result_unknown", "artifact_type_invalid")
	}
}

func archiveJobNo(flowNo, artifactType string) string {
	digest := sha256.Sum256([]byte("contract-archive-job/v1\x00" + flowNo + "\x00" + artifactType))
	return "caj_" + hex.EncodeToString(digest[:16])
}

func reportBusinessKey(jobNo string) string {
	digest := sha256.Sum256([]byte("contract-sign-report/v1\x00" + jobNo))
	return "lbi1_" + base64.RawURLEncoding.EncodeToString(digest[:])
}

func databaseTime(tx gdb.TX) (time.Time, error) {
	value, err := tx.GetValue("SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "read contract archive database time")
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, gerror.New("contract archive database time is unavailable")
	}
	return now.UTC(), nil
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
