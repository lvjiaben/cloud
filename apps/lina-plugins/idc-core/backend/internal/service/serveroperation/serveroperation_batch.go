// Package serveroperation coordinates bounded and auditable server operations.
package serveroperation

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

const (
	PermissionServerBatchView      = "idc:admin:resource:server-batch:view"
	PermissionServerBatchSafe      = "idc:admin:resource:server-batch:safe"
	PermissionServerBatchDangerous = "idc:admin:resource:server-batch:dangerous"
	maximumServerBatchItems        = 20
)

type BatchAction string

const (
	BatchActionTagAdd            BatchAction = "tag_add"
	BatchActionTagRemove         BatchAction = "tag_remove"
	BatchActionClassificationSet BatchAction = "classification_set"
	BatchActionNoteAppend        BatchAction = "note_append"
	BatchActionRenewalReminder   BatchAction = "renewal_reminder"
	BatchActionSuspend           BatchAction = "suspend"
	BatchActionReboot            BatchAction = "reboot"
	BatchActionReinstall         BatchAction = "reinstall"
	BatchActionIPChange          BatchAction = "ip_change"
	BatchActionTerminate         BatchAction = "terminate"
)

var (
	CodeServerBatchInvalid = bizerr.MustDefine("IDC_SERVER_BATCH_INVALID",
		"The server batch request is invalid", gcode.CodeInvalidParameter)
	CodeServerBatchNotAccessible = bizerr.MustDefine("IDC_SERVER_BATCH_NOT_ACCESSIBLE",
		"The server batch is not accessible", gcode.CodeNotAuthorized)
	CodeServerBatchConflict = bizerr.MustDefine("IDC_SERVER_BATCH_CONFLICT",
		"The server batch preview changed; reload it and try again", gcode.CodeInvalidOperation)
	CodeServerBatchUnavailable = bizerr.MustDefine("IDC_SERVER_BATCH_UNAVAILABLE",
		"Server batch processing is temporarily unavailable", gcode.CodeInternalError)
)

type BatchPreviewItemInput struct {
	ServiceNo  string
	Parameters json.RawMessage
}

type BatchPreviewInput struct {
	Action         BatchAction
	Items          []BatchPreviewItemInput
	IdempotencyKey string
}

type BatchImpactItem struct {
	LineNumber                                       int
	ServiceVersion                                   int64
	ServiceNo, CustomerNo, CurrencyCode              string
	EntitlementStatus, RuntimeStatus, ProviderStatus string
	EntitlementExpiresAt                             *time.Time
	UnpaidAmountMinor                                int64
	ActionAmountMinor                                int64
	ResourceSummary                                  string
	Irreversible                                     bool
	Parameters                                       json.RawMessage
	PreviewStatus, FailureCode, ExecutionStatus      string
	OperationNo, NotificationEventNo                 string
	ExecutedAt                                       *time.Time
}

type ServiceBatch struct {
	BatchNo, Status, PreviewDigest string
	Action                         BatchAction
	Dangerous                      bool
	Items                          []BatchImpactItem
	CreatedAt                      time.Time
}

type BatchExecutionInput struct {
	BatchNo, PreviewDigest, ProofToken string
}

type BatchManagement interface {
	// ListServices returns only the bounded server directory available to an actor
	// holding the exact view permission. Callers must not treat an empty page as
	// proof that a server exists outside the actor's current data scope.
	ListServices(context.Context, BatchServiceListInput) (*BatchServicePage, error)
	Preview(context.Context, BatchPreviewInput) (*ServiceBatch, error)
	IssueDangerousConfirmation(context.Context, string, string) (*reauth.ProofContext, error)
	Execute(context.Context, BatchExecutionInput) (*ServiceBatch, error)
	Get(context.Context, string) (*ServiceBatch, error)
	ListClassifications(context.Context) ([]ServerClassification, error)
	SaveClassification(context.Context, ServerClassificationInput) (*ServerClassification, error)
	DisableClassification(context.Context, string, int64) (*ServerClassification, error)
}

