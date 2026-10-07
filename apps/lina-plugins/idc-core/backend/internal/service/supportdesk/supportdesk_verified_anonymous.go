// This file creates anonymous presales tickets only after notify has consumed
// its verified OTP proof and called the source-only support capability.
package supportdesk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/supportcap"
)

const anonymousAccessSecretPurpose = supportcap.AnonymousAccessSecretScope

type verifiedAnonymousCreator struct {
	database gdb.DB
	tokens   secretcap.GeneratedManager
	accessor secretcap.Accessor
}

type anonymousCreateOperation struct {
	TicketNo          string     `orm:"ticket_no"`
	RequestDigest     string     `orm:"request_digest"`
	TokenSecretRef    string     `orm:"token_secret_ref"`
	TokenDigest       string     `orm:"token_digest"`
	DestinationKind   string     `orm:"destination_kind"`
	DestinationDigest string     `orm:"destination_digest"`
	ExpiresAt         time.Time  `orm:"expires_at"`
	RevokedAt         *time.Time `orm:"revoked_at"`
}

var _ supportcap.Service = (*verifiedAnonymousCreator)(nil)

// NewVerifiedAnonymousCreator binds the ticket transaction to one exact
// core-owned secret scope. Caller authorization remains at the supportcap SPI
// factory, while this service repeats all request and persistence checks.
func NewVerifiedAnonymousCreator(
	database gdb.DB,
	tokens secretcap.GeneratedManager,
	accessor secretcap.Accessor,
) (supportcap.Service, error) {
	if database == nil || tokens == nil || accessor == nil {
		return nil, gerror.New("verified anonymous support creation requires database and token services")
	}
	return &verifiedAnonymousCreator{database: database, tokens: tokens, accessor: accessor}, nil
}

func (s *verifiedAnonymousCreator) CreateAnonymousPresales(
	ctx context.Context,
	request supportcap.CreateAnonymousPresalesRequest,
) (*supportcap.CreateAnonymousPresalesResult, error) {
	if ctx == nil || ctx.Err() != nil || supportcap.ValidateCreateRequest(request) != nil {
		return nil, bizInvalid()
	}
	operationDigest, err := supportcap.OperationDigest(request.OperationNo)
	if err != nil {
		return nil, bizInvalid()
	}
	requestDigest, err := anonymousCreateRequestDigest(request)
	if err != nil {
		return nil, wrapUnavailable(err, "digest verified anonymous support request")
	}
	if existing, loadErr := s.loadAnonymousCreateOperation(ctx, operationDigest); loadErr != nil {
		return nil, wrapUnavailable(loadErr, "load verified anonymous support operation")
	} else if existing != nil {
		return s.recoverAnonymousCreateResult(ctx, request, operationDigest, requestDigest, existing)
	}

	secretResult, err := s.tokens.CreateGenerated(
		ctx, operationDigest, secretcap.GeneratedSecretPolicyURLSafe256,
	)
	if err != nil || secretResult.Handle.IsZero() {
		return nil, wrapUnavailable(firstAnonymousCreateError(err), "prepare anonymous support token")
	}
	accessToken, err := s.readAnonymousAccessToken(ctx, secretResult.Handle)
	if err != nil {
		return nil, err
	}
	operation := anonymousCreateOperation{
		TicketNo:      deterministicAnonymousNo("TKT_", operationDigest),
		RequestDigest: requestDigest, TokenSecretRef: secretResult.Handle.OpaqueRef(),
		TokenDigest: anonymousTokenDigest(accessToken), DestinationKind: string(request.DestinationKind),
		DestinationDigest: request.DestinationDigest,
	}
	insertErr := s.insertAnonymousPresales(ctx, request, operationDigest, &operation)
	if insertErr == nil {
		result := &supportcap.CreateAnonymousPresalesResult{
			TicketNo: operation.TicketNo, AccessToken: accessToken, ExpiresAt: operation.ExpiresAt,
		}
		if supportcap.ValidateCreateResult(*result) != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		return result, nil
	}
	// A concurrent identical request can win any of the deterministic unique
	// fences before this transaction reaches the operation row. Reload by the
	// authoritative operation digest and return the winner when it matches.
	existing, loadErr := s.loadAnonymousCreateOperation(ctx, operationDigest)
	if loadErr == nil && existing != nil {
		return s.recoverAnonymousCreateResult(ctx, request, operationDigest, requestDigest, existing)
	}
	return nil, wrapUnavailable(insertErr, "create verified anonymous support ticket")
}

