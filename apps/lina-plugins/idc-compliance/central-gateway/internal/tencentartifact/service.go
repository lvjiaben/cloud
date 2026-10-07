// Package tencentartifact archives Tencent contract artifacts without ever
// persisting or exposing Tencent's short-lived download URLs.
package tencentartifact

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"time"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
	"lina-platform/idc-compliance-gateway/internal/suppliercallback"
	"lina-platform/idc-compliance-gateway/internal/tencentess"
)

const (
	providerCode       = suppliercallback.ProviderTencentESSBasic
	initialReportDelay = 24 * time.Hour
	reportRetryDelay   = time.Hour
)

var (
	ErrInvalid  = errors.New("tencent contract artifact input is invalid")
	ErrConflict = errors.New("tencent contract artifact state conflicts with the request")
	ErrRejected = errors.New("tencent contract artifact operation was rejected")

	routePattern   = regexp.MustCompile(`^lwr_[0-9a-f]{32}$`)
	requestPattern = regexp.MustCompile(`^lbr_[0-9a-f]{32}$`)
)

// State is the safe local result returned to the plugin process.
type State string

const (
	StateReady         State = "ready"
	StatePending       State = "pending"
	StateSkipped       State = "skipped"
	StateResultUnknown State = "result_unknown"
)

// Result contains no Tencent URL, report identifier, credential, or free text.
type Result struct {
	State       State
	Code        string
	NextCheckAt time.Time
	Document    *tencentess.ArtifactDocument
}

type tencentClient interface {
	HasCapability(tencentess.Capability) bool
	FetchFinalPDF(context.Context, string) (*tencentess.ArtifactDocument, error)
	CreateEvidenceReport(context.Context, string, string) (tencentess.EvidenceReportCreateResult, error)
	FetchEvidenceReport(context.Context, string) (tencentess.EvidenceReportResult, error)
}

// Service owns final-PDF reads and one durable paid-report request per flow.
type Service struct {
	database       *sql.DB
	installationID string
	keyring        *callbacksecurity.Ring
	reconciliation *suppliercallback.QueryService
	client         tencentClient
	random         io.Reader
	now            func() time.Time
}