type BatchServiceListInput struct {
	Keyword, EntitlementStatus, RuntimeStatus, Cursor string
	Limit                                             int
	Page                                              int
}

type BatchServiceSummary struct {
	ServiceNo, CustomerNo, ProductNameZhCN, ProductNameEnUS string
	EntitlementStatus, RuntimeStatus, ProviderStatus        string
	EntitlementExpiresAt                                    *time.Time
}

type BatchServicePage struct {
	Items      []BatchServiceSummary
	Total      int
	NextCursor string
}

type ServerClassification struct {
	ClassificationNo, NameZhCN, NameEnUS, Status string
	Version                                      int64
}

type ServerClassificationInput struct {
	ClassificationNo, NameZhCN, NameEnUS, RequestNo string
	ExpectedVersion                                 int64
}

type batchManagementImpl struct {
	management    *managementServiceImpl
	pricing       *accessoryPricingImpl
	recentAuth    reauth.Service
	audit         idccap.BusinessAuditWriter
	notifications notificationcap.EventSink
}

func NewBatchManagement(management ManagementService, pricing AccessoryPricing,
	recentAuth reauth.Service, audit idccap.BusinessAuditWriter,
	notifications notificationcap.EventSink) (BatchManagement, error) {
	manager, managerOK := management.(*managementServiceImpl)
	prices, pricesOK := pricing.(*accessoryPricingImpl)
	if !managerOK || manager == nil || !pricesOK || prices == nil || recentAuth == nil ||
		audit == nil || notifications == nil {
		return nil, ErrServiceUnavailable
	}
	return &batchManagementImpl{management: manager, pricing: prices, recentAuth: recentAuth,
		audit: audit, notifications: notifications}, nil
}

// ListServices authorizes the actor and returns one keyset page without per-row queries.
func (s *batchManagementImpl) ListServices(ctx context.Context, input BatchServiceListInput) (*BatchServicePage, error) {
	actor, grants, err := s.management.authorize(ctx, []string{PermissionServerBatchView})
	if err != nil || actor <= 0 || !grants[PermissionServerBatchView] {
		return nil, bizerr.NewCode(CodeServerBatchNotAccessible)
	}
	input.Keyword = strings.TrimSpace(input.Keyword)
	input.EntitlementStatus = strings.TrimSpace(input.EntitlementStatus)
	input.RuntimeStatus = strings.TrimSpace(input.RuntimeStatus)
	input.Cursor = strings.TrimSpace(input.Cursor)
	if len(input.Keyword) > 80 || len(input.EntitlementStatus) > 32 || len(input.RuntimeStatus) > 32 ||
		(input.Cursor != "" && !validReference(input.Cursor, 40)) {
		return nil, bizerr.NewCode(CodeServerBatchInvalid)
	}
	if input.Limit <= 0 {
		input.Limit = 50
	}
	if input.Limit > 100 || input.Page < 0 || input.Page > 1_000_000 || (input.Page > 0 && input.Cursor != "") {
		return nil, bizerr.NewCode(CodeServerBatchInvalid)
	}
	fields := `SELECT service.service_no,service.customer_no,
product.name_zh_cn AS product_name_zh_cn,product.name_en_us AS product_name_en_us,
service.entitlement_status_code AS entitlement_status,service.runtime_status_code AS runtime_status,
service.provider_task_status_code AS provider_status,service.entitlement_expires_at `
	query := `FROM plugin_idc_core_service_instance service
JOIN plugin_idc_core_catalog_product product
 ON product.tenant_id=0 AND product.product_no=service.product_no
WHERE service.tenant_id=0`
	args := make([]any, 0, 5)
	if input.Keyword != "" {
		query += ` AND (service.service_no ILIKE ? OR service.customer_no ILIKE ?)`
		pattern := "%" + input.Keyword + "%"
		args = append(args, pattern, pattern)
	}
	if input.EntitlementStatus != "" {
		query += ` AND service.entitlement_status_code=?`
		args = append(args, input.EntitlementStatus)
	}
	if input.RuntimeStatus != "" {
		query += ` AND service.runtime_status_code=?`
		args = append(args, input.RuntimeStatus)
	}
	total, err := s.management.directory.database.GetValue(ctx, "SELECT count(*) "+query, args...)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeServerBatchUnavailable)
	}
	if input.Cursor != "" {
		query += ` AND service.service_no<?`
		args = append(args, input.Cursor)
	}
	query += ` ORDER BY service.service_no DESC LIMIT ?`
	args = append(args, input.Limit+1)
	if input.Page > 0 {
		query += ` OFFSET ?`
		args = append(args, (input.Page-1)*input.Limit)
	}
	var rows []BatchServiceSummary
	if err = s.management.directory.database.GetScan(ctx, &rows, fields+query, args...); err != nil {
		return nil, bizerr.WrapCode(err, CodeServerBatchUnavailable)
	}
	page := &BatchServicePage{Items: rows, Total: total.Int()}
	if len(page.Items) > input.Limit {
		page.Items = page.Items[:input.Limit]
		page.NextCursor = page.Items[len(page.Items)-1].ServiceNo
	}
	return page, nil
}

