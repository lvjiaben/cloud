package supportdesk

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/service/supportupstream"
)

const maxUpstreamSelectionItems = 100

type UpstreamChannel struct {
	BaseURL, AuthMode, Status, CredentialMaskedTail string
	Configured                                      bool
	Version                                         int64
	UpdatedAt                                       time.Time
}
type UpstreamChannelInput struct {
	OperationNo, BaseURL, AuthMode, Username, Password, Status, Reason string
	ExpectedVersion                                                    int64
}
type UpstreamDepartmentMapInput struct {
	DepartmentNo                   string
	UpstreamDepartmentID           int64
	UpstreamDepartmentName, Status string
	ExpectedVersion                int64
}
type UpstreamDepartmentMap struct {
	DepartmentNo, UpstreamDepartmentName, Status string
	UpstreamDepartmentID                         int64
	Version                                      int64
}
type UpstreamEntitlementMapInput struct {
	EntitlementNo, UpstreamReference, Status string
	ExpectedVersion                          int64
}
type UpstreamEntitlementMap struct {
	EntitlementNo, UpstreamReference, Status string
	Version                                  int64
}
type UpstreamPrepareInput struct {
	TicketNo                  string
	TicketVersion             int64
	OperationNo               string
	MessageNos, AttachmentNos []string
}
type UpstreamPreview struct {
	ForwardNo, TicketNo, Title, Content, SelectionDigest, PreviewDigest string
	MessageNos, AttachmentNos                                           []string
	DepartmentID                                                        int64
	EntitlementUpstreamRef                                              string
	Version                                                             int64
}
type UpstreamConfirmInput struct {
	ForwardNo, PreviewDigest, OperationNo, Reason string
	ExpectedVersion                               int64
}
type UpstreamForward struct {
	ForwardNo, TicketNo, ProviderTicketReference, Status, ResultClassification, EvidenceCode, PreviewDigest string
	RetryEligible                                                                                           bool
	AttemptCount                                                                                            int
	Version                                                                                                 int64
	UpdatedAt                                                                                               time.Time
}
type UpstreamPublishInput struct {
	TicketNo, LocalMessageNo, Reason string
	TicketVersion                    int64
}

func (s *serviceImpl) UpstreamChannel(ctx context.Context) (*UpstreamChannel, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	return s.loadUpstreamChannel(ctx)
}

