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
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

const maximumAssignmentCursorLength = 1024

type normalizedAssignmentSearch struct {
	customerNo string
	kind       SegmentKind
	locale     SegmentLocale
	limit      int
	cursor     *assignmentBoundary
}

type assignmentBoundary struct {
	assignmentID int64
	number       string
	sortName     string
}

type assignmentCursorDocument struct {
	Version      int    `json:"v"`
	AssignmentID int64  `json:"assignment_id"`
	Number       string `json:"number"`
	SortName     string `json:"sort_name"`
	Digest       string `json:"digest"`
}

type assignmentRow struct {
	AssignmentID int64  `orm:"assignment_id"`
	Number       string `orm:"number"`
	Code         string `orm:"code"`
	NameZhCn     string `orm:"name_zh_cn"`
	NameEnUs     string `orm:"name_en_us"`
	ColorToken   string `orm:"color_token"`
	Status       string `orm:"status_code"`
}

func (s *serviceImpl) SearchAssignments(
	ctx context.Context,
	input AssignmentSearchInput,
) (*AssignmentPage, error) {
	if err := s.authorize(ctx, permissionCustomerQuery); err != nil {
		return nil, err
	}
	normalized, err := normalizeAssignmentSearch(input, s.config.MaximumPageSize)
	if err != nil {
		return nil, err
	}
	page, err := searchAssignments(ctx, normalized)
	if err != nil {
		if bizerr.Is(err, customer.CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	encoded, err := json.Marshal(page)
	if err != nil || len(encoded) > s.config.MaximumResponseBytes {
		return nil, bizerr.NewCode(customer.CodeUnavailable)
	}
	return page, nil
}

func normalizeAssignmentSearch(input AssignmentSearchInput, maximumPageSize int) (normalizedAssignmentSearch, error) {
	if !validPersistedCustomerNo(input.CustomerNo) ||
		(input.Kind != SegmentKindGroup && input.Kind != SegmentKindTag) ||
		(input.Locale != SegmentLocaleZhCN && input.Locale != SegmentLocaleEnUS) {
		return normalizedAssignmentSearch{}, bizerr.NewCode(customer.CodeNotAccessible)
	}
	if input.Limit == 0 {
		input.Limit = min(defaultMaximumPageSize, maximumPageSize)
	}
	if input.Limit < 1 || input.Limit > maximumPageSize {
		return normalizedAssignmentSearch{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	normalized := normalizedAssignmentSearch{
		customerNo: input.CustomerNo, kind: input.Kind, locale: input.Locale, limit: input.Limit,
	}
	if input.Cursor != "" {
		boundary, err := decodeAssignmentCursor(input.Cursor, normalized)
		if err != nil {
			return normalizedAssignmentSearch{}, bizerr.NewCode(customer.CodeProfileInvalid)
		}
		normalized.cursor = boundary
	}
	return normalized, nil
}

func searchAssignments(ctx context.Context, input normalizedAssignmentSearch) (*AssignmentPage, error) {
	count, err := dao.CustomerSubject.Ctx(ctx).Where("tenant_id", idccap.DefaultTenantID).
		Where("customer_no", input.customerNo).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "validate customer assignment owner")
	}
	if count != 1 {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	definitionTable, assignmentTable := dao.CustomerTag.Table(), dao.CustomerTagAssignment.Table()
	numberColumn, codeColumn, colorProjection := "tag_no", "tag_code", "d.color_token"
	if input.kind == SegmentKindGroup {
		definitionTable, assignmentTable = dao.CustomerGroup.Table(), dao.CustomerGroupAssignment.Table()
		numberColumn, codeColumn, colorProjection = "group_no", "group_code", "''"
	}
	sortColumn := "d.name_zh_cn"
	if input.locale == SegmentLocaleEnUS {
		sortColumn = "d.name_en_us"
	}
	base := fmt.Sprintf(` FROM %s a INNER JOIN %s d ON d.%s=a.%s
WHERE a.tenant_id=? AND a.customer_no=? AND a.status_code='active'`,
		assignmentTable, definitionTable, numberColumn, numberColumn)
	if input.cursor != nil {
		boundaryQuery := `SELECT COUNT(*)` + base + fmt.Sprintf(` AND a.id=? AND d.%s=? AND %s=?`, numberColumn, sortColumn)
		value, boundaryErr := g.DB().GetValue(ctx, boundaryQuery, idccap.DefaultTenantID, input.customerNo,
			input.cursor.assignmentID, input.cursor.number, input.cursor.sortName)
		if boundaryErr != nil || value.Int() != 1 {
			return nil, gerror.New("validate customer assignment cursor boundary: record mismatch")
		}
	}
	query := fmt.Sprintf(`SELECT a.id AS assignment_id,d.%s AS number,d.%s AS code,
d.name_zh_cn,d.name_en_us,%s AS color_token,d.status_code`, numberColumn, codeColumn, colorProjection) + base
	arguments := []any{idccap.DefaultTenantID, input.customerNo}
	if input.cursor != nil {
		query += fmt.Sprintf(` AND ((%s>?) OR (%s=? AND a.id>?))`, sortColumn, sortColumn)
		arguments = append(arguments, input.cursor.sortName, input.cursor.sortName, input.cursor.assignmentID)
	}
	query += fmt.Sprintf(` ORDER BY %s ASC,a.id ASC LIMIT ?`, sortColumn)
	arguments = append(arguments, input.limit+1)
	var rows []assignmentRow
	if err = g.DB().GetScan(ctx, &rows, query, arguments...); err != nil {
		return nil, gerror.Wrap(err, "search customer assignments")
	}
	hasMore := len(rows) > input.limit
	if hasMore {
		rows = rows[:input.limit]
	}
	page := &AssignmentPage{Items: make([]SegmentSummary, 0, len(rows))}
	for _, row := range rows {
		if row.AssignmentID <= 0 || row.Number == "" || row.Code == "" ||
			row.NameZhCn == "" || row.NameEnUs == "" || row.Status != "active" {
			return nil, gerror.New("customer assignment projection is inconsistent")
		}
		page.Items = append(page.Items, SegmentSummary{
			Number: row.Number, Code: row.Code, NameZhCn: row.NameZhCn,
			NameEnUs: row.NameEnUs, ColorToken: row.ColorToken, Status: row.Status,
		})
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		sortName := last.NameZhCn
		if input.locale == SegmentLocaleEnUS {
			sortName = last.NameEnUs
		}
		page.NextCursor, err = encodeAssignmentCursor(assignmentBoundary{
			assignmentID: last.AssignmentID, number: last.Number, sortName: sortName,
		}, input)
		if err != nil {
			return nil, err
		}
	}
	return page, nil
}

func encodeAssignmentCursor(boundary assignmentBoundary, input normalizedAssignmentSearch) (string, error) {
	digest := assignmentFilterDigest(input)
	document := assignmentCursorDocument{Version: 1, AssignmentID: boundary.assignmentID,
		Number: boundary.number, SortName: boundary.sortName, Digest: hex.EncodeToString(digest[:16])}
	payload, err := json.Marshal(document)
	if err != nil {
		return "", gerror.Wrap(err, "encode customer assignment cursor")
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeAssignmentCursor(encoded string, input normalizedAssignmentSearch) (*assignmentBoundary, error) {
	if encoded == "" || len(encoded) > maximumAssignmentCursorLength {
		return nil, errInvalidCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errInvalidCursor
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var document assignmentCursorDocument
	if decoder.Decode(&document) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		document.Version != 1 || document.AssignmentID <= 0 || document.Number == "" ||
		document.SortName == "" || !utf8.ValidString(document.SortName) {
		return nil, errInvalidCursor
	}
	expected := assignmentFilterDigest(input)
	actual, err := hex.DecodeString(document.Digest)
	if err != nil || len(actual) != 16 || subtle.ConstantTimeCompare(actual, expected[:16]) != 1 {
		return nil, errInvalidCursor
	}
	return &assignmentBoundary{assignmentID: document.AssignmentID, number: document.Number, sortName: document.SortName}, nil
}

func assignmentFilterDigest(input normalizedAssignmentSearch) [sha256.Size]byte {
	return sha256.Sum256([]byte(input.customerNo + "\x00" + string(input.kind) + "\x00" +
		string(input.locale) + "\x00" + strconv.Itoa(input.limit)))
}
