// This file implements bounded, secret-safe contract management reads.

package electroniccontractpreparation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"lina-plugin-idc-compliance/backend/internal/service/numberedpage"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

const (
	defaultManagementContractPageSize = 20
	maximumManagementContractPageSize = 100
	maximumManagementContractFlows    = 50
	maximumManagementContractEvents   = 200
	maximumManagementContractFiles    = 50
	maximumManagementArchiveJobs      = 50
	maximumManagementReplacements     = 20
	managementContractDirectoryTarget = "electronic-contract-directory"
)

var managementContractStatuses = map[string]struct{}{
	"draft": {}, "pending_launch": {}, "signing": {}, "completed": {},
	"rejected": {}, "expired": {}, "revoked": {}, "failed": {},
	"reissuing": {}, "terminated": {},
}

type managementContractCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        int64     `json:"id"`
}

type managementContractRow struct {
	ID                     int64      `orm:"id"`
	ContractNo             string     `orm:"contract_no"`
	CustomerNo             string     `orm:"customer_no"`
	OrderNo                string     `orm:"order_no"`
	ContractTypeCode       string     `orm:"contract_type_code"`
	RequiredForFulfillment bool       `orm:"required_for_fulfillment"`
	TemplateNo             string     `orm:"template_no"`
	TemplateVersion        int64      `orm:"template_version"`
	TemplateName           string     `orm:"template_name"`
	LanguageCode           string     `orm:"language_code"`
	SubjectTypeCode        string     `orm:"subject_type_code"`
	SigningOrderCode       string     `orm:"signing_order_code"`
	AmountMinor            int64      `orm:"amount_minor"`
	CurrencyCode           string     `orm:"currency_code"`
	StatusCode             string     `orm:"status_code"`
	LaunchedAt             *time.Time `orm:"launched_at"`
	CompletedAt            *time.Time `orm:"completed_at"`
	TerminatedAt           *time.Time `orm:"terminated_at"`
	SigningExpiresAt       *time.Time `orm:"signing_expires_at"`
	CreatedAt              *time.Time `orm:"created_at"`
	UpdatedAt              *time.Time `orm:"updated_at"`
	Version                int64      `orm:"version"`
}

// ListManagementContracts authorizes and returns one bounded page with its authorized total.
func (s *serviceImpl) ListManagementContracts(
	ctx context.Context,
	query ManagementContractQuery,
) (*ManagementContractPage, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionContractQuery, TargetNo: managementContractDirectoryTarget,
	}); err != nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	query, cursor, err := normalizeManagementContractQuery(query)
	if err != nil {
		return nil, err
	}
	model := managementContractBaseModel(ctx)
	if query.StatusCode != "" {
		model = model.Where("c.status_code", query.StatusCode)
	}
	if query.OrderNo != "" {
		model = model.Where("c.order_no", query.OrderNo)
	}
	if query.CustomerNo != "" {
		model = model.Where("c.customer_no", query.CustomerNo)
	}
	countModel := model.Clone()
	if cursor != nil {
		model = model.Where(
			"(c.created_at < ?) OR (c.created_at = ? AND c.id < ?)",
			cursor.CreatedAt, cursor.CreatedAt, cursor.ID,
		)
	}
	var rows []*managementContractRow
	total, err := numberedpage.Scan(ctx, s.database, countModel, managementContractFields(model).OrderDesc("c.created_at").OrderDesc("c.id").Offset(max(query.Page-1, 0)*query.PageSize).Limit(query.PageSize+1), &rows)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	hasMore := len(rows) > query.PageSize
	if hasMore {
		rows = rows[:query.PageSize]
	}
	page := &ManagementContractPage{Total: total, Items: make([]ManagementContractListItem, 0, len(rows))}
	for _, row := range rows {
		item, ok := projectManagementContract(row)
		if !ok {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		page.Items = append(page.Items, item)
	}
	if hasMore && len(rows) > 0 {
		page.NextCursor, err = encodeManagementContractCursor(managementContractCursor{
			CreatedAt: rows[len(rows)-1].CreatedAt.UTC(), ID: rows[len(rows)-1].ID,
		})
		if err != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
	}
	return page, nil
}