func (s *verifiedAnonymousCreator) insertAnonymousPresales(
	ctx context.Context,
	request supportcap.CreateAnonymousPresalesRequest,
	operationDigest string,
	operation *anonymousCreateOperation,
) error {
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if existing, err := s.loadAnonymousCreateOperationForUpdate(ctx, operationDigest); err != nil {
			return err
		} else if existing != nil {
			if !anonymousCreateOperationMatches(request, operation.RequestDigest, existing) ||
				existing.TokenSecretRef != operation.TokenSecretRef ||
				existing.TokenDigest != operation.TokenDigest {
				return bizerr.NewCode(CodeConflict)
			}
			operation.TicketNo, operation.ExpiresAt = existing.TicketNo, existing.ExpiresAt
			return nil
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		input := CreateInput{TicketType: "presales", CategoryNo: request.CategoryNo}
		departmentNo, priority, err := (&serviceImpl{database: s.database}).resolveRoute(ctx, input)
		if err != nil {
			return gerror.Wrap(err, "resolve verified anonymous support route")
		}
		support := &serviceImpl{database: s.database}
		customFields, err := support.prepareCustomFieldSnapshots(ctx, "presales", fromSupportcapCustomFields(request.CustomFields))
		if err != nil {
			return gerror.Wrap(err, "validate verified anonymous support custom fields")
		}
		assignee, assignmentMode, err := support.resolveInitialAssignee(ctx, departmentNo)
		if err != nil {
			return gerror.Wrap(err, "resolve verified anonymous support assignee")
		}
		messageNo := deterministicAnonymousNo("MSG_", operationDigest)
		historyNo := deterministicAnonymousNo("TSH_", operationDigest)
		assignmentNo := deterministicAnonymousNo("TAH_", operationDigest)
		linkNo := deterministicAnonymousNo("SAL_", operationDigest)
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,customer_no,anonymous_destination_digest,anonymous_destination_kind,category_no,department_no,assignee_user_id,subject,base_stage,priority_code,current_round,last_activity_at,created_by,updated_by,created_at,updated_at) VALUES (0,?,'presales','anonymous',NULL,?,?,?,?,? ,?,'new',?,1,?,0,0,?,?)`, operation.TicketNo, request.DestinationDigest, request.DestinationKind, request.CategoryNo, nullable(departmentNo), nullableInt(assignee), request.Subject, priority, now, now, now); err != nil {
			return gerror.Wrap(err, "insert verified anonymous support ticket")
		}
		if err = support.saveCustomFieldSnapshots(ctx, operation.TicketNo, customFields); err != nil {
			return gerror.Wrap(err, "insert verified anonymous support custom fields")
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_message (tenant_id,message_no,ticket_no,message_type,visibility_code,author_type,author_user_id,body,automated,created_at) VALUES (0,?,?,'initial','customer_visible','anonymous',0,?,FALSE,?)`, messageNo, operation.TicketNo, request.Body, now); err != nil {
			return gerror.Wrap(err, "insert verified anonymous support initial message")
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_status_history (tenant_id,history_no,ticket_no,from_base_stage,to_base_stage,source_code,actor_user_id,reason_code,occurred_at) VALUES (0,?,?,NULL,'new','anonymous',0,'created',?)`, historyNo, operation.TicketNo, now); err != nil {
			return gerror.Wrap(err, "insert verified anonymous support status history")
		}
		action := "created"
		if assignee > 0 {
			action = "auto_assigned"
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_assignment_history (tenant_id,history_no,ticket_no,action_code,to_department_no,to_user_id,actor_user_id,reason_code,occurred_at) VALUES (0,?,?,?,?,?,0,?,?)`, assignmentNo, operation.TicketNo, action, nullable(departmentNo), nullableInt(assignee), assignmentMode, now); err != nil {
			return gerror.Wrap(err, "insert verified anonymous support assignment history")
		}
		if err = support.createSLARound(ctx, operation.TicketNo, 1, departmentNo, request.CategoryNo, priority, now); err != nil {
			return gerror.Wrap(err, "create verified anonymous support SLA snapshot")
		}
		operation.ExpiresAt = anonymousAccessExpiry(now)
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_access_link (tenant_id,link_no,ticket_no,token_digest,destination_digest,destination_kind,operation_digest,request_digest,token_secret_ref,expires_at,created_at) VALUES (0,?,?,?,?,?,?,?,?,?,?)`, linkNo, operation.TicketNo, operation.TokenDigest, request.DestinationDigest, request.DestinationKind, operationDigest, operation.RequestDigest, operation.TokenSecretRef, operation.ExpiresAt, now)
		return gerror.Wrap(err, "insert verified anonymous support access link")
	})
}

func (s *verifiedAnonymousCreator) recoverAnonymousCreateResult(
	ctx context.Context,
	request supportcap.CreateAnonymousPresalesRequest,
	operationDigest string,
	requestDigest string,
	existing *anonymousCreateOperation,
) (*supportcap.CreateAnonymousPresalesResult, error) {
	if !anonymousCreateOperationMatches(request, requestDigest, existing) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	secretResult, err := s.tokens.CreateGenerated(
		ctx, operationDigest, secretcap.GeneratedSecretPolicyURLSafe256,
	)
	if err != nil || secretResult.Handle.IsZero() || secretResult.Handle.OpaqueRef() != existing.TokenSecretRef {
		return nil, wrapUnavailable(firstAnonymousCreateError(err), "recover anonymous support token")
	}
	token, err := s.readAnonymousAccessToken(ctx, secretResult.Handle)
	if err != nil {
		return nil, err
	}
	if anonymousTokenDigest(token) != existing.TokenDigest {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	result := &supportcap.CreateAnonymousPresalesResult{
		TicketNo: existing.TicketNo, AccessToken: token, ExpiresAt: existing.ExpiresAt,
	}
	if supportcap.ValidateCreateResult(*result) != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return result, nil
}

func (s *verifiedAnonymousCreator) readAnonymousAccessToken(
	ctx context.Context,
	handle secretcap.Handle,
) (string, error) {
	purpose, err := secretcap.NewPurpose(anonymousAccessSecretPurpose)
	if err != nil {
		return "", bizerr.NewCode(CodeUnavailable)
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return "", bizerr.NewCode(CodeUnavailable)
	}
	var copied []byte
	err = s.accessor.Access(ctx, request, func(plaintext []byte) error {
		copied = append([]byte(nil), plaintext...)
		return nil
	})
	if err != nil {
		clear(copied)
		return "", wrapUnavailable(err, "read anonymous support token")
	}
	token := string(copied)
	clear(copied)
	if supportcap.ValidateCreateResult(supportcap.CreateAnonymousPresalesResult{
		TicketNo: "TKT_00000000000000000000000000000000", AccessToken: token, ExpiresAt: time.Unix(1, 0),
	}) != nil {
		return "", bizerr.NewCode(CodeUnavailable)
	}
	return token, nil
}

func (s *verifiedAnonymousCreator) loadAnonymousCreateOperation(
	ctx context.Context,
	operationDigest string,
) (*anonymousCreateOperation, error) {
	return s.queryAnonymousCreateOperation(ctx, operationDigest, false)
}

func (s *verifiedAnonymousCreator) loadAnonymousCreateOperationForUpdate(
	ctx context.Context,
	operationDigest string,
) (*anonymousCreateOperation, error) {
	return s.queryAnonymousCreateOperation(ctx, operationDigest, true)
}

func (s *verifiedAnonymousCreator) queryAnonymousCreateOperation(
	ctx context.Context,
	operationDigest string,
	lock bool,
) (*anonymousCreateOperation, error) {
	query := `SELECT ticket_no,request_digest,token_secret_ref,token_digest,destination_kind,destination_digest,expires_at,revoked_at FROM plugin_idc_core_support_ticket_access_link WHERE tenant_id=0 AND operation_digest=?`
	if lock {
		query += ` FOR UPDATE`
	}
	var rows []anonymousCreateOperation
	if err := s.database.GetScan(ctx, &rows, query, operationDigest); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 {
		return nil, gerror.New("anonymous support operation is not unique")
	}
	return &rows[0], nil
}

func anonymousCreateOperationMatches(
	request supportcap.CreateAnonymousPresalesRequest,
	requestDigest string,
	existing *anonymousCreateOperation,
) bool {
	return existing != nil && existing.RequestDigest == requestDigest &&
		existing.DestinationKind == string(request.DestinationKind) &&
		existing.DestinationDigest == request.DestinationDigest &&
		existing.TokenSecretRef != "" && existing.TokenDigest != "" && existing.RevokedAt == nil
}

func anonymousCreateRequestDigest(request supportcap.CreateAnonymousPresalesRequest) (string, error) {
	payload, err := json.Marshal(struct {
		ContractVersion   string
		DestinationKind   supportcap.DestinationKind
		DestinationDigest string
		CategoryNo        string
		Subject           string
		Body              string
		CustomFields      []supportcap.CustomFieldValue
	}{
		ContractVersion: supportcap.ContractVersion, DestinationKind: request.DestinationKind,
		DestinationDigest: request.DestinationDigest, CategoryNo: request.CategoryNo,
		Subject: request.Subject, Body: request.Body,
		CustomFields: canonicalAnonymousCustomFields(request.CustomFields),
	})
	if err != nil {
		return "", gerror.Wrap(err, "encode anonymous support request")
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func fromSupportcapCustomFields(items []supportcap.CustomFieldValue) []CustomFieldSubmission {
	result := make([]CustomFieldSubmission, 0, len(items))
	for _, item := range items {
		result = append(result, CustomFieldSubmission{
			FieldNo: item.FieldNo, TextValue: item.TextValue, NumberValue: item.NumberValue,
			DateValue: item.DateValue, BooleanValue: item.BooleanValue,
			OptionNos: append([]string(nil), item.OptionNos...),
		})
	}
	return result
}

func canonicalAnonymousCustomFields(items []supportcap.CustomFieldValue) []supportcap.CustomFieldValue {
	result := make([]supportcap.CustomFieldValue, len(items))
	copy(result, items)
	for index := range result {
		result[index].OptionNos = append([]string(nil), result[index].OptionNos...)
		sort.Strings(result[index].OptionNos)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].FieldNo < result[j].FieldNo })
	return result
}

func deterministicAnonymousNo(prefix, operationDigest string) string {
	digest := sha256.Sum256([]byte("idc-support-anonymous-number-v1\x00" + prefix + "\x00" + operationDigest))
	return prefix + hex.EncodeToString(digest[:16])
}

func firstAnonymousCreateError(err error) error {
	if err != nil {
		return err
	}
	return gerror.New("anonymous support secret result is unavailable")
}
