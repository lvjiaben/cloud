// Package batchrenewal coordinates staff-initiated renewals for resource-owned servers.
package batchrenewal

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

const (
	PermissionView = "idc:admin:resource:server-batch:view"
	PermissionSafe = "idc:admin:resource:server-batch:safe"
	defaultLimit   = 25
	maximumLimit   = 100
)

var (
	CodeInvalid       = bizerr.MustDefine("IDC_RESOURCE_BATCH_RENEWAL_INVALID", "The server renewal request is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible = bizerr.MustDefine("IDC_RESOURCE_BATCH_RENEWAL_NOT_ACCESSIBLE", "The server renewal operation is not accessible", gcode.CodeNotAuthorized)
	CodeConflict      = bizerr.MustDefine("IDC_RESOURCE_BATCH_RENEWAL_CONFLICT", "The server renewal preview has changed", gcode.CodeInvalidOperation)
	CodeUnavailable   = bizerr.MustDefine("IDC_RESOURCE_BATCH_RENEWAL_UNAVAILABLE", "The server renewal operation is temporarily unavailable", gcode.CodeInternalError)
	serverNoPattern   = regexp.MustCompile(`^SVR-[0-9A-F]{32}$`)
)

type Candidate struct {
	ServerNo          string `orm:"server_no"`
	EntitlementNo     string `orm:"entitlement_no"`
	CustomerNo        string `orm:"customer_no"`
	ProductNameZhCN   string `orm:"product_name_zh_cn"`
	ProductNameEnUS   string `orm:"product_name_en_us"`
	ProjectionVersion int64  `orm:"projection_version"`
}

type CandidatePage struct {
	Items      []Candidate
	Total      int
	NextCursor string
}

// ListInput bounds the local candidate directory without changing renewal eligibility.
type ListInput struct {
	Cursor string
	Limit  int
	Page   int
}

type Store interface {
	List(context.Context, ListInput) (*CandidatePage, error)
	Load(context.Context, []string) ([]Candidate, error)
}

type PreviewItem struct {
	ServerNo, ProductNameZhCN, ProductNameEnUS, CurrencyCode, FailureCode string
	ProjectionVersion, TotalAmountMinor                                   int64
	OriginalExpiresAt, ProjectedExpiresAt, QuoteExpiresAt                 *time.Time
	Eligible                                                              bool
	quoteNo, customerNo, entitlementNo                                    string
}

type Preview struct {
	RequestKey, ConfirmationDigest string
	RenewalCycles                  int
	Items                          []PreviewItem
}

type ResultItem struct {
	ServerNo, ProductNameZhCN, ProductNameEnUS, CurrencyCode, FailureCode, Status string
	TotalAmountMinor                                                              int64
	Succeeded                                                                     bool
}

type Confirmation struct {
	RequestKey    string
	RenewalCycles int
	Items         []ResultItem
}

type Service interface {
	List(context.Context, ListInput) (*CandidatePage, error)
	Preview(context.Context, string, int, []string) (*Preview, error)
	Confirm(context.Context, string, int, []string, string) (*Confirmation, error)
}

type serviceImpl struct {
	store    Store
	runtime  managementcap.Runtime
	renewals managementcap.StaffRenewalManager
}

func New(database gdb.DB, runtime managementcap.Runtime) (Service, error) {
	if nilInterface(database) {
		return nil, gerror.New("server batch renewal database is required")
	}
	return NewWithStore(&sqlStore{database: database}, runtime)
}

func NewWithStore(store Store, runtime managementcap.Runtime) (Service, error) {
	if nilInterface(store) || managementcap.ValidateRuntime(runtime) != nil || nilInterface(runtime.Renewals) {
		return nil, gerror.New("server batch renewal dependencies are invalid")
	}
	return &serviceImpl{store: store, runtime: runtime, renewals: runtime.Renewals}, nil
}

// List returns one staff-authorized local server projection page and an exact count.
func (s *serviceImpl) List(ctx context.Context, input ListInput) (*CandidatePage, error) {
	if !validContext(ctx) || (input.Cursor != "" && !serverNoPattern.MatchString(input.Cursor)) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if input.Limit == 0 {
		input.Limit = defaultLimit
	}
	if input.Limit < 1 || input.Limit > maximumLimit || input.Page < 0 || input.Page > 1_000_000 ||
		(input.Page > 0 && input.Cursor != "") {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if !s.authorize(ctx, PermissionView) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	page, err := s.store.List(ctx, input)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if page == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return page, nil
}

func (s *serviceImpl) Preview(ctx context.Context, requestKey string, cycles int, serverNos []string) (*Preview, error) {
	if !validContext(ctx) || !validBatch(requestKey, cycles, serverNos) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if !s.authorize(ctx, PermissionView) || !s.authorize(ctx, PermissionSafe) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return s.preview(ctx, requestKey, cycles, serverNos)
}

func (s *serviceImpl) Confirm(ctx context.Context, requestKey string, cycles int, serverNos []string, digest string) (*Confirmation, error) {
	if !validContext(ctx) || !validBatch(requestKey, cycles, serverNos) || len(digest) != sha256.Size*2 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if !s.authorize(ctx, PermissionView) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{Permission: PermissionSafe, TargetNo: "server-renewals"})
	if err != nil || actor <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	preview, err := s.previewAuthorized(ctx, requestKey, cycles, serverNos)
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(preview.ConfirmationDigest), []byte(strings.ToLower(digest))) != 1 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	result := &Confirmation{RequestKey: requestKey, RenewalCycles: cycles, Items: make([]ResultItem, len(preview.Items))}
	for index := range preview.Items {
		item := preview.Items[index]
		out := ResultItem{ServerNo: item.ServerNo, ProductNameZhCN: item.ProductNameZhCN, ProductNameEnUS: item.ProductNameEnUS,
			CurrencyCode: item.CurrencyCode, TotalAmountMinor: item.TotalAmountMinor}
		if !item.Eligible {
			out.FailureCode = item.FailureCode
			result.Items[index] = out
			continue
		}
		operation, confirmErr := s.renewals.ConfirmForStaff(ctx, managementcap.StaffRenewalConfirmInput{
			CustomerNo: item.customerNo, QuoteNo: item.quoteNo,
			IdempotencyKey: idempotency("resource-renewal-confirm", requestKey, item.ServerNo, cycles, item.ProjectionVersion),
			ActorUserID:    actor,
		})
		if confirmErr != nil || operation == nil {
			out.FailureCode = "confirmation_failed"
		} else {
			out.Succeeded = true
			out.Status = operation.Status
		}
		result.Items[index] = out
	}
	return result, nil
}