// GetManagementContract returns bounded local lifecycle and archive evidence.
func (s *serviceImpl) GetManagementContract(
	ctx context.Context,
	contractNo string,
) (*ManagementContractDetail, error) {
	contractNo = strings.TrimSpace(contractNo)
	if s == nil || ctx == nil || ctx.Err() != nil || !validBusinessNo(contractNo, 64) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionContractQuery, TargetNo: contractNo,
	}); err != nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var row *managementContractRow
	if err := managementContractModel(ctx).Where("c.contract_no", contractNo).Limit(1).Scan(&row); err != nil || row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	item, ok := projectManagementContract(row)
	if !ok {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	detail := &ManagementContractDetail{Item: item}
	if err := loadManagementContractEvidence(ctx, contractNo, detail); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return detail, nil
}

func managementContractModel(ctx context.Context) *gdb.Model {
	return managementContractFields(managementContractBaseModel(ctx))
}

func managementContractBaseModel(ctx context.Context) *gdb.Model {
	return dao.Contract.Ctx(ctx).As("c").
		LeftJoin(dao.ContractTemplateVersion.Table()+" tv",
			"tv.tenant_id = c.tenant_id AND tv.template_no = c.template_no AND tv.version_no = c.template_version").
		Where("c.tenant_id", idccap.DefaultTenantID)
}

func managementContractFields(model *gdb.Model) *gdb.Model {
	return model.Fields(
		"c.id", "c.contract_no", "c.customer_no", "c.order_no", "c.contract_type_code",
		"c.required_for_fulfillment", "c.template_no", "c.template_version",
		"COALESCE(tv.name, '') AS template_name", "COALESCE(tv.language_code, '') AS language_code",
		"c.subject_type_code", "c.signing_order_code", "c.amount_minor", "c.currency_code",
		"c.status_code", "c.launched_at", "c.completed_at", "c.terminated_at",
		"c.signing_expires_at", "c.created_at", "c.updated_at", "c.version",
	)
}

func normalizeManagementContractQuery(
	query ManagementContractQuery,
) (ManagementContractQuery, *managementContractCursor, error) {
	query.StatusCode = strings.TrimSpace(query.StatusCode)
	query.OrderNo = strings.TrimSpace(query.OrderNo)
	query.CustomerNo = strings.TrimSpace(query.CustomerNo)
	if query.PageSize == 0 {
		query.PageSize = defaultManagementContractPageSize
	}
	if !numberedpage.Valid(query.Page, query.PageSize, query.Cursor) || query.PageSize < 1 || query.PageSize > maximumManagementContractPageSize ||
		(query.StatusCode != "" && !validManagementContractStatus(query.StatusCode)) ||
		(query.OrderNo != "" && !validBusinessNo(query.OrderNo, 64)) ||
		(query.CustomerNo != "" && !validBusinessNo(query.CustomerNo, 64)) {
		return ManagementContractQuery{}, nil, bizerr.NewCode(CodeInvalid)
	}
	cursor, err := decodeManagementContractCursor(query.Cursor)
	if err != nil {
		return ManagementContractQuery{}, nil, bizerr.NewCode(CodeInvalid)
	}
	return query, cursor, nil
}

func validManagementContractStatus(value string) bool {
	_, ok := managementContractStatuses[value]
	return ok
}

