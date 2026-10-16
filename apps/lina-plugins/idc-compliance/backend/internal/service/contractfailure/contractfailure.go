// Package contractfailure owns the installation-local deadline queue. It
// delegates all order, inventory, and refund state changes to core.
package contractfailure

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"reflect"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

const (
	MaximumBatchSize = 50
	maximumAttempts  = 5
	leaseDuration    = 5 * time.Minute
)

type ProcessResult struct {
	Claimed       int
	RefundPending int
	Completed     int
	ManualReview  int
	Retried       int
}

type Service interface {
	ProcessDue(context.Context, int) (ProcessResult, error)
}

const (
	PermissionCancelOrder   = "idc:admin:compliance:contract:cancel-order"
	PermissionFailurePolicy = "idc:admin:compliance:contract:failure-policy"
	PermissionContractQuery = "idc:admin:compliance:contract:query"
)

type Policy struct {
	PolicyNo      string    `orm:"policy_no"`
	PolicyVersion int64     `orm:"policy_version"`
	HandlingDays  int       `orm:"handling_days"`
	CreatedAt     time.Time `orm:"created_at"`
	Reused        bool
}

type PolicyUpdateInput struct {
	ExpectedVersion int64
	HandlingDays    int
	IdempotencyKey  string
}

type ManagementService interface {
	Service
	GetPolicy(context.Context) (*Policy, error)
	UpdatePolicy(context.Context, PolicyUpdateInput) (*Policy, error)
	CancelNow(context.Context, string) (*contractcap.FailureCompensationResult, error)
}

type serviceImpl struct {
	database      gdb.DB
	compensations contractcap.FailureCompensator
	audit         idccap.BusinessAuditWriter
	authorizer    managementcap.Authorizer
}

type failureCase struct {
	CaseNo          string    `orm:"case_no"`
	ContractNo      string    `orm:"contract_no"`
	OrderNo         string    `orm:"order_no"`
	TerminalEventNo string    `orm:"terminal_event_no"`
	TerminalStatus  string    `orm:"terminal_status_code"`
	PolicyNo        string    `orm:"policy_no"`
	PolicyVersion   int64     `orm:"policy_version"`
	Deadline        time.Time `orm:"handling_deadline"`
	AttemptCount    int       `orm:"attempt_count"`
	LeaseToken      string    `orm:"lease_token"`
}

var _ Service = (*serviceImpl)(nil)
var _ ManagementService = (*serviceImpl)(nil)

func New(
	database gdb.DB,
	compensations contractcap.FailureCompensator,
	audit idccap.BusinessAuditWriter,
) (Service, error) {
	if nilDependency(database) || nilDependency(compensations) || nilDependency(audit) {
		return nil, gerror.New("contract failure worker dependencies are invalid")
	}
	return &serviceImpl{database: database, compensations: compensations, audit: audit}, nil
}

func NewManagement(
	database gdb.DB,
	compensations contractcap.FailureCompensator,
	audit idccap.BusinessAuditWriter,
	authorizer managementcap.Authorizer,
) (ManagementService, error) {
	service, err := New(database, compensations, audit)
	if err != nil || nilDependency(authorizer) {
		return nil, gerror.New("contract failure management dependencies are invalid")
	}
	result, ok := service.(*serviceImpl)
	if !ok {
		return nil, gerror.New("contract failure management service is invalid")
	}
	result.authorizer = authorizer
	return result, nil
}

func (s *serviceImpl) GetPolicy(ctx context.Context) (*Policy, error) {
	if s == nil || nilDependency(s.authorizer) || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("contract failure policy is unavailable")
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionContractQuery, TargetNo: "contract-failure-policy",
	}); err != nil {
		return nil, gerror.New("contract failure policy is not accessible")
	}
	return s.loadActivePolicy(ctx, false)
}

