// This file implements one sealed audit event with immutable customer lookup links.

package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const (
	maximumAffectedCustomers      = 100
	batchCustomerCountContextKey  = "affected_customer_count"
	batchCustomerDigestContextKey = "affected_customer_digest"
	batchCustomerDigestDomain     = "audit-affected-customers-v1"
)

// AppendBatchTransition appends one event and a bounded set of immutable,
// customer-searchable links inside the caller's transaction.
func (s *serviceImpl) AppendBatchTransition(
	ctx context.Context,
	input BatchTransitionInput,
) (*AppendResult, error) {
	if gdb.TXFromCtx(ctx, dao.BusinessAuditEvent.Group()) == nil {
		return nil, bizerr.NewCode(CodeTransactionRequired)
	}
	if input.BeforeSnapshot == nil || input.AfterSnapshot == nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	normalized, err := s.normalizeBatchEntry(ctx, input)
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	event, replayed, err := s.appendInTransactionWithReplay(ctx, normalized)
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return &AppendResult{AuditNo: event.AuditNo, Replayed: replayed}, nil
}

func (s *serviceImpl) normalizeBatchEntry(
	ctx context.Context,
	input BatchTransitionInput,
) (normalizedEntry, error) {
	if input.CustomerNo != "" {
		return normalizedEntry{}, bizerr.NewCode(CodeInvalid)
	}
	customers, err := normalizeAffectedCustomerNos(input.AffectedCustomerNos)
	if err != nil {
		return normalizedEntry{}, err
	}
	normalized, err := s.normalizeEntry(
		ctx,
		entryKindTransition,
		input.Fields,
		input.BeforeSnapshot,
		input.AfterSnapshot,
	)
	if err != nil {
		return normalizedEntry{}, err
	}
	var originalContext map[string]any
	if err = json.Unmarshal(normalized.callerContextOriginal, &originalContext); err != nil {
		return normalizedEntry{}, gerror.Wrap(err, "decode normalized batch audit context")
	}
	if _, exists := originalContext[batchCustomerCountContextKey]; exists {
		return normalizedEntry{}, bizerr.NewCode(CodeInvalid)
	}
	if _, exists := originalContext[batchCustomerDigestContextKey]; exists {
		return normalizedEntry{}, bizerr.NewCode(CodeInvalid)
	}
	digest := affectedCustomerDigest(customers)
	originalContext[batchCustomerCountContextKey] = len(customers)
	originalContext[batchCustomerDigestContextKey] = digest
	normalized.callerContext[batchCustomerCountContextKey] = len(customers)
	normalized.callerContext[batchCustomerDigestContextKey] = digest
	normalized.callerContextOriginal, err = json.Marshal(originalContext)
	if err != nil {
		return normalizedEntry{}, gerror.Wrap(err, "encode normalized batch audit context")
	}
	normalized.affectedCustomerNos = customers
	return normalized, nil
}