func (s *serviceImpl) preview(ctx context.Context, requestKey string, cycles int, serverNos []string) (*Preview, error) {
	return s.previewAuthorized(ctx, requestKey, cycles, serverNos)
}

func (s *serviceImpl) previewAuthorized(ctx context.Context, requestKey string, cycles int, serverNos []string) (*Preview, error) {
	rows, err := s.store.Load(ctx, serverNos)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	byNo := make(map[string]Candidate, len(rows))
	for _, row := range rows {
		byNo[row.ServerNo] = row
	}
	result := &Preview{RequestKey: requestKey, RenewalCycles: cycles, Items: make([]PreviewItem, len(serverNos))}
	for index, serverNo := range serverNos {
		row, ok := byNo[serverNo]
		if !ok {
			result.Items[index] = PreviewItem{ServerNo: serverNo, FailureCode: "not_available"}
			continue
		}
		item := PreviewItem{ServerNo: row.ServerNo, ProductNameZhCN: row.ProductNameZhCN,
			ProductNameEnUS: row.ProductNameEnUS, ProjectionVersion: row.ProjectionVersion,
			customerNo: row.CustomerNo, entitlementNo: row.EntitlementNo}
		quote, quoteErr := s.renewals.QuoteForStaff(ctx, managementcap.StaffRenewalQuoteInput{
			CustomerNo: row.CustomerNo, EntitlementNo: row.EntitlementNo,
			IdempotencyKey: idempotency("resource-renewal-quote", requestKey, row.ServerNo, cycles, row.ProjectionVersion),
			RenewalCycles:  cycles,
		})
		if quoteErr != nil || quote == nil {
			item.FailureCode = "quote_unavailable"
		} else {
			item.Eligible = true
			item.quoteNo = quote.QuoteNo
			item.CurrencyCode = quote.CurrencyCode
			item.TotalAmountMinor = quote.TotalAmountMinor
			item.OriginalExpiresAt = utcPointer(quote.OriginalExpiresAt)
			item.ProjectedExpiresAt = utcPointer(quote.ProjectedExpiresAt)
			item.QuoteExpiresAt = utcPointer(quote.ExpiresAt)
		}
		result.Items[index] = item
	}
	result.ConfirmationDigest, err = digestPreview(result)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}

func (s *serviceImpl) authorize(ctx context.Context, permission string) bool {
	actor, err := s.runtime.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{Permission: permission, TargetNo: "server-renewals"})
	return err == nil && actor > 0
}

