// Package customerclosure owns the reviewed, irreversible completion path for
// customer account closure. It is separate from customer self-service so a
// customer controller can never approve its own request.
//
//nolint:cyclop // Closure keeps authorization, retention, fencing, anonymization, and audit guards together for atomic review.
package customerclosure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

var (
	CodeInvalid     = bizerr.MustDefine("IDC_CUSTOMER_CLOSURE_INVALID", "The customer closure completion request is invalid", gcode.CodeInvalidParameter)
	CodeUnavailable = bizerr.MustDefine("IDC_CUSTOMER_CLOSURE_UNAVAILABLE", "The customer closure completion service is temporarily unavailable", gcode.CodeInternalError)
	CodeNotAllowed  = bizerr.MustDefine("IDC_CUSTOMER_CLOSURE_NOT_ALLOWED", "The customer closure request cannot be completed", gcode.CodeNotAuthorized)
)

type CompleteInput struct {
	RequestNo      string
	CustomerNo     string
	IdempotencyKey string
}

type CompleteResult struct {
	RequestNo   string
	CustomerNo  string
	Status      customer.ClosureStatus
	CompletedAt time.Time
}

type Service interface {
	Complete(context.Context, CompleteInput) (*CompleteResult, error)
}

type serviceImpl struct {
	database   gdb.DB
	bizCtx     bizctxcap.Service
	principal  principal.Reader
	authorizer customer.AccountStateAuthorizer
	precheck   customer.ClosurePrecheckService
	audit      audit.Writer
}

func New(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader,
	authorizer customer.AccountStateAuthorizer, precheck customer.ClosurePrecheckService,
	auditWriter audit.Writer) (Service, error) {
	if database == nil || bizCtx == nil || principalReader == nil || authorizer == nil || precheck == nil || auditWriter == nil {
		return nil, gerror.New("customer closure completion requires all dependencies")
	}
	return &serviceImpl{database: database, bizCtx: bizCtx, principal: principalReader,
		authorizer: authorizer, precheck: precheck, audit: auditWriter}, nil
}

type closureRow struct {
	RequestNo      string     `orm:"request_no"`
	CustomerNo     string     `orm:"customer_no"`
	StatusCode     string     `orm:"status_code"`
	SubjectVersion int64      `orm:"subject_version"`
	RequestedAt    time.Time  `orm:"requested_at"`
	CompletedAt    *time.Time `orm:"completed_at"`
}