func (s *serviceImpl) UpdatePolicy(
	ctx context.Context,
	input PolicyUpdateInput,
) (result *Policy, err error) {
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if s == nil || nilDependency(s.authorizer) || ctx == nil || ctx.Err() != nil ||
		input.ExpectedVersion < 1 || input.HandlingDays < 0 || input.HandlingDays > 30 ||
		len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 {
		return nil, gerror.New("contract failure policy update is invalid")
	}
	if _, err = s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionFailurePolicy, TargetNo: "contract-failure-policy",
	}); err != nil {
		return nil, gerror.New("contract failure policy update is not accessible")
	}
	requestDigest := sha256Hex("contract-failure-policy-v1\x00" + input.IdempotencyKey)
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var existing []Policy
		if scanErr := s.database.GetScan(ctx, &existing, `
SELECT policy_no, policy_version, handling_days, created_at
FROM plugin_idc_compliance_contract_failure_policy
WHERE tenant_id = 0 AND request_digest = ?`, requestDigest); scanErr != nil {
			return gerror.Wrap(scanErr, "load contract failure policy replay")
		}
		if len(existing) == 1 {
			existing[0].Reused = true
			result = &existing[0]
			return nil
		}
		if len(existing) > 1 {
			return gerror.New("contract failure policy replay is ambiguous")
		}
		current, loadErr := s.loadActivePolicy(ctx, true)
		if loadErr != nil {
			return loadErr
		}
		if current.PolicyVersion != input.ExpectedVersion {
			return gerror.New("contract failure policy version changed")
		}
		now, timeErr := databaseNow(ctx, s.database)
		if timeErr != nil {
			return timeErr
		}
		newVersion := current.PolicyVersion + 1
		policyNo := "cfp_" + requestDigest[:32]
		updated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract_failure_policy
SET status_code = 'superseded'
WHERE tenant_id = 0 AND policy_no = ? AND status_code = 'active'`, current.PolicyNo)
		if updateErr != nil || affected(updated) != 1 {
			return gerror.New("supersede contract failure policy failed")
		}
		if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_failure_policy
    (tenant_id, policy_no, policy_version, handling_days, status_code,
     request_digest, created_at)
VALUES (0, ?, ?, ?, 'active', ?, ?)`,
			policyNo, newVersion, input.HandlingDays, requestDigest, now); insertErr != nil {
			return gerror.Wrap(insertErr, "insert contract failure policy")
		}
		if auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-failure-policy",
			IdempotencyKey:   requestDigest,
			OperationCode:    "compliance.contract-failure-policy-update",
			TargetType:       "contract_failure_policy", TargetNo: policyNo,
			ResultCode: "succeeded", ReasonCode: "administrator_configured",
			RiskCode: "high", RequiredPermission: PermissionFailurePolicy,
			BeforeSnapshot: map[string]any{
				"policy_version": current.PolicyVersion,
				"handling_days":  current.HandlingDays,
			},
			AfterSnapshot: map[string]any{
				"policy_version": newVersion,
				"handling_days":  input.HandlingDays,
			},
		}); auditErr != nil {
			return gerror.Wrap(auditErr, "append contract failure policy audit")
		}
		result = &Policy{PolicyNo: policyNo, PolicyVersion: newVersion,
			HandlingDays: input.HandlingDays, CreatedAt: now}
		return nil
	})
	return result, err
}

func (s *serviceImpl) loadActivePolicy(ctx context.Context, lock bool) (*Policy, error) {
	query := `
SELECT policy_no, policy_version, handling_days, created_at
FROM plugin_idc_compliance_contract_failure_policy
WHERE tenant_id = 0 AND status_code = 'active'`
	if lock {
		query += " FOR UPDATE"
	}
	var rows []Policy
	if err := s.database.GetScan(ctx, &rows, query); err != nil {
		return nil, gerror.Wrap(err, "load active contract failure policy")
	}
	if len(rows) != 1 {
		return nil, gerror.New("active contract failure policy is unavailable")
	}
	return &rows[0], nil
}