func projectManagementContract(row *managementContractRow) (ManagementContractListItem, bool) {
	if row == nil || row.ID < 1 || row.CreatedAt == nil || row.UpdatedAt == nil ||
		!validBusinessNo(row.ContractNo, 64) || !validBusinessNo(row.CustomerNo, 64) ||
		!validBusinessNo(row.OrderNo, 64) || !validManagementContractStatus(row.StatusCode) ||
		row.TemplateName == "" || (row.LanguageCode != "zh-CN" && row.LanguageCode != "en-US") ||
		row.AmountMinor < 0 || row.CurrencyCode == "" || row.Version < 1 {
		return ManagementContractListItem{}, false
	}
	return ManagementContractListItem{
		ContractNo: row.ContractNo, CustomerNo: row.CustomerNo, OrderNo: row.OrderNo,
		ContractTypeCode: row.ContractTypeCode, RequiredForFulfillment: row.RequiredForFulfillment,
		TemplateNo: row.TemplateNo, TemplateVersion: row.TemplateVersion, TemplateName: row.TemplateName,
		LanguageCode: row.LanguageCode, SubjectTypeCode: row.SubjectTypeCode,
		SigningOrderCode: row.SigningOrderCode, AmountMinor: row.AmountMinor,
		CurrencyCode: row.CurrencyCode, StatusCode: row.StatusCode,
		LaunchedAt: cloneContractTime(row.LaunchedAt), CompletedAt: cloneContractTime(row.CompletedAt),
		TerminatedAt: cloneContractTime(row.TerminatedAt), SigningExpiresAt: cloneContractTime(row.SigningExpiresAt),
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), Version: row.Version,
	}, true
}