// New constructs one installation-local artifact service.
func New(
	database *sql.DB,
	installationID string,
	keyring *callbacksecurity.Ring,
	reconciliation *suppliercallback.QueryService,
	client tencentClient,
) (*Service, error) {
	if database == nil || installationID == "" || keyring == nil || reconciliation == nil || client == nil ||
		keyring.ActiveReferenceVersion() == 0 {
		return nil, ErrInvalid
	}
	return &Service{
		database: database, installationID: installationID, keyring: keyring,
		reconciliation: reconciliation, client: client, random: rand.Reader,
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

// FinalPDF reads a terminal agreement PDF through an exact locally verified route.
func (s *Service) FinalPDF(ctx context.Context, routeNo string) (*tencentess.ArtifactDocument, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !routePattern.MatchString(routeNo) {
		return nil, ErrInvalid
	}
	var document *tencentess.ArtifactDocument
	err := s.reconciliation.WithTerminalReference(
		ctx, routeNo, providerCode,
		[]suppliercallback.TerminalReferenceRule{
			{OperationScope: tencentess.OperationFlowCreate, ProviderStatus: "ALL"},
			{OperationScope: tencentess.OperationFlowRelease, ProviderStatus: "RELIEVED"},
		},
		func(reference []byte) error {
			var fetchErr error
			document, fetchErr = s.client.FetchFinalPDF(ctx, string(reference))
			return fetchErr
		},
	)
	if err != nil {
		return nil, err
	}
	if document == nil || len(document.Content) == 0 {
		return nil, ErrRejected
	}
	return document, nil
}

// EnsureSignReport creates at most one paid report and polls only its encrypted identifier.
func (s *Service) EnsureSignReport(
	ctx context.Context,
	sourceRouteNo string,
	requestNo string,
) (Result, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !routePattern.MatchString(sourceRouteNo) ||
		!requestPattern.MatchString(requestNo) {
		return Result{}, ErrInvalid
	}
	if !s.client.HasCapability(tencentess.CapabilityEvidenceReport) {
		return Result{State: StateSkipped, Code: "report_not_enabled"}, nil
	}
	job, err := s.loadJob(ctx, sourceRouteNo)
	if err == nil {
		return s.resumeReport(ctx, job)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrRejected
	}
	var created tencentess.EvidenceReportCreateResult
	err = s.reconciliation.WithCompletedReference(
		ctx, sourceRouteNo, providerCode, tencentess.OperationFlowCreate,
		func(reference []byte) error {
			var createErr error
			created, createErr = s.client.CreateEvidenceReport(ctx, requestNo, string(reference))
			return createErr
		},
	)
	if err != nil {
		if created.RouteNo == "" {
			if code, ok := nonChargeableSkip(err); ok {
				return Result{State: StateSkipped, Code: code}, nil
			}
			return Result{}, err
		}
		return s.persistCreateFailure(ctx, sourceRouteNo, created.RouteNo, err)
	}
	if created.RouteNo == "" || created.ReportID == "" || created.Recovered {
		if created.RouteNo == "" {
			return Result{}, ErrRejected
		}
		return s.persistCreateFailure(
			ctx, sourceRouteNo, created.RouteNo,
			&tencentess.Error{Code: tencentess.ErrorResultUnknown},
		)
	}
	job, err = s.persistCreatedReport(ctx, sourceRouteNo, created)
	if err != nil {
		return Result{}, err
	}
	return s.resumeReport(ctx, job)
}

type reportJob struct {
	JobNo            string
	SourceRouteNo    string
	ReportRouteNo    string
	Status           string
	Reason           string
	NextQueryAt      sql.NullTime
	CipherFormat     sql.NullInt16
	EncryptionKey    sql.NullInt32
	Nonce            []byte
	Ciphertext       []byte
	SupplierObserved sql.NullTime
	Version          int64
}

func (s *Service) loadJob(ctx context.Context, sourceRouteNo string) (reportJob, error) {
	var job reportJob
	err := s.database.QueryRowContext(ctx, `
SELECT job_no, source_route_no, report_route_no, status_code, last_reason_code,
       next_query_at, report_cipher_format_version, report_encryption_key_version,
       report_nonce, report_ciphertext, supplier_observed_at, version
FROM idc_gateway_tencent_contract_report_job
WHERE installation_id = $1 AND source_route_no = $2`,
		s.installationID, sourceRouteNo).Scan(
		&job.JobNo, &job.SourceRouteNo, &job.ReportRouteNo, &job.Status, &job.Reason,
		&job.NextQueryAt, &job.CipherFormat, &job.EncryptionKey, &job.Nonce,
		&job.Ciphertext, &job.SupplierObserved, &job.Version,
	)
	return job, err
}

func (s *Service) persistCreatedReport(
	ctx context.Context,
	sourceRouteNo string,
	created tencentess.EvidenceReportCreateResult,
) (reportJob, error) {
	jobNo, err := randomIdentifier(s.random, "lrj_")
	if err != nil {
		return reportJob{}, ErrRejected
	}
	sealed, err := s.keyring.SealReference(
		s.random, created.RouteNo, providerCode, []byte(created.ReportID),
	)
	if err != nil {
		return reportJob{}, ErrRejected
	}
	defer clear(sealed.Nonce)
	defer clear(sealed.Ciphertext)
	status := "executing"
	reason := "report_executing"
	var nextQueryAt any = s.now().Add(initialReportDelay)
	if created.Status == "EvidenceStatusSuccess" {
		status = "ready"
		reason = "report_ready"
		nextQueryAt = nil
	}
	result, err := s.database.ExecContext(ctx, `
INSERT INTO idc_gateway_tencent_contract_report_job
    (job_no, installation_id, source_route_no, report_route_no, status_code,
     report_cipher_format_version, report_encryption_key_version, report_nonce,
     report_ciphertext, next_query_at, last_reason_code, supplier_observed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (installation_id, source_route_no) DO NOTHING`,
		jobNo, s.installationID, sourceRouteNo, created.RouteNo, status,
		sealed.FormatVersion, sealed.KeyVersion, sealed.Nonce, sealed.Ciphertext,
		nextQueryAt, reason, created.ObservedAt.UTC(),
	)
	if err != nil {
		return reportJob{}, ErrRejected
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return reportJob{}, ErrRejected
	}
	if affected != 1 {
		return s.loadJob(ctx, sourceRouteNo)
	}
	return s.loadJob(ctx, sourceRouteNo)
}

func (s *Service) persistCreateFailure(
	ctx context.Context,
	sourceRouteNo string,
	reportRouteNo string,
	failure error,
) (Result, error) {
	jobNo, err := randomIdentifier(s.random, "lrj_")
	if err != nil {
		return Result{}, ErrRejected
	}
	status := "failed"
	state := StateSkipped
	reason := "report_rejected"
	var safe *tencentess.Error
	if errors.As(failure, &safe) {
		reason = string(safe.Code)
		if safe.Code == tencentess.ErrorResultUnknown {
			status = "result_unknown"
			state = StateResultUnknown
		}
	}
	_, err = s.database.ExecContext(ctx, `
INSERT INTO idc_gateway_tencent_contract_report_job
    (job_no, installation_id, source_route_no, report_route_no, status_code,
     next_query_at, last_reason_code, supplier_observed_at)
VALUES ($1, $2, $3, $4, $5, NULL, $6, CURRENT_TIMESTAMP)
ON CONFLICT (installation_id, source_route_no) DO NOTHING`,
		jobNo, s.installationID, sourceRouteNo, reportRouteNo, status, reason,
	)
	if err != nil {
		return Result{}, ErrRejected
	}
	job, loadErr := s.loadJob(ctx, sourceRouteNo)
	if loadErr != nil {
		return Result{}, ErrRejected
	}
	if job.Status == "result_unknown" {
		return Result{State: StateResultUnknown, Code: job.Reason}, nil
	}
	if job.Status == "failed" {
		return Result{State: StateSkipped, Code: job.Reason}, nil
	}
	return Result{State: state, Code: reason}, nil
}

func (s *Service) resumeReport(ctx context.Context, job reportJob) (Result, error) {
	switch job.Status {
	case "failed":
		return Result{State: StateSkipped, Code: job.Reason}, nil
	case "result_unknown":
		return Result{State: StateResultUnknown, Code: job.Reason}, nil
	case "executing", "ready":
	default:
		return Result{}, ErrConflict
	}
	if job.Status == "executing" && job.NextQueryAt.Valid && job.NextQueryAt.Time.After(s.now()) {
		return Result{State: StatePending, Code: job.Reason, NextCheckAt: job.NextQueryAt.Time.UTC()}, nil
	}
	if !job.CipherFormat.Valid || !job.EncryptionKey.Valid || len(job.Nonce) == 0 || len(job.Ciphertext) == 0 {
		return Result{}, ErrRejected
	}
	sealed := callbacksecurity.SealedEvidence{
		FormatVersion: uint16(job.CipherFormat.Int16), KeyVersion: uint32(job.EncryptionKey.Int32),
		Nonce: job.Nonce, Ciphertext: job.Ciphertext,
	}
	defer clear(sealed.Nonce)
	defer clear(sealed.Ciphertext)
	reportID, err := s.keyring.OpenReference(job.ReportRouteNo, providerCode, sealed)
	if err != nil {
		return Result{}, ErrRejected
	}
	defer clear(reportID)
	queried, err := s.client.FetchEvidenceReport(ctx, string(reportID))
	if err != nil {
		if code, terminal := terminalReportReadFailure(err); terminal {
			if updateErr := s.updateTerminal(ctx, job, "failed", code, s.now()); updateErr != nil {
				return Result{}, updateErr
			}
			return Result{State: StateSkipped, Code: code}, nil
		}
		if job.Status == "executing" {
			next := s.now().Add(reportRetryDelay)
			if updateErr := s.updateExecuting(ctx, job, next, safeReason(err)); updateErr != nil {
				return Result{}, updateErr
			}
			return Result{State: StatePending, Code: safeReason(err), NextCheckAt: next}, nil
		}
		return Result{}, err
	}
	switch queried.Status {
	case "EvidenceStatusExecuting":
		next := s.now().Add(reportRetryDelay)
		if err = s.updateExecuting(ctx, job, next, "report_executing"); err != nil {
			return Result{}, err
		}
		return Result{State: StatePending, Code: "report_executing", NextCheckAt: next}, nil
	case "EvidenceStatusFailed":
		if err = s.updateTerminal(ctx, job, "failed", "report_failed", queried.ObservedAt); err != nil {
			return Result{}, err
		}
		return Result{State: StateSkipped, Code: "report_failed"}, nil
	case "EvidenceStatusSuccess":
		if queried.Document == nil || len(queried.Document.Content) == 0 {
			return Result{}, ErrRejected
		}
		if err = s.updateTerminal(ctx, job, "ready", "report_ready", queried.ObservedAt); err != nil {
			clear(queried.Document.Content)
			return Result{}, err
		}
		return Result{State: StateReady, Code: "report_ready", Document: queried.Document}, nil
	default:
		return Result{}, ErrRejected
	}
}

func (s *Service) updateExecuting(ctx context.Context, job reportJob, next time.Time, reason string) error {
	result, err := s.database.ExecContext(ctx, `
UPDATE idc_gateway_tencent_contract_report_job
SET next_query_at = $1, last_reason_code = $2, updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE job_no = $3 AND installation_id = $4 AND version = $5 AND status_code = 'executing'`,
		next.UTC(), reason, job.JobNo, s.installationID, job.Version,
	)
	if err != nil {
		return ErrRejected
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ErrRejected
	}
	if affected != 1 {
		return ErrConflict
	}
	return nil
}

func (s *Service) updateTerminal(
	ctx context.Context,
	job reportJob,
	status string,
	reason string,
	observedAt time.Time,
) error {
	result, err := s.database.ExecContext(ctx, `
UPDATE idc_gateway_tencent_contract_report_job
SET status_code = $1, next_query_at = NULL, last_reason_code = $2,
    supplier_observed_at = $3, updated_at = CURRENT_TIMESTAMP, version = version + 1
WHERE job_no = $4 AND installation_id = $5 AND version = $6
  AND status_code IN ('executing', 'ready')`,
		status, reason, observedAt.UTC(), job.JobNo, s.installationID, job.Version,
	)
	if err != nil {
		return ErrRejected
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return ErrRejected
	}
	if affected != 1 {
		return ErrConflict
	}
	return nil
}

func safeReason(err error) string {
	var safe *tencentess.Error
	if errors.As(err, &safe) && safe.Code != "" {
		return string(safe.Code)
	}
	return "report_query_failed"
}

func nonChargeableSkip(err error) (string, bool) {
	var safe *tencentess.Error
	if !errors.As(err, &safe) {
		return "", false
	}
	switch safe.Code {
	case tencentess.ErrorNotConfigured, tencentess.ErrorCredentialRejected,
		tencentess.ErrorPermissionDenied, tencentess.ErrorSubjectUnavailable:
		return string(safe.Code), true
	default:
		return "", false
	}
}

func terminalReportReadFailure(err error) (string, bool) {
	var safe *tencentess.Error
	if !errors.As(err, &safe) {
		return "", false
	}
	switch safe.Code {
	case tencentess.ErrorNotConfigured, tencentess.ErrorCredentialRejected,
		tencentess.ErrorPermissionDenied, tencentess.ErrorSubjectUnavailable,
		tencentess.ErrorResourceNotFound, tencentess.ErrorInvalidInput,
		tencentess.ErrorRejected:
		return string(safe.Code), true
	default:
		return "", false
	}
}

func randomIdentifier(source io.Reader, prefix string) (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(source, value); err != nil {
		clear(value)
		return "", err
	}
	defer clear(value)
	return prefix + hex.EncodeToString(value), nil
}