func (s *serviceImpl) ProcessDue(ctx context.Context, limit int) (ProcessResult, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > MaximumBatchSize {
		return ProcessResult{}, gerror.New("contract failure worker request is invalid")
	}
	var result ProcessResult
	for result.Claimed < limit {
		row, err := s.claim(ctx, "", "deadline")
		if err != nil {
			return result, err
		}
		if row == nil {
			break
		}
		result.Claimed++
		outcome, err := s.execute(ctx, row, "deadline")
		if err != nil {
			retried, persistErr := s.releaseFailure(ctx, row, err)
			if persistErr != nil {
				return result, persistErr
			}
			if retried {
				result.Retried++
			} else {
				result.ManualReview++
			}
			continue
		}
		switch outcome.StatusCode {
		case "refund_pending":
			result.RefundPending++
		case "completed":
			result.Completed++
		default:
			result.ManualReview++
		}
	}
	return result, nil
}

func (s *serviceImpl) CancelNow(
	ctx context.Context,
	contractNo string,
) (*contractcap.FailureCompensationResult, error) {
	contractNo = strings.TrimSpace(contractNo)
	if s == nil || nilDependency(s.authorizer) || ctx == nil || ctx.Err() != nil || contractNo == "" {
		return nil, gerror.New("contract immediate cancellation request is invalid")
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionCancelOrder, TargetNo: contractNo,
	}); err != nil {
		return nil, gerror.New("contract immediate cancellation is not accessible")
	}
	row, err := s.claim(ctx, contractNo, "admin_cancel")
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, gerror.New("contract failure case is not available for immediate cancellation")
	}
	result, err := s.execute(ctx, row, "admin_cancel")
	if err != nil {
		if _, releaseErr := s.releaseFailure(ctx, row, err); releaseErr != nil {
			return nil, gerror.Wrapf(releaseErr,
				"release immediate contract compensation after execution failure: %v", err)
		}
		return nil, err
	}
	return result, nil
}