type batchServiceRow struct {
	ServiceNo, CustomerNo, CurrencyCode, EntitlementStatus, RuntimeStatus string
	ProviderStatus, ProductType, PoolNo, ChannelNo                        string
	Version                                                               int64
	EntitlementExpiresAt                                                  *time.Time
	UnpaidAmountMinor                                                     int64
}

func (s *batchManagementImpl) Preview(ctx context.Context, input BatchPreviewInput) (*ServiceBatch, error) {
	permission := batchPermission(input.Action)
	actor, grants, err := s.management.authorize(ctx, []string{PermissionServerBatchView, permission})
	if err != nil || actor <= 0 || !grants[PermissionServerBatchView] || !grants[permission] {
		return nil, bizerr.NewCode(CodeServerBatchNotAccessible)
	}
	items, requestDigest, err := normalizeBatchPreview(input)
	if err != nil {
		return nil, err
	}
	idempotencyDigest, _ := digestValue(struct {
		Actor int
		Key   string
	}{actor, input.IdempotencyKey})
	var replay *struct{ BatchNo, RequestDigest string }
	if err = s.management.directory.database.GetScan(ctx, &replay, `SELECT batch_no,request_digest
FROM plugin_idc_core_server_service_batch
WHERE tenant_id=0 AND requested_by=? AND idempotency_digest=?`, actor, idempotencyDigest); err != nil {
		return nil, bizerr.WrapCode(err, CodeServerBatchUnavailable)
	}
	if replay != nil {
		if replay.RequestDigest != requestDigest {
			return nil, bizerr.NewCode(CodeServerBatchConflict)
		}
		return s.loadBatch(ctx, replay.BatchNo, actor)
	}
	impacts, err := s.buildBatchImpacts(ctx, input, items)
	if err != nil {
		return nil, err
	}
	previewDigest, err := digestValue(struct {
		Action BatchAction
		Items  []BatchImpactItem
	}{input.Action, impacts})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeServerBatchUnavailable)
	}
	batchNo, _ := deterministicReference("sbt_", struct {
		Actor int
		Key   string
	}{actor, input.IdempotencyKey})
	now, err := databaseNow(ctx, s.management.directory.database)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeServerBatchUnavailable)
	}
	err = s.management.directory.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, insertErr := s.management.directory.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_service_batch
(tenant_id,batch_no,action_code,dangerous,status_code,item_count,preview_digest,idempotency_digest,
 request_digest,requested_by,created_at,updated_at)