func normalizeAffectedCustomerNos(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > maximumAffectedCustomers {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	normalized := append([]string(nil), values...)
	sort.Strings(normalized)
	for index, value := range normalized {
		if !validBatchCustomerNo(value) || (index > 0 && value == normalized[index-1]) {
			return nil, bizerr.NewCode(CodeInvalid)
		}
	}
	return normalized, nil
}

func validBatchCustomerNo(value string) bool {
	if len(value) != 36 || !strings.HasPrefix(value, "CUS_") {
		return false
	}
	for _, character := range value[4:] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func affectedCustomerDigest(customers []string) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(batchCustomerDigestDomain))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write([]byte(strconv.Itoa(len(customers))))
	for _, customerNo := range customers {
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(customerNo))
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func insertAuditCustomerLinks(ctx context.Context, auditNo string, customers []string) error {
	if len(customers) == 0 {
		return nil
	}
	rows := make([]do.BusinessAuditCustomerLink, 0, len(customers))
	for _, customerNo := range customers {
		rows = append(rows, do.BusinessAuditCustomerLink{
			TenantId: idccap.DefaultTenantID, AuditNo: auditNo, CustomerNo: customerNo,
		})
	}
	if _, err := dao.BusinessAuditCustomerLink.Ctx(ctx).Data(rows).Insert(); err != nil {
		return gerror.Wrap(err, "insert batch audit customer links")
	}
	return nil
}

func validateAuditCustomerLinks(ctx context.Context, auditNo string, input normalizedEntry) error {
	if len(input.affectedCustomerNos) == 0 {
		return nil
	}
	columns := dao.BusinessAuditCustomerLink.Columns()
	var links []*entity.BusinessAuditCustomerLink
	if err := dao.BusinessAuditCustomerLink.Ctx(ctx).
		Fields(columns.CustomerNo).
		Where(do.BusinessAuditCustomerLink{
			TenantId: idccap.DefaultTenantID,
			AuditNo:  auditNo,
		}).
		OrderAsc(columns.CustomerNo).
		Scan(&links); err != nil {
		return gerror.Wrap(err, "load batch audit customer links")
	}
	if len(links) != len(input.affectedCustomerNos) {
		return gerror.New("batch audit customer links are incomplete")
	}
	for index, link := range links {
		if link == nil || link.CustomerNo != input.affectedCustomerNos[index] {
			return gerror.New("batch audit customer links do not match sealed evidence")
		}
	}
	return nil
}

func loadAuditCustomerLinks(
	ctx context.Context,
	rows []*entity.BusinessAuditEvent,
) (map[string][]string, error) {
	result := make(map[string][]string)
	if len(rows) == 0 {
		return result, nil
	}
	auditNumbers := make([]string, 0, len(rows))
	for _, row := range rows {
		if row == nil || row.AuditNo == "" {
			return nil, gerror.New("audit customer link projection is missing an audit number")
		}
		auditNumbers = append(auditNumbers, row.AuditNo)
	}
	columns := dao.BusinessAuditCustomerLink.Columns()
	var links []*entity.BusinessAuditCustomerLink
	if err := dao.BusinessAuditCustomerLink.Ctx(ctx).
		Fields(columns.AuditNo, columns.CustomerNo).
		Where(do.BusinessAuditCustomerLink{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.AuditNo, auditNumbers).
		OrderAsc(columns.AuditNo).
		OrderAsc(columns.CustomerNo).
		Scan(&links); err != nil {
		return nil, gerror.Wrap(err, "batch load audit customer links")
	}
	for _, link := range links {
		if link == nil || link.AuditNo == "" || !validBatchCustomerNo(link.CustomerNo) {
			return nil, gerror.New("persisted audit customer link is invalid")
		}
		result[link.AuditNo] = append(result[link.AuditNo], link.CustomerNo)
	}
	return result, nil
}

func validateVerifiedAffectedCustomers(verified *verifiedEvidence, customers []string) error {
	if verified == nil {
		return gerror.New("verified batch audit evidence is unavailable")
	}
	countValue, hasCount := verified.parsed.data[batchCustomerCountContextKey]
	digestValue, hasDigest := verified.parsed.data[batchCustomerDigestContextKey]
	if !hasCount && !hasDigest {
		if len(customers) != 0 {
			return gerror.New("unsealed audit customer links were found")
		}
		return nil
	}
	if !hasCount || !hasDigest || len(customers) == 0 || len(customers) > maximumAffectedCustomers {
		return gerror.New("sealed batch audit customer metadata is incomplete")
	}
	count, countOK := strictJSONInt(countValue)
	digest, digestOK := digestValue.(string)
	if !countOK || count != len(customers) || !digestOK ||
		!isCanonicalDigest(digest) || digest != affectedCustomerDigest(customers) {
		return gerror.New("sealed batch audit customer metadata does not match its links")
	}
	for index, customerNo := range customers {
		if !validBatchCustomerNo(customerNo) || (index > 0 && customerNo <= customers[index-1]) {
			return gerror.New("sealed batch audit customer links are not canonical")
		}
	}
	return nil
}