func (s *serviceImpl) claim(
	ctx context.Context,
	contractNo string,
	triggerCode string,
) (*failureCase, error) {
	var claimed *failureCase
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		query := `
SELECT case_no, contract_no, order_no, terminal_event_no, terminal_status_code,
       policy_no, policy_version, handling_deadline, attempt_count
FROM plugin_idc_compliance_contract_failure_case
WHERE tenant_id = 0
  AND (status_code = 'pending'
       OR (status_code = 'processing' AND lease_expires_at <= ?))`
		arguments := []any{now}
		if contractNo == "" {
			query += " AND handling_deadline <= ?"
			arguments = append(arguments, now)
		} else {
			query += " AND contract_no = ?"
			arguments = append(arguments, contractNo)
		}
		query += " ORDER BY handling_deadline, id FOR UPDATE SKIP LOCKED LIMIT 1"
		var rows []failureCase
		if err = s.database.GetScan(ctx, &rows, query, arguments...); err != nil {
			return gerror.Wrap(err, "claim contract failure case")
		}
		if len(rows) == 0 {
			return nil
		}
		leaseToken, err := randomDigest()
		if err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract_failure_case
SET status_code = 'processing', trigger_code = ?, lease_token = ?,
    lease_expires_at = ?, updated_at = ?
WHERE tenant_id = 0 AND case_no = ?
  AND (status_code = 'pending'
       OR (status_code = 'processing' AND lease_expires_at <= ?))`,
			triggerCode, leaseToken, now.Add(leaseDuration), now,
			rows[0].CaseNo, now)
		if err != nil || affected(result) != 1 {
			return gerror.New("contract failure case changed while claiming")
		}
		rows[0].LeaseToken = leaseToken
		claimed = &rows[0]
		return nil
	})
	return claimed, err
}

func (s *serviceImpl) execute(
	ctx context.Context,
	row *failureCase,
	triggerCode string,
) (*contractcap.FailureCompensationResult, error) {
	result, err := s.compensations.CompensateContractFailure(ctx, contractcap.FailureCompensationInput{
		ContractNo: row.ContractNo, OrderNo: row.OrderNo,
		TerminalEventNo: row.TerminalEventNo, TerminalStatus: row.TerminalStatus,
		PolicyNo: row.PolicyNo, PolicyVersion: row.PolicyVersion,
		HandlingDeadline: row.Deadline, TriggerCode: triggerCode,
	})
	if err != nil {
		return nil, err
	}
	statusCode := result.StatusCode
	if statusCode != "refund_pending" && statusCode != "completed" && statusCode != "manual_review" {
		return nil, gerror.New("core contract compensation returned an invalid status")
	}
	var completedAt any
	if statusCode == "completed" {
		now, timeErr := databaseNow(ctx, s.database)
		if timeErr != nil {
			return nil, timeErr
		}
		completedAt = now
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		resultDB, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract_failure_case
SET status_code = ?, compensation_no = ?, refund_request_no = NULLIF(?, ''),
    last_reason_code = ?, completed_at = ?, lease_token = NULL,
    lease_expires_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND case_no = ? AND status_code = 'processing'
  AND lease_token = ?`, statusCode, result.CompensationNo, result.RefundRequestNo,
			result.ReasonCode, completedAt, row.CaseNo, row.LeaseToken)
		if updateErr != nil || affected(resultDB) != 1 {
			return gerror.New("persist contract compensation result failed")
		}
		if auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-failure-case",
			IdempotencyKey:   row.CaseNo + ":" + result.CompensationNo,
			OperationCode:    "compliance.contract-failure-case",
			TargetType:       "contract",
			TargetNo:         row.ContractNo,
			ResultCode:       idccap.BusinessAuditResultSucceeded,
			ReasonCode:       result.ReasonCode,
			RiskCode:         "critical",
			CorrelationNo:    result.CompensationNo,
			BeforeSnapshot: map[string]any{
				"case_status": "processing", "trigger": triggerCode,
			},
			AfterSnapshot: map[string]any{
				"case_status":            statusCode,
				"refund_request_created": result.RefundRequestNo != "",
			},
		}); auditErr != nil {
			return gerror.Wrap(auditErr, "append contract failure case audit")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *serviceImpl) releaseFailure(
	ctx context.Context,
	row *failureCase,
	cause error,
) (bool, error) {
	nextAttempts := row.AttemptCount + 1
	statusCode := "pending"
	if nextAttempts >= maximumAttempts {
		statusCode = "manual_review"
	}
	reasonDigest := sha256.Sum256([]byte(cause.Error()))
	reasonCode := "compensation_retry_" + hex.EncodeToString(reasonDigest[:])[:16]
	result, err := s.database.Exec(ctx, `
UPDATE plugin_idc_compliance_contract_failure_case
SET status_code = ?, attempt_count = ?, last_reason_code = ?,
    lease_token = NULL, lease_expires_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND case_no = ? AND status_code = 'processing'
  AND lease_token = ?`, statusCode, nextAttempts, reasonCode, row.CaseNo, row.LeaseToken)
	if err != nil || affected(result) != 1 {
		return false, gerror.New("release failed contract compensation claim failed")
	}
	return statusCode == "pending", nil
}

func databaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	var row struct {
		Now time.Time `orm:"database_now"`
	}
	if err := database.GetScan(ctx, &row, `SELECT CURRENT_TIMESTAMP AS database_now`); err != nil || row.Now.IsZero() {
		return time.Time{}, gerror.New("read contract failure database time failed")
	}
	return row.Now.UTC(), nil
}

func randomDigest() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", gerror.Wrap(err, "create contract failure lease")
	}
	return hex.EncodeToString(buffer), nil
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func affected(result sql.Result) int64 {
	if result == nil {
		return -1
	}
	count, err := result.RowsAffected()
	if err != nil {
		return -1
	}
	return count
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