VALUES (0,?,?,?,'previewed',?,?,?,?,?,?,?)`, batchNo, input.Action, dangerousBatchAction(input.Action),
			len(impacts), previewDigest, idempotencyDigest, requestDigest, actor, now, now)
		if insertErr != nil {
			return insertErr
		}
		for _, impact := range impacts {
			_, insertErr = s.management.directory.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_service_batch_item
(tenant_id,batch_no,line_number,service_no,customer_no,service_version,currency_code,
 entitlement_status_code,runtime_status_code,provider_task_status_code,entitlement_expires_at,
 unpaid_amount_minor,action_amount_minor,resource_summary,irreversible,parameters,preview_status_code,failure_code,
 execution_status_code,created_at,updated_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?::JSONB,?,?,'pending',?,?)`, batchNo, impact.LineNumber,
				impact.ServiceNo, impact.CustomerNo, impact.ServiceVersion, impact.CurrencyCode,
				impact.EntitlementStatus, impact.RuntimeStatus, impact.ProviderStatus,
				impact.EntitlementExpiresAt, impact.UnpaidAmountMinor, impact.ActionAmountMinor, impact.ResourceSummary,
				impact.Irreversible, string(impact.Parameters), impact.PreviewStatus,
				impact.FailureCode, now, now)
			if insertErr != nil {
				return insertErr
			}
		}
		_, auditErr := s.audit.AppendBusinessBatchTransition(ctx, idccap.BusinessAuditBatchTransition{
			IdempotencyScope: "server.batch.preview", IdempotencyKey: input.IdempotencyKey,
			OperationCode: "server.batch.preview", TargetType: "server_service_batch", TargetNo: batchNo,
			AffectedCustomerNos: batchCustomerNos(impacts), ResultCode: "succeeded",
			ReasonCode: "staff_batch_preview", RiskCode: batchRisk(input.Action),
			RequiredPermission: permission, AfterSnapshot: map[string]any{
				"action": input.Action, "item_count": len(impacts), "preview_digest": previewDigest,
			},
		})
		return auditErr
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeServerBatchUnavailable)
	}
	return &ServiceBatch{BatchNo: batchNo, Status: "previewed", PreviewDigest: previewDigest,
		Action: input.Action, Dangerous: dangerousBatchAction(input.Action), Items: impacts, CreatedAt: now.UTC()}, nil
}

func (s *batchManagementImpl) buildBatchImpacts(ctx context.Context, input BatchPreviewInput,
	items []BatchPreviewItemInput) ([]BatchImpactItem, error) {
	serviceNos := make([]string, 0, len(items))
	for _, item := range items {
		serviceNos = append(serviceNos, item.ServiceNo)
	}
	rows, err := s.loadBatchServices(ctx, serviceNos)
	if err != nil || len(rows) != len(items) {
		return nil, bizerr.NewCode(CodeServerBatchNotAccessible)
	}
	byNo := make(map[string]batchServiceRow, len(rows))
	for _, row := range rows {
		byNo[row.ServiceNo] = row
	}
	classifications, err := s.loadBatchClassifications(ctx, input.Action, items)
	if err != nil {
		return nil, err
	}
	impacts := make([]BatchImpactItem, 0, len(items))
	for index, requested := range items {
		row, exists := byNo[requested.ServiceNo]
		if !exists {
			return nil, bizerr.NewCode(CodeServerBatchNotAccessible)
		}
		impact := projectBatchImpact(index+1, input.Action, requested.Parameters, row)
		if err = s.validateBatchImpact(ctx, input, requested, row, &impact, classifications); err != nil {
			return nil, err
		}
		impacts = append(impacts, impact)
	}
	return impacts, nil
}