func (s *serviceImpl) Complete(ctx context.Context, input CompleteInput) (*CompleteResult, error) {
	input.RequestNo = strings.TrimSpace(input.RequestNo)
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.RequestNo == "" || input.CustomerNo == "" || len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.directStaff(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.authorizer.AuthorizeTransition(ctx, input.CustomerNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAllowed)
	}
	var result *CompleteResult
	err = s.database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		var row *closureRow
		if loadErr := tx.GetScan(&row, `SELECT request_no,customer_no,status_code,subject_version,requested_at,completed_at FROM plugin_idc_core_customer_closure_request WHERE tenant_id=0 AND request_no=? AND customer_no=? FOR UPDATE`, input.RequestNo, input.CustomerNo); loadErr != nil {
			return loadErr
		}
		if row == nil {
			return bizerr.NewCode(CodeNotAllowed)
		}
		if row.StatusCode == string(customer.ClosureStatusCompleted) && row.CompletedAt != nil {
			result = &CompleteResult{RequestNo: row.RequestNo, CustomerNo: row.CustomerNo,
				Status: customer.ClosureStatusCompleted, CompletedAt: *row.CompletedAt}
			return nil
		}
		if row.StatusCode != string(customer.ClosureStatusPendingReview) {
			return bizerr.NewCode(CodeNotAllowed)
		}
		checks, checkErr := s.precheck.BatchCheck(txCtx, []string{row.CustomerNo})
		if checkErr != nil {
			return bizerr.WrapCode(checkErr, CodeUnavailable)
		}
		if len(checks) != 1 || checks[0].CustomerNo != row.CustomerNo || checks[0].ContractVersion != 1 {
			return bizerr.WrapCode(gerror.New("customer closure final precheck is incomplete"), CodeUnavailable)
		}
		for _, finding := range checks[0].Findings {
			if finding.Code != customer.ClosureFindingLegalRetention && finding.Count > 0 {
				return bizerr.NewCode(CodeNotAllowed)
			}
		}
		nowValue, timeErr := tx.GetValue(`SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return timeErr
		}
		now := nowValue.Time()
		if now.IsZero() {
			return gerror.New("customer closure database time is unavailable")
		}
		if anonErr := anonymizeCustomer(txCtx, tx, row.CustomerNo, row.SubjectVersion, actor.UserID, now); anonErr != nil {
			return anonErr
		}
		update, updateErr := tx.Exec(`UPDATE plugin_idc_core_customer_closure_request SET status_code='completed',reviewed_by_user_id=?,reviewed_at=?,completed_at=?,updated_at=? WHERE tenant_id=0 AND request_no=? AND customer_no=? AND status_code='pending_review'`, actor.UserID, now, now, now, row.RequestNo, row.CustomerNo)
		if updateErr != nil {
			return updateErr
		}
		if affected, affectedErr := update.RowsAffected(); affectedErr != nil || affected != 1 {
			return gerror.Wrap(affectedErr, "complete customer closure request")
		}
		_, auditErr := s.audit.AppendTransition(txCtx, audit.TransitionInput{
			Fields: audit.Fields{
				IdempotencyScope: "customer.closure.complete", IdempotencyKey: input.IdempotencyKey,
				OperationCode: "customer.closure.complete", TargetType: "customer",
				TargetNo: row.CustomerNo, CustomerNo: row.CustomerNo,
				Result: audit.ResultSucceeded, RequiredPermission: "idc:admin:customer:update",
			},
			BeforeSnapshot: map[string]any{"status": "pending_review"},
			AfterSnapshot:  map[string]any{"status": "anonymized", "request_no": row.RequestNo},
		})
		if auditErr != nil {
			return auditErr
		}
		result = &CompleteResult{RequestNo: row.RequestNo, CustomerNo: row.CustomerNo,
			Status: customer.ClosureStatusCompleted, CompletedAt: now}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeNotAllowed) || bizerr.Is(err, CodeUnavailable) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}

func (s *serviceImpl) directStaff(ctx context.Context) (*principal.Audience, error) {
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant {
		return nil, bizerr.NewCode(CodeNotAllowed)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAllowed)
	}
	if audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive {
		return nil, bizerr.NewCode(CodeNotAllowed)
	}
	return audience, nil
}

func anonymizeCustomer(_ context.Context, tx gdb.TX, customerNo string, expectedVersion int64, actorUserID int, now time.Time) error {
	hash := sha256.Sum256([]byte("customer-closure-contact-v1\x00" + customerNo))
	digest := hex.EncodeToString(hash[:])
	email := "closed+" + digest[:24] + "@invalid.local"
	statements := []struct {
		query string
		args  []any
	}{
		{`UPDATE plugin_idc_core_customer_individual_profile SET legal_name='',legal_name_masked='',identity_identifier_masked='',identity_lookup_digest='',identity_status_code='revoked',identity_verified_at=NULL,identity_expires_at=NULL,version=version+1,updated_at=? WHERE tenant_id=0 AND customer_no=?`, []any{now, customerNo}},
		{`UPDATE plugin_idc_core_customer_enterprise_profile SET legal_name='',registration_identifier='',registration_identifier_masked='',registration_lookup_digest='',industry_code='',verification_status_code='revoked',verified_at=NULL,expires_at=NULL,version=version+1,updated_at=? WHERE tenant_id=0 AND customer_no=?`, []any{now, customerNo}},
		{`UPDATE plugin_idc_core_customer_contact SET display_name='Anonymized customer',email=?,email_lookup_digest=?,email_verified_at=NULL,phone_country_code='',phone='',phone_lookup_digest='',phone_verified_at=NULL,status_code='archived',version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=?`, []any{email, digest, actorUserID, now, customerNo}},
		{`UPDATE plugin_idc_core_customer_contact_preference SET enabled=FALSE,version=version+1,updated_at=? WHERE tenant_id=0 AND customer_no=?`, []any{now, customerNo}},
		{`UPDATE plugin_idc_core_customer_contact_role SET status_code='revoked',revoked_by=?,revoked_at=COALESCE(revoked_at,?),updated_at=? WHERE tenant_id=0 AND customer_no=? AND status_code='active'`, []any{actorUserID, now, now, customerNo}},
		{`UPDATE plugin_idc_core_customer_address SET label='',recipient_name='',recipient_phone='',province='',city='',district='',address_line1='anonymized',address_line2='',postal_code='',is_default=FALSE,status_code='archived',version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=?`, []any{actorUserID, now, customerNo}},
		{`UPDATE plugin_idc_core_customer_identity_binding SET status_code='revoked',revoked_at=COALESCE(revoked_at,?),version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND status_code='active'`, []any{now, actorUserID, now, customerNo}},
		{`UPDATE plugin_idc_core_principal_audience SET status_code='revoked',revoked_at=COALESCE(revoked_at,?),version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND audience_code='customer' AND status_code='active' AND sys_user_id IN (SELECT sys_user_id FROM plugin_idc_core_customer_identity_binding WHERE tenant_id=0 AND customer_no=?)`, []any{now, actorUserID, now, customerNo}},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement.query, statement.args...); err != nil {
			return err
		}
	}
	update, err := tx.Exec(`UPDATE plugin_idc_core_customer_subject SET status_code='anonymized',display_name='Anonymized customer',search_name='',country_code='ZZ',preferred_locale='zh-CN',status_reason_code='approved_closure',status_reason='',status_changed_by=?,status_changed_at=?,anonymized_at=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND version=? AND status_code<>'anonymized'`, actorUserID, now, now, actorUserID, now, customerNo, expectedVersion)
	if err != nil {
		return err
	}
	affected, err := update.RowsAffected()
	if err != nil || affected != 1 {
		return gerror.Wrap(err, "anonymize customer subject")
	}
	return nil
}
