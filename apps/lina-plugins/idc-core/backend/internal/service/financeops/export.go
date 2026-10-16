package financeops

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/xuri/excelize/v2"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

const (
	exportLifetime       = 24 * time.Hour
	exportTicketLifetime = 10 * time.Minute
	maximumExportRows    = 100_000
	maximumExportBytes   = 50 * 1024 * 1024
)

type ExportRequestInput struct {
	ExportType     string
	Filter         json.RawMessage
	IdempotencyKey string
}

type ExportStatus struct {
	RequestNo, ExportType, Status, FailureCode string
	RequestedAt                                time.Time
	GeneratedAt, ExpiresAt                     *time.Time
	RowCount                                   int
	Reused                                     bool
}

type ExportTicket struct {
	Token     string
	ExpiresAt time.Time
}

type ExportDownload struct {
	FileName, MediaType string
	Size                int64
	Body                io.ReadCloser
}

func (s *serviceImpl) CreateExport(ctx context.Context, input ExportRequestInput) (*ExportStatus, error) {
	authorization, permission, err := s.authorizeExport(ctx, input.ExportType)
	filter, filterErr := normalizeJSONObject(input.Filter)
	if err != nil {
		return nil, err
	}
	if filterErr != nil || !validIdempotency(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	idempotencyDigest := digestString(input.IdempotencyKey)
	filterDigest := digestString(string(filter))
	var replay *exportRequestRow
	if err = s.database.GetScan(ctx, &replay, exportRequestSelect+` AND requested_by=? AND export_type_code=? AND idempotency_digest=?`, authorization.UserID, input.ExportType, idempotencyDigest); err != nil {
		return nil, normalize(err)
	}
	if replay != nil {
		if replay.FilterDigest != filterDigest || replay.DataScope != authorization.DataScope {
			return nil, bizerr.NewCode(CodeConflict)
		}
		status := exportStatusFromRow(replay)
		status.Reused = true
		return status, nil
	}
	requestNo, err := newNumber(s.random, "FEX_")
	if err != nil {
		return nil, normalize(err)
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, normalize(err)
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_financial_export_request (tenant_id,request_no,export_type_code,filter_json,filter_digest,permission_code,data_scope_code,requested_by,idempotency_digest,authorization_checked_at,status_code,created_at) VALUES (0,?,?,?,?,?,?,?,?,?,'queued',?)`, requestNo, input.ExportType, string(filter), filterDigest, permission, authorization.DataScope, authorization.UserID, idempotencyDigest, now, now); insertErr != nil {
			return insertErr
		}
		return s.audit.AppendBusinessTransition(ctx, idccapTransition("billing.financial_export.create", input.IdempotencyKey, requestNo, permission, "queued", input.ExportType, authorization.DataScope))
	})
	if err != nil {
		return nil, normalize(err)
	}
	return &ExportStatus{RequestNo: requestNo, ExportType: input.ExportType, Status: "queued", RequestedAt: now.UTC()}, nil
}

func (s *serviceImpl) GenerateExport(ctx context.Context, requestNo string) (*ExportStatus, error) {
	row, authorization, _, err := s.authorizeOwnedExport(ctx, requestNo)
	if err != nil {
		return nil, err
	}
	if row.Status == "ready" {
		return exportStatusFromRow(row), nil
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_financial_export_request SET status_code='generating',failure_code='',authorization_checked_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND request_no=? AND requested_by=? AND data_scope_code=? AND status_code IN ('queued','failed')`, requestNo, authorization.UserID, authorization.DataScope)
	if err != nil {
		return nil, normalize(err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	content, count, buildErr := s.buildExportWorkbook(ctx, row.ExportType, row.Filter)
	if buildErr != nil {
		_, _ = s.database.Exec(ctx, `UPDATE plugin_idc_core_financial_export_request SET status_code='failed',failure_code='generation_failed' WHERE tenant_id=0 AND request_no=? AND status_code='generating'`, requestNo)
		return nil, normalize(buildErr)
	}
	if len(content) == 0 || len(content) > maximumExportBytes {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	digestValue := sha256.Sum256(content)
	digest := hex.EncodeToString(digestValue[:])
	objectRef := fmt.Sprintf("private/financial-exports/%s/%s.xlsx", requestNo, digest)
	if _, err = s.storage.Put(ctx, storagecap.PutInput{Path: objectRef, Body: bytes.NewReader(content), Size: int64(len(content)), ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", Overwrite: false}); err != nil {
		stat, statErr := s.storage.Stat(ctx, storagecap.StatInput{Path: objectRef})
		if statErr != nil || !stat.Found || stat.Object == nil || stat.Object.Size != int64(len(content)) {
			return nil, normalize(err)
		}
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, normalize(err)
	}
	expiresAt := now.Add(exportLifetime)
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		// Generation-time permission was checked immediately before the bounded
		// projection was read. Persist that check with the resulting artifact.
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_financial_export_artifact (tenant_id,request_no,object_ref,content_sha256,size_bytes,row_count,status_code,expires_at,created_at,updated_at) VALUES (0,?,?,?,?,?,'ready',?,?,?) ON CONFLICT (request_no) DO NOTHING`, requestNo, objectRef, digest, len(content), count, expiresAt, now, now); insertErr != nil {
			return insertErr
		}
		updated, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_financial_export_request SET status_code='ready',generated_at=?,authorization_checked_at=?,failure_code='' WHERE tenant_id=0 AND request_no=? AND requested_by=? AND data_scope_code=? AND status_code='generating'`, now, now, requestNo, authorization.UserID, authorization.DataScope)
		if updateErr != nil {
			return updateErr
		}
		updatedCount, _ := updated.RowsAffected()
		if updatedCount != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return s.audit.AppendBusinessTransition(ctx, idccapTransition("billing.financial_export.generate", requestNo, requestNo, row.PermissionCode, "ready", row.ExportType, authorization.DataScope))
	})
	if err != nil {
		_ = s.storage.Delete(ctx, storagecap.DeleteInput{Path: objectRef})
		return nil, normalize(err)
	}
	generated, _, _, err := s.authorizeOwnedExport(ctx, requestNo)
	return exportStatusFromRow(generated), err
}

func (s *serviceImpl) GetExportStatus(ctx context.Context, requestNo string) (*ExportStatus, error) {
	row, _, _, err := s.authorizeOwnedExport(ctx, requestNo)
	if err != nil {
		return nil, err
	}
	return exportStatusFromRow(row), nil
}

func (s *serviceImpl) IssueExportTicket(ctx context.Context, requestNo string) (*ExportTicket, error) {
	row, authorization, permission, err := s.authorizeOwnedExport(ctx, requestNo)
	if err != nil || row.Status != "ready" {
		return nil, inaccessible(err)
	}
	raw := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, raw); err != nil {
		return nil, normalize(err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	clear(raw)
	tokenDigest := digestString(token)
	ticketNo, err := newNumber(s.random, "FET_")
	if err != nil {
		return nil, normalize(err)
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, normalize(err)
	}
	expiresAt := now.Add(exportTicketLifetime)
	if row.ExpiresAt != nil && expiresAt.After(*row.ExpiresAt) {
		expiresAt = *row.ExpiresAt
	}
	result, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_financial_export_ticket (tenant_id,ticket_no,token_digest,request_no,requested_by,permission_code,data_scope_code,expires_at,created_at) SELECT 0,?,?,?,?,?,?,?,? FROM plugin_idc_core_financial_export_artifact artifact WHERE artifact.tenant_id=0 AND artifact.request_no=? AND artifact.status_code='ready' AND artifact.expires_at>?`, ticketNo, tokenDigest, requestNo, authorization.UserID, permission, authorization.DataScope, expiresAt, now, requestNo, now)
	if err != nil {
		return nil, normalize(err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return &ExportTicket{Token: token, ExpiresAt: expiresAt.UTC()}, nil
}

func (s *serviceImpl) OpenExport(ctx context.Context, token string) (*ExportDownload, error) {
	token = strings.TrimSpace(token)
	if len(token) < 32 || len(token) > 128 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	tokenDigest := digestString(token)
	var ticket *struct {
		RequestNo, ExportType, PermissionCode, ObjectRef, ContentSHA256 string
		RequestedBy, DataScope                                          int
		SizeBytes                                                       int64
	}
	if err := s.database.GetScan(ctx, &ticket, `SELECT ticket.request_no,request.export_type_code AS export_type,request.permission_code,ticket.requested_by,ticket.data_scope_code AS data_scope,artifact.object_ref,artifact.content_sha256,artifact.size_bytes FROM plugin_idc_core_financial_export_ticket ticket JOIN plugin_idc_core_financial_export_request request ON request.tenant_id=ticket.tenant_id AND request.request_no=ticket.request_no JOIN plugin_idc_core_financial_export_artifact artifact ON artifact.tenant_id=ticket.tenant_id AND artifact.request_no=ticket.request_no WHERE ticket.tenant_id=0 AND ticket.token_digest=? AND ticket.consumed_at IS NULL AND ticket.expires_at>CURRENT_TIMESTAMP AND artifact.status_code='ready' AND artifact.expires_at>CURRENT_TIMESTAMP`, tokenDigest); err != nil || ticket == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	authorization, permission, err := s.authorizeExport(ctx, ticket.ExportType)
	if err != nil || authorization.UserID != ticket.RequestedBy || authorization.DataScope != ticket.DataScope || permission != ticket.PermissionCode {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_financial_export_ticket SET consumed_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND token_digest=? AND consumed_at IS NULL AND expires_at>CURRENT_TIMESTAMP`, tokenDigest)
	if err != nil {
		return nil, normalize(err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	object, err := s.storage.Get(ctx, storagecap.GetInput{Path: ticket.ObjectRef})
	if err != nil || object == nil || !object.Found || object.Body == nil || object.Object == nil || object.Object.Size != ticket.SizeBytes {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer object.Body.Close()
	content, err := io.ReadAll(io.LimitReader(object.Body, maximumExportBytes+1))
	if err != nil || int64(len(content)) != ticket.SizeBytes || len(content) > maximumExportBytes {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != ticket.ContentSHA256 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &ExportDownload{FileName: ticket.RequestNo + ".xlsx", MediaType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", Size: ticket.SizeBytes, Body: io.NopCloser(bytes.NewReader(content))}, nil
}

type exportRequestRow struct {
	RequestNo, ExportType, Status, FailureCode, FilterDigest, PermissionCode string
	Filter                                                                   json.RawMessage
	DataScope, RequestedBy, RowCount                                         int
	RequestedAt                                                              time.Time `orm:"requested_at"`
	GeneratedAt, ExpiresAt                                                   *time.Time
}

const exportRequestSelect = `SELECT request.request_no,request.export_type_code AS export_type,request.filter_json AS filter,request.filter_digest,request.permission_code,request.data_scope_code AS data_scope,request.requested_by,request.status_code AS status,request.failure_code,request.created_at AS requested_at,request.generated_at,artifact.expires_at,COALESCE(artifact.row_count,0) AS row_count FROM plugin_idc_core_financial_export_request request LEFT JOIN plugin_idc_core_financial_export_artifact artifact ON artifact.tenant_id=request.tenant_id AND artifact.request_no=request.request_no WHERE request.tenant_id=0`

func (s *serviceImpl) authorizeOwnedExport(ctx context.Context, requestNo string) (*exportRequestRow, billing.ManagementFinanceAuthorization, string, error) {
	if !validReference(requestNo, 40) {
		return nil, billing.ManagementFinanceAuthorization{}, "", bizerr.NewCode(CodeInvalid)
	}
	var row *exportRequestRow
	if err := s.database.GetScan(ctx, &row, exportRequestSelect+` AND request.request_no=?`, requestNo); err != nil || row == nil {
		return nil, billing.ManagementFinanceAuthorization{}, "", inaccessible(err)
	}
	authorization, permission, err := s.authorizeExport(ctx, row.ExportType)
	if err != nil || authorization.UserID != row.RequestedBy || authorization.DataScope != row.DataScope || permission != row.PermissionCode {
		return nil, billing.ManagementFinanceAuthorization{}, "", bizerr.NewCode(CodeNotAccessible)
	}
	return row, authorization, permission, nil
}

func (s *serviceImpl) authorizeExport(ctx context.Context, exportType string) (billing.ManagementFinanceAuthorization, string, error) {
	authorization, err := s.authorizer.AuthorizeFinancialExport(ctx)
	if err != nil {
		return billing.ManagementFinanceAuthorization{}, "", err
	}
	var permission string
	switch exportType {
	case "invoice":
		_, err = s.authorizer.AuthorizeInvoiceList(ctx)
		permission = "idc:admin:billing:invoice:list"
	case "transaction":
		_, err = s.authorizer.AuthorizeTransactionList(ctx)
		permission = "idc:admin:billing:transaction:list"
	case "adjustment":
		_, err = s.authorizer.AuthorizeAdjustmentList(ctx)
		permission = "idc:admin:billing:adjustment:list"
	case "receivable":
		_, err = s.authorizer.AuthorizeFinancialOverview(ctx)
		permission = "idc:admin:billing:overview"
	default:
		return billing.ManagementFinanceAuthorization{}, "", bizerr.NewCode(CodeInvalid)
	}
	if err != nil {
		return billing.ManagementFinanceAuthorization{}, "", err
	}
	return authorization, permission, nil
}

func exportStatusFromRow(row *exportRequestRow) *ExportStatus {
	if row == nil {
		return nil
	}
	return &ExportStatus{RequestNo: row.RequestNo, ExportType: row.ExportType, Status: row.Status, FailureCode: row.FailureCode, RequestedAt: row.RequestedAt.UTC(), GeneratedAt: utcPointer(row.GeneratedAt), ExpiresAt: utcPointer(row.ExpiresAt), RowCount: row.RowCount}
}

func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func idccapTransition(operation, idempotency, requestNo, permission, state, exportType string, dataScope int) idccap.BusinessAuditTransition {
	before := "not_created"
	if state == "ready" {
		before = "generating"
	}
	return idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"state": before}, IdempotencyScope: operation, IdempotencyKey: idempotency, OperationCode: operation, TargetType: "financial_export", TargetNo: requestNo, ResultCode: "succeeded", ReasonCode: "staff_financial_export", RiskCode: "medium", RequiredPermission: "idc:admin:billing:export", CorrelationNo: requestNo, AfterSnapshot: map[string]any{"state": state, "export_type": exportType, "source_permission": permission, "data_scope": dataScope, "format": "xlsx"}}
}

func (s *serviceImpl) buildExportWorkbook(ctx context.Context, exportType string, filter json.RawMessage) ([]byte, int, error) {
	var values map[string]any
	if err := json.Unmarshal(filter, &values); err != nil {
		return nil, 0, err
	}
	headers, query, args, err := exportQuery(exportType, values)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.database.GetAll(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	if len(rows) > maximumExportRows {
		return nil, 0, bizerr.NewCode(CodeInvalid)
	}
	file := excelize.NewFile()
	defer file.Close()
	const sheet = "Financial data"
	if err = file.SetSheetName("Sheet1", sheet); err != nil {
		return nil, 0, err
	}
	stream, err := file.NewStreamWriter(sheet)
	if err != nil {
		return nil, 0, err
	}
	headerValues := make([]any, len(headers))
	for index, value := range headers {
		headerValues[index] = value
	}
	if err = stream.SetRow("A1", headerValues); err != nil {
		return nil, 0, err
	}
	for index, row := range rows {
		values := make([]any, len(headers))
		for column, header := range headers {
			if value := row[header]; value != nil {
				values[column] = exportCell(value.Val())
			} else {
				values[column] = ""
			}
		}
		cell, _ := excelize.CoordinatesToCellName(1, index+2)
		if err = stream.SetRow(cell, values); err != nil {
			return nil, 0, err
		}
	}
	if err = stream.Flush(); err != nil {
		return nil, 0, err
	}
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, 0, err
	}
	return buffer.Bytes(), len(rows), nil
}

func exportQuery(exportType string, filter map[string]any) ([]string, string, []any, error) {
	customer, _ := filter["customerNo"].(string)
	currency, _ := filter["currencyCode"].(string)
	status, _ := filter["statusCode"].(string)
	typeCode, _ := filter["typeCode"].(string)
	if (customer != "" && !validReference(customer, 40)) || (currency != "" && (len(currency) != 3 || currency != strings.ToUpper(currency))) || (status != "" && !validReference(status, 32)) || (typeCode != "" && !validReference(typeCode, 64)) {
		return nil, "", nil, bizerr.NewCode(CodeInvalid)
	}
	where, args := []string{"tenant_id=0"}, []any{}
	if customer != "" {
		where = append(where, "customer_no=?")
		args = append(args, customer)
	}
	if currency != "" {
		where = append(where, "currency_code=?")
		args = append(args, currency)
	}
	switch exportType {
	case "invoice":
		if status != "" {
			where = append(where, "status_code=?")
			args = append(args, status)
		}
		headers := []string{"invoice_no", "customer_no", "order_no", "currency_code", "status_code", "total_amount_minor", "paid_amount_minor", "waived_amount_minor", "outstanding_amount_minor", "payment_due_at", "created_at"}
		query := `SELECT invoice_no,customer_no,COALESCE(order_no,'') AS order_no,currency_code,status_code,total_amount_minor,paid_amount_minor,waived_amount_minor,(total_amount_minor-paid_amount_minor-waived_amount_minor-credit_amount_minor) AS outstanding_amount_minor,payment_due_at,created_at FROM plugin_idc_core_billing_invoice WHERE ` + strings.Join(where, " AND ") + ` ORDER BY id LIMIT 100001`
		return headers, query, args, nil
	case "transaction":
		txWhere, txArgs := []string{"transaction.tenant_id=0"}, []any{}
		if currency != "" {
			txWhere = append(txWhere, "transaction.currency_code=?")
			txArgs = append(txArgs, currency)
		}
		if typeCode != "" {
			txWhere = append(txWhere, "transaction.transaction_type_code=?")
			txArgs = append(txArgs, typeCode)
		}
		if customer != "" {
			txWhere = append(txWhere, `EXISTS (SELECT 1 FROM plugin_idc_core_billing_ledger_entry scoped_entry JOIN plugin_idc_core_billing_ledger_account scoped_account ON scoped_account.tenant_id=scoped_entry.tenant_id AND scoped_account.account_no=scoped_entry.account_no WHERE scoped_entry.tenant_id=transaction.tenant_id AND scoped_entry.transaction_no=transaction.transaction_no AND scoped_account.customer_no=?)`)
			txArgs = append(txArgs, customer)
		}
		headers := []string{"transaction_no", "transaction_type_code", "customer_no", "currency_code", "amount_minor", "source_type_code", "source_reference", "related_invoice_no", "posted_at"}
		query := `SELECT transaction.transaction_no,transaction.transaction_type_code,COALESCE(MAX(account.customer_no),'') AS customer_no,transaction.currency_code,transaction.debit_total_minor AS amount_minor,transaction.source_type_code,transaction.source_reference,COALESCE(MAX(entry.invoice_no),'') AS related_invoice_no,transaction.posted_at FROM plugin_idc_core_billing_ledger_transaction transaction LEFT JOIN plugin_idc_core_billing_ledger_entry entry ON entry.tenant_id=transaction.tenant_id AND entry.transaction_no=transaction.transaction_no LEFT JOIN plugin_idc_core_billing_ledger_account account ON account.tenant_id=entry.tenant_id AND account.account_no=entry.account_no WHERE ` + strings.Join(txWhere, " AND ") + ` GROUP BY transaction.id ORDER BY transaction.id LIMIT 100001`
		return headers, query, txArgs, nil
	case "adjustment":
		if status != "" {
			where = append(where, "status_code=?")
			args = append(args, status)
		}
		if typeCode != "" {
			where = append(where, "adjustment_type_code=?")
			args = append(args, typeCode)
		}
		headers := []string{"adjustment_no", "adjustment_type_code", "customer_no", "invoice_no", "currency_code", "amount_minor", "status_code", "reason_code", "requested_by", "created_at"}
		query := `SELECT adjustment_no,adjustment_type_code,customer_no,COALESCE(invoice_no,'') AS invoice_no,currency_code,amount_minor,status_code,reason_code,requested_by,created_at FROM plugin_idc_core_billing_adjustment_request WHERE ` + strings.Join(where, " AND ") + ` ORDER BY id LIMIT 100001`
		return headers, query, args, nil
	case "receivable":
		headers := []string{"currency_code", "open_invoice_count", "outstanding_amount_minor"}
		query := `SELECT currency_code,COUNT(*) AS open_invoice_count,SUM(total_amount_minor-paid_amount_minor-waived_amount_minor-credit_amount_minor) AS outstanding_amount_minor FROM plugin_idc_core_billing_invoice WHERE ` + strings.Join(where, " AND ") + ` AND status_code IN ('pending_payment','partially_paid','overdue') GROUP BY currency_code ORDER BY currency_code LIMIT 65`
		return headers, query, args, nil
	default:
		return nil, "", nil, bizerr.NewCode(CodeInvalid)
	}
}

func exportCell(value any) any {
	switch item := value.(type) {
	case time.Time:
		return item.UTC().Format(time.RFC3339)
	case *time.Time:
		if item == nil {
			return ""
		}
		return item.UTC().Format(time.RFC3339)
	case []byte:
		return string(item)
	default:
		if value == nil {
			return ""
		}
		return value
	}
}