func (s *batchManagementImpl) validateBatchImpact(ctx context.Context, input BatchPreviewInput,
	requested BatchPreviewItemInput, row batchServiceRow, impact *BatchImpactItem,
	classifications map[string]struct{}) error {
	if input.Action == BatchActionClassificationSet && impact.PreviewStatus == "eligible" {
		classificationNo, valid := batchClassificationReference(requested.Parameters)
		if _, exists := classifications[classificationNo]; !valid || !exists {
			impact.PreviewStatus, impact.FailureCode = "ineligible", "classification_not_available"
		}
	}
	if input.Action == BatchActionReinstall && impact.PreviewStatus == "eligible" {
		imageReference, valid := batchReinstallReference(requested.Parameters)
		projection, imageErr := s.management.directory.images.GetReinstallImages(ctx,
			serveroperationresourcecap.CapabilityRoute{ServiceNo: row.ServiceNo,
				ChannelNo: row.ChannelNo, PoolNo: row.PoolNo})
		selectable := false
		if imageErr == nil && projection != nil && projection.ServiceNo == row.ServiceNo {
			for _, image := range projection.Images {
				if image.Reference == imageReference && image.Selectable {
					selectable = true
					break
				}
			}
		}
		if !valid || !selectable {
			impact.PreviewStatus, impact.FailureCode = "ineligible", "reinstall_image_not_available"
		}
	}
	if input.Action != BatchActionIPChange || impact.PreviewStatus != "eligible" {
		return nil
	}
	quote, quoteErr := s.previewIPChangeForStaff(ctx, row.CustomerNo, IPChangeQuoteInput{
		ServiceNo: row.ServiceNo, IdempotencyKey: input.IdempotencyKey + ":" + row.ServiceNo,
	})
	if quoteErr != nil {
		impact.PreviewStatus, impact.FailureCode = "ineligible", batchExecutionError(quoteErr)
		return nil
	}
	impact.ActionAmountMinor = quote.AmountMinor
	parameters, marshalErr := json.Marshal(map[string]string{"quoteNo": quote.QuoteNo})
	if marshalErr != nil {
		return bizerr.WrapCode(marshalErr, CodeServerBatchUnavailable)
	}
	impact.Parameters = parameters
	return nil
}

func (s *batchManagementImpl) IssueDangerousConfirmation(ctx context.Context,
	batchNo, previewDigest string) (*reauth.ProofContext, error) {
	actor, grants, err := s.management.authorize(ctx, []string{PermissionServerBatchDangerous})
	if err != nil || actor <= 0 || !grants[PermissionServerBatchDangerous] ||
		!validReference(batchNo, 40) || len(previewDigest) != 64 {
		return nil, bizerr.NewCode(CodeServerBatchNotAccessible)
	}
	batch, err := s.loadBatch(ctx, batchNo, actor)
	if err != nil || !batch.Dangerous || batch.Status != "previewed" || batch.PreviewDigest != previewDigest {
		return nil, bizerr.NewCode(CodeServerBatchConflict)
	}
	return s.recentAuth.Issue(ctx, batchReauthBinding(batch))
}

func (s *batchManagementImpl) Get(ctx context.Context, batchNo string) (*ServiceBatch, error) {
	actor, grants, err := s.management.authorize(ctx, []string{PermissionServerBatchView})
	if err != nil || actor <= 0 || !grants[PermissionServerBatchView] || !validReference(batchNo, 40) {
		return nil, bizerr.NewCode(CodeServerBatchNotAccessible)
	}
	return s.loadBatch(ctx, batchNo, actor)
}