func (s *serviceImpl) loadUpstreamChannel(ctx context.Context) (*UpstreamChannel, error) {
	var rows []struct {
		BaseURL, AuthMode, Status, CredentialMaskedTail string
		Version                                         int64
		UpdatedAt                                       time.Time
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT base_url,auth_mode,status_code AS status,credential_masked_tail,version,updated_at FROM plugin_idc_core_support_upstream_channel WHERE tenant_id=0 AND channel_no='legacy-finance-ticket' LIMIT 1`); err != nil {
		return nil, wrapUnavailable(err, "load upstream ticket channel")
	}
	if len(rows) == 0 {
		return &UpstreamChannel{}, nil
	}
	row := rows[0]
	return &UpstreamChannel{BaseURL: row.BaseURL, AuthMode: row.AuthMode, Status: row.Status, CredentialMaskedTail: row.CredentialMaskedTail, Configured: true, Version: row.Version, UpdatedAt: row.UpdatedAt}, nil
}

func (s *serviceImpl) ConfigureUpstreamChannel(ctx context.Context, input UpstreamChannelInput) (*UpstreamChannel, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	if s.upstreamSecrets == nil || s.upstreamSecretAccess == nil || s.sessions == nil || s.newUpstreamClient == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if !businessNoPattern.MatchString(input.OperationNo) || len(strings.TrimSpace(input.Reason)) < 3 || len(input.Reason) > 512 || (input.Status != "enabled" && input.Status != "disabled") {
		return nil, bizInvalid()
	}
	if _, err = s.newUpstreamClient(input.BaseURL, input.AuthMode); err != nil {
		return nil, bizInvalid()
	}
	if err = s.requireRecentSupportLogin(ctx, a.UserID); err != nil {
		return nil, err
	}
	credentials := supportupstream.Credentials{Username: strings.TrimSpace(input.Username), Password: input.Password}
	if credentials.Username == "" || credentials.Password == "" {
		return nil, bizInvalid()
	}
	payload, err := json.Marshal(credentials)
	if err != nil {
		return nil, bizInvalid()
	}
	defer clear(payload)
	var existing []struct {
		SecretRef       string    `orm:"credential_secret_ref"`
		SecretUpdatedAt time.Time `orm:"credential_updated_at"`
		Version         int64
	}
	if err = s.database.GetScan(ctx, &existing, `SELECT credential_secret_ref,credential_updated_at,version FROM plugin_idc_core_support_upstream_channel WHERE tenant_id=0 AND channel_no='legacy-finance-ticket' LIMIT 1`); err != nil {
		return nil, wrapUnavailable(err, "load upstream ticket channel fence")
	}
	var handle secretcap.Handle
	var metadata secretcap.Metadata
	producer := func(consume secretcap.Consumer) error { return consume(payload) }
	if len(existing) == 0 {
		if input.ExpectedVersion != 0 {
			return nil, bizerr.NewCode(CodeConflict)
		}
		created, createErr := s.upstreamSecrets.Create(ctx, input.OperationNo, producer)
		if createErr != nil {
			created, createErr = s.upstreamSecrets.RecoverCreate(ctx, input.OperationNo)
		}
		if createErr != nil {
			return nil, wrapUnavailable(createErr, "store upstream ticket credential")
		}
		handle = created.Handle
		metadata = created.Metadata
	} else {
		if input.ExpectedVersion != existing[0].Version {
			return nil, bizerr.NewCode(CodeConflict)
		}
		handle, err = secretcap.NewHandle(existing[0].SecretRef)
		if err != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		replaced, replaceErr := s.upstreamSecrets.Replace(ctx, handle, existing[0].SecretUpdatedAt, input.OperationNo, producer)
		if replaceErr != nil {
			return nil, wrapUnavailable(replaceErr, "replace upstream ticket credential")
		}
		metadata = replaced.Metadata
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, err
	}
	if len(existing) == 0 {
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_upstream_channel (tenant_id,channel_no,base_url,auth_mode,credential_secret_ref,credential_masked_tail,credential_updated_at,status_code,version,created_by,updated_by,created_at,updated_at) VALUES (0,'legacy-finance-ticket',?,?,?,?,?,?,1,?,?,?,?)`, strings.TrimRight(strings.TrimSpace(input.BaseURL), "/"), input.AuthMode, handle.OpaqueRef(), metadata.MaskedTail, metadata.UpdatedAt, input.Status, a.UserID, a.UserID, now, now)
	} else {
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_upstream_channel SET base_url=?,auth_mode=?,credential_secret_ref=?,credential_masked_tail=?,credential_updated_at=?,status_code=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND channel_no='legacy-finance-ticket' AND version=?`, strings.TrimRight(strings.TrimSpace(input.BaseURL), "/"), input.AuthMode, handle.OpaqueRef(), metadata.MaskedTail, metadata.UpdatedAt, input.Status, a.UserID, now, input.ExpectedVersion)
		err = updateErr
		if err == nil {
			if changed, _ := result.RowsAffected(); changed != 1 {
				err = bizerr.NewCode(CodeConflict)
			}
		}
	}
	if err != nil {
		return nil, wrapUnavailable(err, "save upstream ticket channel")
	}
	return s.UpstreamChannel(ctx)
}

func (s *serviceImpl) UpstreamDepartmentMap(ctx context.Context, departmentNo string) (*UpstreamDepartmentMap, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	if !businessNoPattern.MatchString(departmentNo) {
		return nil, bizInvalid()
	}
	var rows []UpstreamDepartmentMap
	if err := s.database.GetScan(ctx, &rows, `SELECT department_no,upstream_department_id,upstream_department_name,status_code AS status,version FROM plugin_idc_core_support_upstream_department_map WHERE tenant_id=0 AND department_no=? LIMIT 1`, departmentNo); err != nil {
		return nil, wrapUnavailable(err, "load upstream department map")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (s *serviceImpl) SaveUpstreamDepartmentMap(ctx context.Context, input UpstreamDepartmentMapInput) (*UpstreamDepartmentMap, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	if !businessNoPattern.MatchString(input.DepartmentNo) || input.UpstreamDepartmentID <= 0 || input.ExpectedVersion < 0 || (input.Status != "active" && input.Status != "disabled") {
		return nil, bizInvalid()
	}
	var result sql.Result
	if input.ExpectedVersion == 0 {
		result, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_upstream_department_map (tenant_id,department_no,upstream_department_id,upstream_department_name,status_code,version,created_by,updated_by) VALUES (0,?,?,?,?,1,?,?) ON CONFLICT (tenant_id,department_no) DO NOTHING`, input.DepartmentNo, input.UpstreamDepartmentID, strings.TrimSpace(input.UpstreamDepartmentName), input.Status, a.UserID, a.UserID)
	} else {
		result, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_support_upstream_department_map SET upstream_department_id=?,upstream_department_name=?,status_code=?,version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND department_no=? AND version=?`, input.UpstreamDepartmentID, strings.TrimSpace(input.UpstreamDepartmentName), input.Status, a.UserID, input.DepartmentNo, input.ExpectedVersion)
	}
	if err != nil {
		return nil, wrapUnavailable(err, "save upstream department map")
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return s.UpstreamDepartmentMap(ctx, input.DepartmentNo)
}

func (s *serviceImpl) UpstreamEntitlementMap(ctx context.Context, entitlementNo string) (*UpstreamEntitlementMap, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	if !businessNoPattern.MatchString(entitlementNo) {
		return nil, bizInvalid()
	}
	var rows []UpstreamEntitlementMap
	if err := s.database.GetScan(ctx, &rows, `SELECT entitlement_no,upstream_reference,status_code AS status,version FROM plugin_idc_core_support_upstream_entitlement_map WHERE tenant_id=0 AND entitlement_no=? LIMIT 1`, entitlementNo); err != nil {
		return nil, wrapUnavailable(err, "load upstream service map")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (s *serviceImpl) SaveUpstreamEntitlementMap(ctx context.Context, input UpstreamEntitlementMapInput) (*UpstreamEntitlementMap, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	if !businessNoPattern.MatchString(input.EntitlementNo) || !businessNoPattern.MatchString(input.UpstreamReference) || input.ExpectedVersion < 0 || (input.Status != "active" && input.Status != "disabled") {
		return nil, bizInvalid()
	}
	var result sql.Result
	if input.ExpectedVersion == 0 {
		result, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_upstream_entitlement_map (tenant_id,entitlement_no,upstream_reference,status_code,version,created_by,updated_by) VALUES (0,?,?,?,1,?,?) ON CONFLICT (tenant_id,entitlement_no) DO NOTHING`, input.EntitlementNo, input.UpstreamReference, input.Status, a.UserID, a.UserID)
	} else {
		result, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_support_upstream_entitlement_map SET upstream_reference=?,status_code=?,version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND entitlement_no=? AND version=?`, input.UpstreamReference, input.Status, a.UserID, input.EntitlementNo, input.ExpectedVersion)
	}
	if err != nil {
		return nil, wrapUnavailable(err, "save upstream service map")
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return s.UpstreamEntitlementMap(ctx, input.EntitlementNo)
}

func (s *serviceImpl) PrepareUpstream(ctx context.Context, input UpstreamPrepareInput) (*UpstreamPreview, error) {
	a, err := s.staffActor(ctx, PermissionUpstream)
	if err != nil {
		return nil, err
	}
	if !businessNoPattern.MatchString(input.TicketNo) || !businessNoPattern.MatchString(input.OperationNo) || input.TicketVersion <= 0 || len(input.MessageNos)+len(input.AttachmentNos) == 0 || len(input.MessageNos)+len(input.AttachmentNos) > maxUpstreamSelectionItems {
		return nil, bizInvalid()
	}
	ticket, err := s.staffAccessibleTicket(ctx, input.TicketNo, a)
	if err != nil {
		return nil, err
	}
	if ticket.Version != input.TicketVersion {
		return nil, bizerr.NewCode(CodeConflict)
	}
	channel, departmentID, upstreamRef, err := s.upstreamAuthority(ctx, ticket)
	if err != nil {
		return nil, err
	}
	_ = channel
	messages, attachments, content, err := s.upstreamSelection(ctx, ticket, input.MessageNos, input.AttachmentNos)
	if err != nil {
		return nil, err
	}
	selectionDigest := digest(strings.Join(append(append([]string{}, messages...), attachments...), "\x00"))
	title := "[" + ticket.TicketNo + "] " + ticket.Subject
	previewDigest := digest(title + "\x00" + content + "\x00" + selectionDigest + "\x00" + strconv.FormatInt(departmentID, 10) + "\x00" + upstreamRef)
	forwardNo, err := s.newNo("UPF_")
	if err != nil {
		return nil, err
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		_, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_upstream_forward (tenant_id,forward_no,ticket_no,provider_id,selection_digest,preview_digest,status_code,operation_no,request_digest,upstream_department_id,entitlement_upstream_ref,version,created_at,updated_at) VALUES (0,?,?,'legacy-finance-ticket',?,?,'prepared',?,?,?,?,1,?,?)`, forwardNo, ticket.TicketNo, selectionDigest, previewDigest, input.OperationNo, digest(input.OperationNo+"\x00"+previewDigest), departmentID, nullable(upstreamRef), now, now)
		if insertErr != nil {
			return insertErr
		}
		sortOrder := 0
		for _, value := range messages {
			sortOrder++
			if _, insertErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_upstream_item (tenant_id,forward_no,item_type,item_no,sort_order) VALUES (0,?,'message',?,?)`, forwardNo, value, sortOrder); insertErr != nil {
				return insertErr
			}
		}
		for _, value := range attachments {
			sortOrder++
			if _, insertErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_upstream_item (tenant_id,forward_no,item_type,item_no,sort_order) VALUES (0,?,'attachment',?,?)`, forwardNo, value, sortOrder); insertErr != nil {
				return insertErr
			}
		}
		return nil
	})
	if err != nil {
		return nil, wrapUnavailable(err, "prepare upstream ticket preview")
	}
	return &UpstreamPreview{ForwardNo: forwardNo, TicketNo: ticket.TicketNo, Title: title, Content: content, SelectionDigest: selectionDigest, PreviewDigest: previewDigest, MessageNos: messages, AttachmentNos: attachments, DepartmentID: departmentID, EntitlementUpstreamRef: upstreamRef, Version: 1}, nil
}

