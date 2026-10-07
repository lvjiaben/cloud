package customermanagement

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

type activityBoundary struct {
	id        int64
	createdAt time.Time
}

type normalizedActivitySearch struct {
	customerNo string
	kind       ActivityKind
	limit      int
	cursor     *activityBoundary
}

type activityCursorDocument struct {
	Version   int    `json:"v"`
	ID        int64  `json:"id"`
	CreatedAt int64  `json:"created_at"`
	Digest    string `json:"digest"`
}

type activityRow struct {
	ID        int64      `orm:"internal_id"`
	Number    string     `orm:"number"`
	Category  string     `orm:"category"`
	Status    string     `orm:"status_code"`
	Title     string     `orm:"title"`
	Detail    string     `orm:"detail"`
	RiskLevel string     `orm:"risk_level"`
	PlannedAt *time.Time `orm:"planned_at"`
	CreatedAt time.Time  `orm:"created_at"`
	UpdatedAt time.Time  `orm:"updated_at"`
}

func (s *serviceImpl) SearchActivities(ctx context.Context, input ActivitySearchInput) (*ActivityPage, error) {
	if err := s.authorize(ctx, permissionCustomerQuery); err != nil {
		return nil, err
	}
	normalized, err := normalizeActivitySearch(input, s.config.MaximumPageSize)
	if err != nil {
		return nil, err
	}
	page, err := searchActivities(ctx, normalized)
	if err != nil {
		if bizerr.Is(err, customer.CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	payload, err := json.Marshal(page)
	if err != nil || len(payload) > s.config.MaximumResponseBytes {
		return nil, bizerr.NewCode(customer.CodeUnavailable)
	}
	return page, nil
}

func normalizeActivitySearch(input ActivitySearchInput, maximumPageSize int) (normalizedActivitySearch, error) {
	if !validPersistedCustomerNo(input.CustomerNo) ||
		(input.Kind != ActivityKindNote && input.Kind != ActivityKindFollowUp && input.Kind != ActivityKindBlacklist) {
		return normalizedActivitySearch{}, bizerr.NewCode(customer.CodeNotAccessible)
	}
	if input.Limit == 0 {
		input.Limit = min(defaultMaximumPageSize, maximumPageSize)
	}
	if input.Limit < 1 || input.Limit > maximumPageSize {
		return normalizedActivitySearch{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	normalized := normalizedActivitySearch{customerNo: input.CustomerNo, kind: input.Kind, limit: input.Limit}
	if input.Cursor != "" {
		boundary, err := decodeActivityCursor(input.Cursor, normalized)
		if err != nil {
			return normalizedActivitySearch{}, bizerr.NewCode(customer.CodeProfileInvalid)
		}
		normalized.cursor = boundary
	}
	return normalized, nil
}

func searchActivities(ctx context.Context, input normalizedActivitySearch) (*ActivityPage, error) {
	count, err := dao.CustomerSubject.Ctx(ctx).Where("tenant_id", 0).Where("customer_no", input.customerNo).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "validate customer activity owner")
	}
	if count != 1 {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	table, numberColumn, projection := activityProjection(input.kind)
	if table == "" {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	if input.cursor != nil {
		value, boundaryErr := g.DB().GetValue(ctx, fmt.Sprintf(`SELECT id FROM %s WHERE tenant_id=0 AND customer_no=? AND id=? AND created_at=?`, table), input.customerNo, input.cursor.id, input.cursor.createdAt)
		if boundaryErr != nil || value.Int64() <= 0 {
			return nil, gerror.New("validate customer activity cursor boundary: record mismatch")
		}
	}
	query := fmt.Sprintf(`SELECT id AS internal_id,%s AS number,%s FROM %s WHERE tenant_id=0 AND customer_no=?`, numberColumn, projection, table)
	arguments := []any{input.customerNo}
	if input.cursor != nil {
		query += ` AND ((created_at<?) OR (created_at=? AND id<?))`
		arguments = append(arguments, input.cursor.createdAt, input.cursor.createdAt, input.cursor.id)
	}
	query += ` ORDER BY created_at DESC,id DESC LIMIT ?`
	arguments = append(arguments, input.limit+1)
	var rows []activityRow
	if err = g.DB().GetScan(ctx, &rows, query, arguments...); err != nil {
		return nil, gerror.Wrap(err, "search customer activity history")
	}
	hasMore := len(rows) > input.limit
	if hasMore {
		rows = rows[:input.limit]
	}
	page := &ActivityPage{Items: make([]ActivityItem, 0, len(rows))}
	for _, row := range rows {
		if row.ID <= 0 || row.Number == "" || row.Status == "" || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
			return nil, gerror.New("customer activity projection is inconsistent")
		}
		page.Items = append(page.Items, ActivityItem{Number: row.Number, Kind: input.kind,
			Category: row.Category, Status: row.Status, Title: row.Title, Detail: row.Detail,
			RiskLevel: row.RiskLevel, PlannedAt: row.PlannedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor, err = encodeActivityCursor(activityBoundary{id: last.ID, createdAt: last.CreatedAt}, input)
		if err != nil {
			return nil, err
		}
	}
	return page, nil
}

func activityProjection(kind ActivityKind) (string, string, string) {
	switch kind {
	case ActivityKindNote:
		return dao.CustomerNote.Table(), "note_no", `category_code AS category,status_code,content AS title,'' AS detail,'' AS risk_level,NULL::timestamptz AS planned_at,created_at,updated_at`
	case ActivityKindFollowUp:
		return dao.CustomerFollowUp.Table(), "follow_up_no", `channel_code AS category,status_code,subject AS title,detail,'' AS risk_level,planned_at,created_at,updated_at`
	case ActivityKindBlacklist:
		return dao.CustomerBlacklistEntry.Table(), "entry_no", `scope_code AS category,status_code,reason_code AS title,reason_detail AS detail,risk_level_code AS risk_level,starts_at AS planned_at,created_at,updated_at`
	default:
		return "", "", ""
	}
}

func encodeActivityCursor(boundary activityBoundary, input normalizedActivitySearch) (string, error) {
	digest := activityFilterDigest(input)
	payload, err := json.Marshal(activityCursorDocument{Version: 1, ID: boundary.id,
		CreatedAt: boundary.createdAt.UnixNano(), Digest: hex.EncodeToString(digest[:16])})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeActivityCursor(encoded string, input normalizedActivitySearch) (*activityBoundary, error) {
	if encoded == "" || len(encoded) > 1024 {
		return nil, errInvalidCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errInvalidCursor
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var document activityCursorDocument
	if decoder.Decode(&document) != nil || decoder.Decode(&struct{}{}) != io.EOF || document.Version != 1 || document.ID <= 0 || document.CreatedAt <= 0 {
		return nil, errInvalidCursor
	}
	expected := activityFilterDigest(input)
	actual, err := hex.DecodeString(document.Digest)
	if err != nil || len(actual) != 16 || subtle.ConstantTimeCompare(actual, expected[:16]) != 1 {
		return nil, errInvalidCursor
	}
	return &activityBoundary{id: document.ID, createdAt: time.Unix(0, document.CreatedAt)}, nil
}

func activityFilterDigest(input normalizedActivitySearch) [sha256.Size]byte {
	return sha256.Sum256([]byte(input.customerNo + "\x00" + string(input.kind) + "\x00" + strconv.Itoa(input.limit)))
}