func normalizeBatchPreview(input BatchPreviewInput) ([]BatchPreviewItemInput, string, error) {
	if batchPermission(input.Action) == "" || len(input.Items) < 1 || len(input.Items) > maximumServerBatchItems ||
		len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 || strings.TrimSpace(input.IdempotencyKey) != input.IdempotencyKey {
		return nil, "", bizerr.NewCode(CodeServerBatchInvalid)
	}
	items := append([]BatchPreviewItemInput(nil), input.Items...)
	seen := make(map[string]struct{}, len(items))
	for index := range items {
		items[index].ServiceNo = strings.TrimSpace(items[index].ServiceNo)
		if !validReference(items[index].ServiceNo, 40) {
			return nil, "", bizerr.NewCode(CodeServerBatchInvalid)
		}
		if _, exists := seen[items[index].ServiceNo]; exists {
			return nil, "", bizerr.NewCode(CodeServerBatchInvalid)
		}
		seen[items[index].ServiceNo] = struct{}{}
		var object map[string]any
		if len(items[index].Parameters) == 0 {
			items[index].Parameters = json.RawMessage(`{}`)
		}
		if json.Unmarshal(items[index].Parameters, &object) != nil || object == nil {
			return nil, "", bizerr.NewCode(CodeServerBatchInvalid)
		}
		encoded, marshalErr := json.Marshal(object)
		if marshalErr != nil || len(encoded) > 4096 {
			return nil, "", bizerr.NewCode(CodeServerBatchInvalid)
		}
		items[index].Parameters = encoded
		if !validBatchParameters(input.Action, encoded) {
			return nil, "", bizerr.NewCode(CodeServerBatchInvalid)
		}
	}
	digest, err := digestValue(struct {
		Action BatchAction
		Items  []BatchPreviewItemInput
	}{input.Action, items})
	return items, digest, err
}

func validBatchParameters(action BatchAction, raw json.RawMessage) bool {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	switch action {
	case BatchActionTagAdd, BatchActionTagRemove:
		tag, ok := value["tag"].(string)
		return ok && len(value) == 1 && tag == strings.ToLower(strings.TrimSpace(tag)) && validTag(tag)
	case BatchActionClassificationSet:
		_, ok := batchClassificationReference(raw)
		return ok
	case BatchActionNoteAppend:
		note, ok := value["note"].(string)
		return ok && len(value) == 1 && note == strings.TrimSpace(note) && len([]rune(note)) >= 1 && len([]rune(note)) <= 1000
	case BatchActionRenewalReminder, BatchActionReboot, BatchActionSuspend, BatchActionTerminate:
		return len(value) == 0
	case BatchActionReinstall:
		_, ok := batchReinstallReference(raw)
		return ok
	case BatchActionIPChange:
		return len(value) == 0
	default:
		return false
	}
}

func batchClassificationReference(raw json.RawMessage) (string, bool) {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil || len(value) != 1 {
		return "", false
	}
	reference, ok := value["classificationNo"].(string)
	reference = strings.TrimSpace(reference)
	return reference, ok && validReference(reference, 40)
}

func batchReinstallReference(raw json.RawMessage) (string, bool) {
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil || len(value) != 1 {
		return "", false
	}
	var reference int64
	if json.Unmarshal(value["os"], &reference) != nil || reference <= 0 {
		return "", false
	}
	encoded, err := json.Marshal(reference)
	return string(encoded), err == nil
}

func validTag(value string) bool {
	if len(value) < 1 || len(value) > 40 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && !strings.ContainsRune("._-", r) {
			return false
		}
	}
	return value[0] != '.' && value[0] != '_' && value[0] != '-'
}

func batchPermission(action BatchAction) string {
	if dangerousBatchAction(action) {
		return PermissionServerBatchDangerous
	}
	switch action {
	case BatchActionTagAdd, BatchActionTagRemove, BatchActionClassificationSet, BatchActionNoteAppend, BatchActionRenewalReminder:
		return PermissionServerBatchSafe
	default:
		return ""
	}
}

func dangerousBatchAction(action BatchAction) bool {
	switch action {
	case BatchActionSuspend, BatchActionReboot, BatchActionReinstall, BatchActionIPChange, BatchActionTerminate:
		return true
	default:
		return false
	}
}