func (s *serviceImpl) ConfirmUpstream(ctx context.Context, input UpstreamConfirmInput) (*UpstreamForward, error) {
	return s.executeUpstream(ctx, input, false)
}
func (s *serviceImpl) RetryUpstream(ctx context.Context, input UpstreamConfirmInput) (*UpstreamForward, error) {
	return s.executeUpstream(ctx, input, true)
}

func (s *serviceImpl) executeUpstream(ctx context.Context, input UpstreamConfirmInput, retry bool) (*UpstreamForward, error) {
	a, err := s.staffActor(ctx, PermissionUpstream)
	if err != nil {
		return nil, err
	}
	reason, ok := normalizeText(input.Reason, 512)
	if !ok || !businessNoPattern.MatchString(input.ForwardNo) || !businessNoPattern.MatchString(input.OperationNo) || len(input.PreviewDigest) != 64 || input.ExpectedVersion <= 0 {
		return nil, bizInvalid()
	}
	var row struct {
		ForwardNo, TicketNo, PreviewDigest, Status, ProviderRef, BaseURL, AuthMode, SecretRef string
		DepartmentID                                                                          int64
		UpstreamRef                                                                           string
		AttemptCount                                                                          int
		Version                                                                               int64
		RetryEligible                                                                         bool
		SecretUpdatedAt                                                                       time.Time
	}
	if err = s.database.GetScan(ctx, &row, `SELECT f.forward_no,f.ticket_no,f.preview_digest,f.status_code AS status,COALESCE(f.provider_ticket_reference,'') AS provider_ref,f.upstream_department_id AS department_id,COALESCE(f.entitlement_upstream_ref,'') AS upstream_ref,f.attempt_count,f.version,f.retry_eligible,c.base_url,c.auth_mode,c.credential_secret_ref AS secret_ref,c.credential_updated_at AS secret_updated_at FROM plugin_idc_core_support_ticket_upstream_forward f JOIN plugin_idc_core_support_upstream_channel c ON c.tenant_id=0 AND c.channel_no='legacy-finance-ticket' AND c.status_code='enabled' WHERE f.tenant_id=0 AND f.forward_no=?`, input.ForwardNo); err != nil {
		return nil, wrapUnavailable(err, "load upstream forward")
	}
	if row.ForwardNo == "" || row.Version != input.ExpectedVersion || row.PreviewDigest != input.PreviewDigest || (!retry && row.Status != "prepared") || (retry && (row.Status != "definitive_failure" || !row.RetryEligible)) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	ticket, err := s.staffAccessibleTicket(ctx, row.TicketNo, a)
	if err != nil {
		return nil, err
	}
	messages, attachments, content, err := s.forwardSelection(ctx, ticket, row.ForwardNo)
	if err != nil {
		return nil, err
	}
	_ = messages
	client, err := s.newUpstreamClient(row.BaseURL, row.AuthMode)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	handle, err := secretcap.NewHandle(row.SecretRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	operationDigest := digest(input.OperationNo + "\x00" + input.ForwardNo + "\x00" + strconv.FormatInt(input.ExpectedVersion, 10))
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_upstream_forward SET status_code='submitted',retry_eligible=FALSE,confirmed_by=?,confirmed_at=CURRENT_TIMESTAMP,confirmed_reason=?,request_digest=?,attempt_count=attempt_count+1,last_attempt_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND forward_no=? AND version=? AND status_code=?`, a.UserID, reason, operationDigest, row.ForwardNo, row.Version, row.Status)
	if err != nil {
		return nil, wrapUnavailable(err, "claim upstream forward")
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	files := make([]supportupstream.Attachment, 0, len(attachments))
	for _, attachmentNo := range attachments {
		opened, openErr := s.OpenStaffAttachment(ctx, attachmentNo)
		if openErr != nil {
			return s.finishUpstream(ctx, row, supportupstream.Result{Classification: supportupstream.ResultUnknown, EvidenceCode: "selected_attachment_unavailable"}, retry, reason, ticket.CustomerNo)
		}
		data, readErr := io.ReadAll(io.LimitReader(opened.Body, maxAttachmentBytes+1))
		closeErr := opened.Body.Close()
		if readErr != nil || closeErr != nil || int64(len(data)) > maxAttachmentBytes {
			return s.finishUpstream(ctx, row, supportupstream.Result{Classification: supportupstream.ResultUnknown, EvidenceCode: "selected_attachment_read_failed"}, retry, reason, ticket.CustomerNo)
		}
		files = append(files, supportupstream.Attachment{FileName: opened.FileName, MediaType: opened.MediaType, Content: data})
	}
	credentials := supportupstream.Credentials{}
	purpose, err := secretcap.NewPurpose("core.support-upstream-send")
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	accessRequest, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	remoteResult := supportupstream.Result{Classification: supportupstream.ResultUnknown, EvidenceCode: "credential_access_failed"}
	accessErr := s.upstreamSecretAccess.Access(ctx, accessRequest, func(plaintext []byte) error {
		if json.Unmarshal(plaintext, &credentials) != nil {
			return bizInvalid()
		}
		remoteResult = client.Create(ctx, credentials, supportupstream.CreateRequest{DepartmentID: row.DepartmentID, HostID: row.UpstreamRef, Title: "[" + ticket.TicketNo + "] " + ticket.Subject, Content: content, Priority: ticket.Priority, Attachments: files})
		credentials.Password = ""
		return nil
	})
	if accessErr != nil {
		remoteResult = supportupstream.Result{Classification: supportupstream.ResultUnknown, EvidenceCode: "credential_access_failed"}
	}
	return s.finishUpstream(ctx, row, remoteResult, retry, reason, ticket.CustomerNo)
}

func (s *serviceImpl) finishUpstream(ctx context.Context, row struct {
	ForwardNo, TicketNo, PreviewDigest, Status, ProviderRef, BaseURL, AuthMode, SecretRef string
	DepartmentID                                                                          int64
	UpstreamRef                                                                           string
	AttemptCount                                                                          int
	Version                                                                               int64
	RetryEligible                                                                         bool
	SecretUpdatedAt                                                                       time.Time
}, remote supportupstream.Result, retry bool, reason, customerNo string) (*UpstreamForward, error) {
	auditResult := "result_unknown"
	status := "result_unknown"
	retryEligible := false
	if remote.Classification == supportupstream.ResultAccepted {
		status = "succeeded"
		auditResult = idccap.BusinessAuditResultSucceeded
	} else if remote.Classification == supportupstream.ResultDefinitiveNotReceived {
		status = "definitive_failure"
		auditResult = "failed"
		retryEligible = true
	}
	action := "create"
	if retry {
		action = "retry"
	}
	attemptNo, err := s.newNo("UPA_")
	if err != nil {
		return nil, err
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, err
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_upstream_attempt (tenant_id,attempt_no,forward_no,attempt_seq,action_code,result_classification,http_status,evidence_code,provider_ticket_reference,started_at,finished_at) VALUES (0,?,?,?,?,?,?,?,?,?,?)`, attemptNo, row.ForwardNo, row.AttemptCount+1, action, remote.Classification, nullableInt(int64(remote.HTTPStatus)), remote.EvidenceCode, nullable(remote.TicketReference), now, now)
		if insertErr != nil {
			return insertErr
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_upstream_forward SET provider_ticket_reference=COALESCE(NULLIF(?,''),provider_ticket_reference),status_code=?,result_classification=?,retry_eligible=?,last_evidence_code=?,version=version+1,updated_at=? WHERE tenant_id=0 AND forward_no=? AND status_code='submitted'`, remote.TicketReference, status, remote.Classification, retryEligible, remote.EvidenceCode, now, row.ForwardNo)
		if updateErr != nil {
			return updateErr
		}
		changed, changedErr := result.RowsAffected()
		if changedErr != nil || changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if s.audit == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "support.ticket.upstream-forward",
			IdempotencyKey:   attemptNo,
			OperationCode:    "support.ticket.upstream-forward." + action,
			TargetType:       "support_ticket", TargetNo: row.TicketNo, CustomerNo: customerNo,
			ResultCode: auditResult, ReasonCode: reason, RiskCode: "high",
			RequiredPermission: PermissionUpstream, CorrelationNo: row.ForwardNo,
			BeforeSnapshot: map[string]any{"forward_no": row.ForwardNo, "status": "submitted", "attempt": row.AttemptCount + 1},
			AfterSnapshot:  map[string]any{"status": status, "classification": remote.Classification, "retry_eligible": retryEligible},
		})
	})
	if err != nil {
		return nil, wrapUnavailable(err, "finish upstream forward")
	}
	return s.UpstreamForward(ctx, row.ForwardNo)
}

func (s *serviceImpl) UpstreamForward(ctx context.Context, forwardNo string) (*UpstreamForward, error) {
	a, err := s.staffActor(ctx, PermissionUpstream)
	if err != nil {
		return nil, err
	}
	var item UpstreamForward
	err = s.database.GetScan(ctx, &item, `SELECT forward_no,ticket_no,COALESCE(provider_ticket_reference,'') AS provider_ticket_reference,status_code AS status,COALESCE(result_classification,'') AS result_classification,COALESCE(last_evidence_code,'') AS evidence_code,preview_digest,retry_eligible,attempt_count,version,updated_at FROM plugin_idc_core_support_ticket_upstream_forward WHERE tenant_id=0 AND forward_no=?`, forwardNo)
	if err != nil || item.ForwardNo == "" {
		return nil, bizNotAccessible()
	}
	if _, err = s.staffAccessibleTicket(ctx, item.TicketNo, a); err != nil {
		return nil, bizNotAccessible()
	}
	return &item, nil
}

func (s *serviceImpl) UpstreamForwards(ctx context.Context, ticketNo string) ([]UpstreamForward, error) {
	a, err := s.staffActor(ctx, PermissionUpstream)
	if err != nil {
		return nil, err
	}
	_, err = s.staffAccessibleTicket(ctx, ticketNo, a)
	if err != nil {
		return nil, err
	}
	var items []UpstreamForward
	if err = s.database.GetScan(ctx, &items, `SELECT forward_no,ticket_no,COALESCE(provider_ticket_reference,'') AS provider_ticket_reference,status_code AS status,COALESCE(result_classification,'') AS result_classification,COALESCE(last_evidence_code,'') AS evidence_code,preview_digest,retry_eligible,attempt_count,version,updated_at FROM plugin_idc_core_support_ticket_upstream_forward WHERE tenant_id=0 AND ticket_no=? ORDER BY id DESC LIMIT 20`, ticketNo); err != nil {
		return nil, wrapUnavailable(err, "load ticket upstream forwards")
	}
	return items, nil
}

func (s *serviceImpl) SyncUpstreamReplies(ctx context.Context, forwardNo string, expectedVersion int64) (int, error) {
	a, err := s.staffActor(ctx, PermissionUpstream)
	if err != nil {
		return 0, err
	}
	var row struct {
		ForwardNo, TicketNo, ProviderRef, BaseURL, AuthMode, SecretRef string
		Version                                                        int64
	}
	if err = s.database.GetScan(ctx, &row, `SELECT f.forward_no,f.ticket_no,COALESCE(f.provider_ticket_reference,'') AS provider_ref,f.version,c.base_url,c.auth_mode,c.credential_secret_ref AS secret_ref FROM plugin_idc_core_support_ticket_upstream_forward f JOIN plugin_idc_core_support_upstream_channel c ON c.tenant_id=0 AND c.channel_no='legacy-finance-ticket' AND c.status_code='enabled' WHERE f.tenant_id=0 AND f.forward_no=? AND f.status_code='succeeded'`, forwardNo); err != nil || row.ForwardNo == "" || row.Version != expectedVersion || row.ProviderRef == "" {
		return 0, bizerr.NewCode(CodeConflict)
	}
	ticket, err := s.staffAccessibleTicket(ctx, row.TicketNo, a)
	if err != nil {
		return 0, err
	}
	client, err := s.newUpstreamClient(row.BaseURL, row.AuthMode)
	if err != nil {
		return 0, bizerr.NewCode(CodeUnavailable)
	}
	handle, err := secretcap.NewHandle(row.SecretRef)
	if err != nil {
		return 0, bizerr.NewCode(CodeUnavailable)
	}
	var replies []supportupstream.Reply
	remote := supportupstream.Result{}
	purpose, err := secretcap.NewPurpose("core.support-upstream-query")
	if err != nil {
		return 0, bizerr.NewCode(CodeUnavailable)
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return 0, bizerr.NewCode(CodeUnavailable)
	}
	accessErr := s.upstreamSecretAccess.Access(ctx, request, func(plaintext []byte) error {
		var credentials supportupstream.Credentials
		if json.Unmarshal(plaintext, &credentials) != nil {
			return bizInvalid()
		}
		replies, remote = client.Detail(ctx, credentials, row.ProviderRef)
		credentials.Password = ""
		return nil
	})
	if accessErr != nil || remote.Classification != supportupstream.ResultAccepted {
		return 0, bizerr.NewCode(CodeUnavailable)
	}
	auditKey, err := s.newNo("UPSQ_")
	if err != nil {
		return 0, err
	}
	inserted := 0
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		for _, reply := range replies {
			body := strings.TrimSpace(reply.Content)
			if body == "" || len(body) > maxTicketBodyBytes {
				continue
			}
			messageNo, noErr := s.newNo("MSG_")
			if noErr != nil {
				return noErr
			}
			syncNo, noErr := s.newNo("UPS_")
			if noErr != nil {
				return noErr
			}
			digestValue := digest(reply.Reference + "\x00" + body)
			result, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_message (tenant_id,message_no,ticket_no,message_type,visibility_code,author_type,author_user_id,body,automated,created_at) SELECT 0,?,?,'upstream_reply','internal_only','upstream',0,?,FALSE,? WHERE NOT EXISTS (SELECT 1 FROM plugin_idc_core_support_upstream_reply WHERE forward_no=? AND provider_reply_reference=?)`, messageNo, ticket.TicketNo, body, reply.CreatedAt, forwardNo, reply.Reference)
			if insertErr != nil {
				return insertErr
			}
			changed, changedErr := result.RowsAffected()
			if changedErr != nil {
				return changedErr
			}
			if changed == 0 {
				continue
			}
			if _, insertErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_upstream_reply (tenant_id,sync_no,forward_no,ticket_no,provider_reply_reference,upstream_content_digest,local_message_no,synced_at) VALUES (0,?,?,?,?,?,?,CURRENT_TIMESTAMP)`, syncNo, forwardNo, ticket.TicketNo, reply.Reference, digestValue, messageNo); insertErr != nil {
				return insertErr
			}
			inserted++
		}
		if inserted == 0 {
			return nil
		}
		if s.audit == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "support.ticket.upstream-reply-sync",
			IdempotencyKey:   auditKey,
			OperationCode:    "support.ticket.upstream-reply-sync",
			TargetType:       "support_ticket", TargetNo: ticket.TicketNo, CustomerNo: ticket.CustomerNo,
			ResultCode: "succeeded", ReasonCode: "manual_query", RiskCode: "medium",
			RequiredPermission: PermissionUpstream, CorrelationNo: forwardNo,
			BeforeSnapshot: map[string]any{"forward_version": expectedVersion},
			AfterSnapshot:  map[string]any{"new_internal_replies": inserted},
		})
	})
	return inserted, wrapUnavailable(err, "sync upstream replies")
}

func (s *serviceImpl) PublishUpstreamReply(ctx context.Context, input UpstreamPublishInput) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionReply)
	if err != nil {
		return nil, err
	}
	reason, ok := normalizeText(input.Reason, 512)
	if !ok || !businessNoPattern.MatchString(input.TicketNo) || !businessNoPattern.MatchString(input.LocalMessageNo) || input.TicketVersion <= 0 {
		return nil, bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, loadErr := s.lockAccessibleTicket(ctx, input.TicketNo, a, "")
		if loadErr != nil {
			return loadErr
		}
		if ticket.Version != input.TicketVersion || ticket.BaseStage == StageClosed {
			return bizerr.NewCode(CodeConflict)
		}
		var rows []struct {
			Body   string
			SyncNo string
		}
		if loadErr = s.database.GetScan(ctx, &rows, `SELECT m.body,r.sync_no FROM plugin_idc_core_support_ticket_message m JOIN plugin_idc_core_support_upstream_reply r ON r.local_message_no=m.message_no AND r.published_at IS NULL WHERE m.tenant_id=0 AND m.ticket_no=? AND m.message_no=? AND m.message_type='upstream_reply' AND m.visibility_code='internal_only' AND m.hidden_at IS NULL FOR UPDATE`, input.TicketNo, input.LocalMessageNo); loadErr != nil || len(rows) != 1 {
			return bizNotAccessible()
		}
		messageNo, noErr := s.newNo("MSG_")
		if noErr != nil {
			return noErr
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if _, loadErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_message (tenant_id,message_no,ticket_no,message_type,visibility_code,author_type,author_user_id,body,automated,created_at) VALUES (0,? ,?,'public_reply','customer_visible','staff',?,?,FALSE,?)`, messageNo, input.TicketNo, a.UserID, rows[0].Body, now); loadErr != nil {
			return loadErr
		}
		if _, loadErr = s.database.Exec(ctx, `UPDATE plugin_idc_core_support_upstream_reply SET published_message_no=?,published_by=?,published_at=? WHERE sync_no=? AND published_at IS NULL`, messageNo, a.UserID, now, rows[0].SyncNo); loadErr != nil {
			return loadErr
		}
		result, loadErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET version=version+1,last_activity_at=?,updated_by=?,updated_at=? WHERE ticket_no=? AND version=?`, now, a.UserID, now, input.TicketNo, input.TicketVersion)
		if loadErr != nil {
			return loadErr
		}
		changed, changedErr := result.RowsAffected()
		if changedErr != nil {
			return changedErr
		}
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if s.audit == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		if loadErr = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "support.ticket.upstream-reply-publication",
			IdempotencyKey:   input.TicketNo + ":" + input.LocalMessageNo,
			OperationCode:    "support.ticket.upstream-reply-publication",
			TargetType:       "support_ticket", TargetNo: input.TicketNo, CustomerNo: ticket.CustomerNo,
			ResultCode: "succeeded", ReasonCode: reason, RiskCode: "medium",
			RequiredPermission: PermissionUpstream, CorrelationNo: rows[0].SyncNo,
			BeforeSnapshot: map[string]any{"message_no": input.LocalMessageNo, "visibility": "internal_only", "ticket_version": ticket.Version},
			AfterSnapshot:  map[string]any{"message_no": messageNo, "visibility": "customer_visible", "ticket_version": ticket.Version + 1},
		}); loadErr != nil {
			return loadErr
		}
		return s.enqueueCustomerTicketEvent(ctx, ticket, supportEventTicketPublicUpdated, ticket.Version+1)
	})
	if err != nil {
		return nil, wrapUnavailable(err, "publish upstream reply")
	}
	return s.DetailStaff(ctx, input.TicketNo)
}

func (s *serviceImpl) requireRecentSupportLogin(ctx context.Context, userID int) error {
	session, err := s.sessions.Current(ctx)
	if err != nil || session == nil || session.LoginAt == nil || session.UserID != strconv.Itoa(userID) {
		return bizNotAccessible()
	}
	now := s.now().UTC()
	loginAt := session.LoginAt.UTC()
	if loginAt.After(now.Add(time.Minute)) {
		return bizNotAccessible()
	}
	return nil
}
func (s *serviceImpl) upstreamAuthority(ctx context.Context, ticket *Ticket) (*UpstreamChannel, int64, string, error) {
	channel, err := s.loadUpstreamChannel(ctx)
	if err != nil || !channel.Configured || channel.Status != "enabled" {
		return nil, 0, "", bizerr.NewCode(CodeUnavailable)
	}
	var departmentID int64
	if err = s.database.GetScan(ctx, &departmentID, `SELECT upstream_department_id FROM plugin_idc_core_support_upstream_department_map WHERE tenant_id=0 AND department_no=? AND status_code='active'`, ticket.DepartmentNo); err != nil || departmentID <= 0 {
		return nil, 0, "", bizerr.NewCode(CodeUnavailable)
	}
	upstreamRef := ""
	if ticket.RelatedEntitlementNo != "" {
		if err = s.database.GetScan(ctx, &upstreamRef, `SELECT upstream_reference FROM plugin_idc_core_support_upstream_entitlement_map WHERE tenant_id=0 AND entitlement_no=? AND status_code='active'`, ticket.RelatedEntitlementNo); err != nil || upstreamRef == "" {
			return nil, 0, "", bizerr.NewCode(CodeUnavailable)
		}
	}
	return channel, departmentID, upstreamRef, nil
}
func (s *serviceImpl) staffAccessibleTicket(ctx context.Context, ticketNo string, staff *actor) (*Ticket, error) {
	if staff == nil || !businessNoPattern.MatchString(ticketNo) {
		return nil, bizNotAccessible()
	}
	var where strings.Builder
	where.WriteString(" WHERE t.tenant_id=0 AND t.ticket_no=?")
	args := []any{ticketNo}
	appendStaffAccess(&where, &args, staff, "t")
	rows, err := scanTickets(ctx, s.database, "SELECT "+ticketProjection("t", "c", "customer")+" FROM plugin_idc_core_support_ticket t LEFT JOIN plugin_idc_core_support_issue_category c ON c.tenant_id=0 AND c.category_no=t.category_no LEFT JOIN plugin_idc_core_customer_subject customer ON customer.tenant_id=0 AND customer.customer_no=t.customer_no"+where.String()+" LIMIT 1", args...)
	if err != nil || len(rows) != 1 {
		return nil, bizNotAccessible()
	}
	return rows[0], nil
}

func (s *serviceImpl) upstreamSelection(ctx context.Context, ticket *Ticket, messageNos, attachmentNos []string) ([]string, []string, string, error) {
	messageNos = uniqueSorted(messageNos)
	attachmentNos = uniqueSorted(attachmentNos)
	if len(messageNos)+len(attachmentNos) == 0 {
		return nil, nil, "", bizInvalid()
	}
	var messages []struct{ MessageNo, Body string }
	if len(messageNos) > 0 {
		args := []any{ticket.TicketNo}
		for _, value := range messageNos {
			args = append(args, value)
		}
		query := `SELECT message_no,body FROM plugin_idc_core_support_ticket_message WHERE tenant_id=0 AND ticket_no=? AND visibility_code='customer_visible' AND hidden_at IS NULL AND message_no IN (` + sqlPlaceholders(len(messageNos)) + `) ORDER BY created_at,id`
		if err := s.database.GetScan(ctx, &messages, query, args...); err != nil || len(messages) != len(messageNos) {
			return nil, nil, "", bizNotAccessible()
		}
	}
	var attachments []struct{ AttachmentNo, FileName string }
	if len(attachmentNos) > 0 {
		args := []any{ticket.TicketNo}
		for _, value := range attachmentNos {
			args = append(args, value)
		}
		query := `SELECT attachment_no,file_name FROM plugin_idc_core_support_ticket_attachment WHERE tenant_id=0 AND ticket_no=? AND visibility_code='customer_visible' AND hidden_at IS NULL AND attachment_no IN (` + sqlPlaceholders(len(attachmentNos)) + `) ORDER BY created_at,id`
		if err := s.database.GetScan(ctx, &attachments, query, args...); err != nil || len(attachments) != len(attachmentNos) {
			return nil, nil, "", bizNotAccessible()
		}
	}
	var content strings.Builder
	for _, message := range messages {
		content.WriteString(message.Body)
		content.WriteString("\n\n")
	}
	if len(attachments) > 0 {
		content.WriteString("Attachments: ")
		for index, attachment := range attachments {
			if index > 0 {
				content.WriteString(", ")
			}
			content.WriteString(attachment.FileName)
		}
	}
	normalized := strings.TrimSpace(content.String())
	if normalized == "" || len(normalized) > 10000 {
		return nil, nil, "", bizInvalid()
	}
	return messageNos, attachmentNos, normalized, nil
}
func (s *serviceImpl) forwardSelection(ctx context.Context, ticket *Ticket, forwardNo string) ([]string, []string, string, error) {
	var rows []struct {
		Type, No  string
		SortOrder int
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT item_type AS type,item_no AS no,sort_order FROM plugin_idc_core_support_ticket_upstream_item WHERE tenant_id=0 AND forward_no=? ORDER BY sort_order`, forwardNo); err != nil {
		return nil, nil, "", err
	}
	messages := []string{}
	attachments := []string{}
	for _, row := range rows {
		if row.Type == "message" {
			messages = append(messages, row.No)
		} else {
			attachments = append(attachments, row.No)
		}
	}
	return s.upstreamSelection(ctx, ticket, messages, attachments)
}
func uniqueSorted(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	write := 0
	for _, value := range result {
		if !businessNoPattern.MatchString(value) || (write > 0 && result[write-1] == value) {
			continue
		}
		result[write] = value
		write++
	}
	return result[:write]
}
func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