func loadManagementContractEvidence(
	ctx context.Context,
	contractNo string,
	detail *ManagementContractDetail,
) error {
	var (
		flows        []*entity.ContractFlow
		events       []*entity.ContractStatusEvent
		files        []*entity.ContractFile
		jobs         []*entity.ContractArchiveJob
		replacements []*entity.ContractReplacement
	)
	if err := dao.ContractFlow.Ctx(ctx).Where(do.ContractFlow{TenantId: idccap.DefaultTenantID, ContractNo: contractNo}).
		OrderDesc("sequence_no").OrderDesc("id").Limit(maximumManagementContractFlows + 1).Scan(&flows); err != nil {
		return err
	}
	if err := dao.ContractStatusEvent.Ctx(ctx).Where(do.ContractStatusEvent{TenantId: idccap.DefaultTenantID, ContractNo: contractNo}).
		OrderDesc("supplier_observed_at").OrderDesc("id").Limit(maximumManagementContractEvents + 1).Scan(&events); err != nil {
		return err
	}
	if err := dao.ContractFile.Ctx(ctx).Where(do.ContractFile{TenantId: idccap.DefaultTenantID, ContractNo: contractNo}).
		OrderDesc("created_at").OrderDesc("id").Limit(maximumManagementContractFiles + 1).Scan(&files); err != nil {
		return err
	}
	if err := dao.ContractArchiveJob.Ctx(ctx).Where(do.ContractArchiveJob{TenantId: idccap.DefaultTenantID, ContractNo: contractNo}).
		OrderDesc("created_at").OrderDesc("id").Limit(maximumManagementArchiveJobs + 1).Scan(&jobs); err != nil {
		return err
	}
	if err := dao.ContractReplacement.Ctx(ctx).Where(do.ContractReplacement{TenantId: idccap.DefaultTenantID}).
		Where("old_contract_no = ? OR new_contract_no = ?", contractNo, contractNo).
		OrderDesc("id").Limit(maximumManagementReplacements + 1).Scan(&replacements); err != nil {
		return err
	}
	detail.HistoryTruncated = len(flows) > maximumManagementContractFlows || len(events) > maximumManagementContractEvents ||
		len(files) > maximumManagementContractFiles || len(jobs) > maximumManagementArchiveJobs ||
		len(replacements) > maximumManagementReplacements
	flows = bounded(flows, maximumManagementContractFlows)
	events = bounded(events, maximumManagementContractEvents)
	files = bounded(files, maximumManagementContractFiles)
	jobs = bounded(jobs, maximumManagementArchiveJobs)
	replacements = bounded(replacements, maximumManagementReplacements)
	detail.Flows = make([]ManagementContractFlow, 0, len(flows))
	for _, row := range flows {
		if row == nil || row.CreatedAt == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		detail.Flows = append(detail.Flows, ManagementContractFlow{
			FlowNo: row.FlowNo, SequenceNo: row.SequenceNo, PurposeCode: row.PurposeCode,
			StatusCode: row.StatusCode, ProviderStatus: stringValue(row.ProviderStatus), FailureCode: row.FailureCode,
			ExpiresAt: cloneContractTime(row.ExpiresAt), SubmittedAt: cloneContractTime(row.SubmittedAt),
			CompletedAt: cloneContractTime(row.CompletedAt), CreatedAt: row.CreatedAt.UTC(),
		})
	}
	detail.Events = make([]ManagementContractEvent, 0, len(events))
	for _, row := range events {
		if row == nil || row.SupplierObservedAt == nil || row.ObservedAt == nil || row.CreatedAt == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		detail.Events = append(detail.Events, ManagementContractEvent{
			FlowNo: row.FlowNo, SourceKind: row.SourceKind, ProviderStatus: row.ProviderStatus,
			ProjectedStatus: row.ProjectedStatus, DecisionCode: row.DecisionCode, ReasonCode: row.ReasonCode,
			SupplierObservedAt: row.SupplierObservedAt.UTC(), ObservedAt: row.ObservedAt.UTC(), CreatedAt: row.CreatedAt.UTC(),
		})
	}
	detail.Files = make([]ManagementContractFile, 0, len(files))
	for _, row := range files {
		if row == nil || row.CreatedAt == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		detail.Files = append(detail.Files, ManagementContractFile{
			FileNo: row.FileNo, FlowNo: stringValue(row.FlowNo), FileTypeCode: row.FileTypeCode,
			ContentSHA256: row.ContentSha256, MediaType: row.MediaType, ByteSize: row.ByteSize,
			StatusCode: row.StatusCode, Immutable: row.Immutable, ArchivedAt: cloneContractTime(row.ArchivedAt),
			RetentionUntil: cloneContractTime(row.RetentionUntil), LegalHold: row.LegalHold,
			SupplierObservedAt: cloneContractTime(row.SupplierObservedAt), CreatedAt: row.CreatedAt.UTC(),
		})
	}
	detail.ArchiveJobs = make([]ManagementContractArchiveJob, 0, len(jobs))
	for _, row := range jobs {
		if row == nil || row.CreatedAt == nil || row.UpdatedAt == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		detail.ArchiveJobs = append(detail.ArchiveJobs, ManagementContractArchiveJob{
			JobNo: row.JobNo, FlowNo: row.FlowNo, ArtifactTypeCode: row.ArtifactTypeCode,
			StatusCode: row.StatusCode, AttemptCount: row.AttemptCount, LastReasonCode: row.LastReasonCode,
			SupplierObservedAt: cloneContractTime(row.SupplierObservedAt), CompletedAt: cloneContractTime(row.CompletedAt),
			NextAttemptAt: cloneContractTime(row.NextAttemptAt), CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
		})
	}
	detail.Replacements = make([]ManagementContractReplacement, 0, len(replacements))
	for _, row := range replacements {
		if row == nil || row.CreatedAt == nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		detail.Replacements = append(detail.Replacements, ManagementContractReplacement{
			ReplacementNo: row.ReplacementNo, OldContractNo: row.OldContractNo,
			NewContractNo: row.NewContractNo, CreatedAt: row.CreatedAt.UTC(),
		})
	}
	return nil
}

func cloneContractTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func bounded[T any](values []T, maximum int) []T {
	if len(values) > maximum {
		return values[:maximum]
	}
	return values
}

func encodeManagementContractCursor(cursor managementContractCursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeManagementContractCursor(value string) (*managementContractCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if len(value) > 256 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	var cursor managementContractCursor
	if json.Unmarshal(payload, &cursor) != nil || cursor.ID < 1 || cursor.CreatedAt.IsZero() {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	cursor.CreatedAt = cursor.CreatedAt.UTC()
	return &cursor, nil
}