func projectBatchImpact(line int, action BatchAction, parameters json.RawMessage, row batchServiceRow) BatchImpactItem {
	impact := BatchImpactItem{LineNumber: line, ServiceNo: row.ServiceNo, CustomerNo: row.CustomerNo,
		CurrencyCode: row.CurrencyCode, ServiceVersion: row.Version, EntitlementStatus: row.EntitlementStatus,
		RuntimeStatus: row.RuntimeStatus, ProviderStatus: row.ProviderStatus,
		EntitlementExpiresAt: row.EntitlementExpiresAt, UnpaidAmountMinor: row.UnpaidAmountMinor,
		ResourceSummary: row.ProductType + " / " + row.PoolNo, Parameters: parameters,
		PreviewStatus: "eligible", ExecutionStatus: "pending", Irreversible: action == BatchActionTerminate || action == BatchActionReinstall}
	if row.ChannelNo == "" || row.PoolNo == "" || row.ProviderStatus == "result_unknown" || row.ProviderStatus == "manual_review" ||
		row.RuntimeStatus == "unknown" || row.RuntimeStatus == "terminating" || row.RuntimeStatus == "terminated" {
		impact.PreviewStatus, impact.FailureCode = "ineligible", "service_state_not_safe"
	}
	if impact.PreviewStatus == "eligible" {
		service := &serviceRow{ServiceNo: row.ServiceNo, CustomerNo: row.CustomerNo,
			CurrencyCode: row.CurrencyCode, PoolNo: row.PoolNo, ChannelNo: row.ChannelNo,
			EntitlementStatusCode: row.EntitlementStatus, RuntimeStatusCode: row.RuntimeStatus,
			ProviderTaskStatus: row.ProviderStatus, Version: row.Version}
		switch action {
		case BatchActionReboot:
			if !allowedAction(service, serveroperationresourcecap.ActionReboot) {
				impact.PreviewStatus, impact.FailureCode = "ineligible", "action_not_available"
			}
		case BatchActionReinstall:
			if !allowedAction(service, serveroperationresourcecap.ActionReinstall) {
				impact.PreviewStatus, impact.FailureCode = "ineligible", "action_not_available"
			}
		case BatchActionSuspend:
			if row.EntitlementStatus != "active" && row.EntitlementStatus != "grace" ||
				(row.RuntimeStatus != "running" && row.RuntimeStatus != "stopped") {
				impact.PreviewStatus, impact.FailureCode = "ineligible", "action_not_available"
			}
		case BatchActionTerminate:
			if row.EntitlementStatus == "terminated" || row.EntitlementStatus == "cancelled" ||
				(row.RuntimeStatus != "running" && row.RuntimeStatus != "stopped" && row.RuntimeStatus != "suspended") {
				impact.PreviewStatus, impact.FailureCode = "ineligible", "action_not_available"
			}
		}
	}
	return impact
}

func (s *batchManagementImpl) loadBatchServices(ctx context.Context, serviceNos []string) ([]batchServiceRow, error) {
	var rows []batchServiceRow
	err := s.management.directory.database.GetScan(ctx, &rows, `SELECT service.service_no,service.customer_no,
service.currency_code,service.entitlement_status_code AS entitlement_status,
service.runtime_status_code AS runtime_status,service.provider_task_status_code AS provider_status,
service.product_type_code AS product_type,service.pool_no,COALESCE(service.channel_no,'') AS channel_no,
service.version,service.entitlement_expires_at,
COALESCE(invoice.unpaid_amount_minor,0) AS unpaid_amount_minor
FROM plugin_idc_core_service_instance service
LEFT JOIN (
 SELECT operation.service_no,SUM(invoice.total_amount_minor-invoice.paid_amount_minor) AS unpaid_amount_minor
 FROM (
  SELECT DISTINCT service_no,invoice_no
  FROM plugin_idc_core_business_operation
  WHERE tenant_id=0 AND service_no IN (?) AND invoice_no IS NOT NULL
 ) operation
 JOIN plugin_idc_core_billing_invoice invoice
  ON invoice.tenant_id=0 AND invoice.invoice_no=operation.invoice_no
 WHERE invoice.status_code IN ('pending_payment','partially_paid','overdue','bad_debt')
 GROUP BY operation.service_no
) invoice ON invoice.service_no=service.service_no
WHERE service.tenant_id=0 AND service.service_no IN (?)
ORDER BY service.service_no`, serviceNos, serviceNos)
	return rows, err
}