func validContext(ctx context.Context) bool { return ctx != nil && ctx.Err() == nil }

func nilInterface(value any) bool {
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

func validBatch(requestKey string, cycles int, serverNos []string) bool {
	if requestKey != strings.TrimSpace(requestKey) || len(requestKey) < 8 || len(requestKey) > 128 || cycles < 1 || cycles > 120 || len(serverNos) < 1 || len(serverNos) > managementcap.MaximumStaffRenewalBatch {
		return false
	}
	seen := make(map[string]struct{}, len(serverNos))
	for _, serverNo := range serverNos {
		if !serverNoPattern.MatchString(serverNo) {
			return false
		}
		if _, exists := seen[serverNo]; exists {
			return false
		}
		seen[serverNo] = struct{}{}
	}
	return true
}

func idempotency(domain, requestKey, serverNo string, cycles int, version int64) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%d", requestKey, serverNo, cycles, version)))
	return domain + "-" + hex.EncodeToString(digest[:])
}

func digestPreview(preview *Preview) (string, error) {
	type canonicalItem struct {
		ServerNo, EntitlementNo, CustomerNo, ProductNameZhCN, ProductNameEnUS string
		CurrencyCode, FailureCode, QuoteNo                                    string
		ProjectionVersion, TotalAmountMinor                                   int64
		OriginalExpiresAt, ProjectedExpiresAt, QuoteExpiresAt                 *time.Time
		Eligible                                                              bool
	}
	items := make([]canonicalItem, len(preview.Items))
	for index, item := range preview.Items {
		items[index] = canonicalItem{ServerNo: item.ServerNo, EntitlementNo: item.entitlementNo,
			CustomerNo: item.customerNo, ProductNameZhCN: item.ProductNameZhCN,
			ProductNameEnUS: item.ProductNameEnUS, CurrencyCode: item.CurrencyCode,
			FailureCode: item.FailureCode, QuoteNo: item.quoteNo, ProjectionVersion: item.ProjectionVersion,
			TotalAmountMinor: item.TotalAmountMinor, OriginalExpiresAt: item.OriginalExpiresAt,
			ProjectedExpiresAt: item.ProjectedExpiresAt, QuoteExpiresAt: item.QuoteExpiresAt,
			Eligible: item.Eligible}
	}
	canonical := struct {
		Version       string          `json:"version"`
		RequestKey    string          `json:"requestKey"`
		RenewalCycles int             `json:"renewalCycles"`
		Items         []canonicalItem `json:"items"`
	}{Version: "idc-resource-server-renewal-v1", RequestKey: preview.RequestKey, RenewalCycles: preview.RenewalCycles, Items: items}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func utcPointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	result := value.UTC()
	return &result
}

type sqlStore struct{ database gdb.DB }

// List counts and pages the same tenant-scoped entitlement projection.
func (s *sqlStore) List(ctx context.Context, input ListInput) (*CandidatePage, error) {
	model := s.database.Model("plugin_idc_resource_server_entitlement_projection").Safe().Ctx(ctx).
		Where("tenant_id", idccap.DefaultTenantID)
	total, err := model.Clone().Count()
	if err != nil {
		return nil, err
	}
	if input.Page > 0 {
		model = model.Offset((input.Page - 1) * input.Limit)
	}
	var rows []Candidate
	err = model.Fields("server_no,entitlement_no,customer_no,COALESCE(product_name_zh_cn,'') product_name_zh_cn,COALESCE(product_name_en_us,'') product_name_en_us,version projection_version").
		WhereGT("server_no", input.Cursor).OrderAsc("server_no").Limit(input.Limit + 1).Scan(&rows)
	if err != nil {
		return nil, err
	}
	page := &CandidatePage{Items: rows, Total: total}
	if len(rows) > input.Limit {
		page.Items = rows[:input.Limit]
		page.NextCursor = page.Items[len(page.Items)-1].ServerNo
	}
	return page, nil
}

func (s *sqlStore) Load(ctx context.Context, serverNos []string) ([]Candidate, error) {
	var rows []Candidate
	err := s.database.Model("plugin_idc_resource_server_entitlement_projection").Ctx(ctx).
		Fields("server_no,entitlement_no,customer_no,COALESCE(product_name_zh_cn,'') product_name_zh_cn,COALESCE(product_name_en_us,'') product_name_en_us,version projection_version").
		Where("tenant_id", idccap.DefaultTenantID).WhereIn("server_no", serverNos).Scan(&rows)
	return rows, err
}