func (s *batchManagementImpl) loadBatchClassifications(ctx context.Context, action BatchAction,
	items []BatchPreviewItemInput) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if action != BatchActionClassificationSet {
		return result, nil
	}
	references := make([]string, 0, len(items))
	for _, item := range items {
		reference, ok := batchClassificationReference(item.Parameters)
		if !ok {
			return nil, bizerr.NewCode(CodeServerBatchInvalid)
		}
		references = append(references, reference)
	}
	var rows []struct{ ClassificationNo string }
	if err := s.management.directory.database.GetScan(ctx, &rows, `SELECT classification_no
FROM plugin_idc_core_server_classification
WHERE tenant_id=0 AND status_code='active' AND classification_no IN (?)`, references); err != nil {
		return nil, wrapBatchUnavailable(err)
	}
	for _, row := range rows {
		result[row.ClassificationNo] = struct{}{}
	}
	return result, nil
}

func batchReauthBinding(batch *ServiceBatch) reauth.Binding {
	digest, _ := digestValue(struct{ Contract, BatchNo, PreviewDigest string }{
		"server-dangerous-batch/v1", batch.BatchNo, batch.PreviewDigest})
	return reauth.Binding{Purpose: reauth.PurposeBulkResource, TargetDigest: digest}
}

func batchRisk(action BatchAction) string {
	if dangerousBatchAction(action) {
		return "high"
	}
	return "low"
}

func batchCustomerNos(items []BatchImpactItem) []string {
	values := make(map[string]struct{}, len(items))
	for _, item := range items {
		values[item.CustomerNo] = struct{}{}
	}
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (s *batchManagementImpl) loadBatch(ctx context.Context, batchNo string, actor int) (*ServiceBatch, error) {
	var batch *struct {
		BatchNo, Status, PreviewDigest string
		Action                         BatchAction
		Dangerous                      bool
		CreatedAt                      time.Time
	}
	err := s.management.directory.database.GetScan(ctx, &batch, `SELECT batch_no,status_code AS status,
preview_digest,action_code AS action,dangerous,created_at
FROM plugin_idc_core_server_service_batch
WHERE tenant_id=0 AND batch_no=? AND requested_by=?`, batchNo, actor)
	if err != nil || batch == nil {
		return nil, bizerr.NewCode(CodeServerBatchNotAccessible)
	}
	var items []BatchImpactItem
	err = s.management.directory.database.GetScan(ctx, &items, `SELECT line_number,service_no,customer_no,
service_version,currency_code,entitlement_status_code AS entitlement_status,
runtime_status_code AS runtime_status,provider_task_status_code AS provider_status,
entitlement_expires_at,unpaid_amount_minor,action_amount_minor,resource_summary,irreversible,parameters,
preview_status_code AS preview_status,failure_code,execution_status_code AS execution_status,
COALESCE(operation_no,'') AS operation_no,COALESCE(notification_event_no,'') AS notification_event_no,
executed_at
FROM plugin_idc_core_server_service_batch_item
WHERE tenant_id=0 AND batch_no=? ORDER BY line_number`, batchNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeServerBatchUnavailable)
	}
	return &ServiceBatch{BatchNo: batch.BatchNo, Status: batch.Status, PreviewDigest: batch.PreviewDigest,
		Action: batch.Action, Dangerous: batch.Dangerous, CreatedAt: batch.CreatedAt.UTC(), Items: items}, nil
}

func batchExecutionError(err error) string {
	if business, ok := bizerr.As(err); ok {
		code := strings.ToLower(strings.TrimPrefix(business.RuntimeCode(), "IDC_"))
		if len(code) > 0 && len(code) <= 64 {
			return code
		}
	}
	return "operation_unavailable"
}

func wrapBatchUnavailable(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(gerror.Wrap(err, "process server batch"), CodeServerBatchUnavailable)
}
