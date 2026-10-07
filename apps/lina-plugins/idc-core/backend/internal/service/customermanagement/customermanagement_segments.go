// This file implements active customer group and tag candidate pagination.

package customermanagement

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

const (
	segmentCursorVersion          = 1
	maximumSegmentCursorLength    = 1024
	maximumSegmentNamePrefixRunes = 128
)

type segmentCursorBoundary struct {
	internalID int64
	number     string
	sortName   string
}

type normalizedSegmentSearch struct {
	kind       SegmentKind
	locale     SegmentLocale
	namePrefix string
	limit      int
	cursor     *segmentCursorBoundary
}

type segmentCursorDocument struct {
	Version  int    `json:"v"`
	ID       int64  `json:"id"`
	Number   string `json:"number"`
	SortName string `json:"sort_name"`
	Digest   string `json:"digest"`
}

type segmentRow struct {
	InternalID int64  `orm:"internal_id"`
	Number     string `orm:"number"`
	Code       string `orm:"code"`
	NameZhCn   string `orm:"name_zh_cn"`
	NameEnUs   string `orm:"name_en_us"`
	ColorToken string `orm:"color_token"`
	Status     string `orm:"status_code"`
}

func normalizeSegmentSearch(input SegmentSearchInput, maximumPageSize int) (normalizedSegmentSearch, error) {
	prefix := strings.Join(strings.Fields(input.NamePrefix), " ")
	if (input.Kind != SegmentKindGroup && input.Kind != SegmentKindTag) ||
		(input.Locale != SegmentLocaleZhCN && input.Locale != SegmentLocaleEnUS) ||
		!utf8.ValidString(prefix) || utf8.RuneCountInString(prefix) > maximumSegmentNamePrefixRunes {
		return normalizedSegmentSearch{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	if input.Limit == 0 {
		input.Limit = min(defaultMaximumPageSize, maximumPageSize)
	}
	if input.Limit < 1 || input.Limit > maximumPageSize {
		return normalizedSegmentSearch{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	normalized := normalizedSegmentSearch{
		kind: input.Kind, locale: input.Locale, namePrefix: prefix, limit: input.Limit,
	}
	if input.Cursor != "" {
		boundary, err := decodeSegmentCursor(input.Cursor, normalized)
		if err != nil {
			return normalizedSegmentSearch{}, bizerr.NewCode(customer.CodeProfileInvalid)
		}
		normalized.cursor = boundary
	}
	return normalized, nil
}

func (p *daoPersistence) searchSegments(
	ctx context.Context,
	input normalizedSegmentSearch,
) (*SegmentPage, error) {
	var rows []segmentRow
	var err error
	switch input.kind {
	case SegmentKindGroup:
		columns := dao.CustomerGroup.Columns()
		sortColumn := columns.NameZhCn
		if input.locale == SegmentLocaleEnUS {
			sortColumn = columns.NameEnUs
		}
		model := dao.CustomerGroup.Ctx(ctx).Fields(
			columns.Id+" AS internal_id", columns.GroupNo+" AS number",
			columns.GroupCode+" AS code", columns.NameZhCn, columns.NameEnUs,
			"'' AS color_token", columns.StatusCode,
		).Where(do.CustomerGroup{TenantId: idccap.DefaultTenantID, StatusCode: "active"})
		if input.cursor != nil {
			count, countErr := dao.CustomerGroup.Ctx(ctx).
				Where(do.CustomerGroup{TenantId: idccap.DefaultTenantID, StatusCode: "active"}).
				Where(columns.Id, input.cursor.internalID).
				Where(columns.GroupNo, input.cursor.number).
				Where(sortColumn, input.cursor.sortName).Count()
			if countErr != nil || count != 1 {
				return nil, gerror.New("validate customer group cursor boundary: record mismatch")
			}
		}
		model = applySegmentBoundary(model, input, sortColumn, columns.Id)
		err = model.OrderAsc(sortColumn).OrderAsc(columns.Id).Limit(input.limit + 1).Scan(&rows)
	case SegmentKindTag:
		columns := dao.CustomerTag.Columns()
		sortColumn := columns.NameZhCn
		if input.locale == SegmentLocaleEnUS {
			sortColumn = columns.NameEnUs
		}
		model := dao.CustomerTag.Ctx(ctx).Fields(
			columns.Id+" AS internal_id", columns.TagNo+" AS number",
			columns.TagCode+" AS code", columns.NameZhCn, columns.NameEnUs,
			columns.ColorToken, columns.StatusCode,
		).Where(do.CustomerTag{TenantId: idccap.DefaultTenantID, StatusCode: "active"})
		if input.cursor != nil {
			count, countErr := dao.CustomerTag.Ctx(ctx).
				Where(do.CustomerTag{TenantId: idccap.DefaultTenantID, StatusCode: "active"}).
				Where(columns.Id, input.cursor.internalID).
				Where(columns.TagNo, input.cursor.number).
				Where(sortColumn, input.cursor.sortName).Count()
			if countErr != nil || count != 1 {
				return nil, gerror.New("validate customer tag cursor boundary: record mismatch")
			}
		}
		model = applySegmentBoundary(model, input, sortColumn, columns.Id)
		err = model.OrderAsc(sortColumn).OrderAsc(columns.Id).Limit(input.limit + 1).Scan(&rows)
	default:
		return nil, gerror.New("customer segment kind is invalid")
	}
	if err != nil {
		return nil, gerror.Wrap(err, "search customer segments")
	}
	return projectSegmentPage(rows, input)
}

func applySegmentBoundary(
	model *gdb.Model,
	input normalizedSegmentSearch,
	sortColumn string,
	idColumn string,
) *gdb.Model {
	if input.namePrefix != "" {
		model = model.Where(sortColumn+" LIKE ? ESCAPE '\\'", escapeLikeLiteral(input.namePrefix)+"%")
	}
	if input.cursor != nil {
		model = model.Where(
			"("+sortColumn+" > ?) OR ("+sortColumn+" = ? AND "+idColumn+" > ?)",
			input.cursor.sortName,
			input.cursor.sortName,
			input.cursor.internalID,
		)
	}
	return model
}

func projectSegmentPage(rows []segmentRow, input normalizedSegmentSearch) (*SegmentPage, error) {
	hasMore := len(rows) > input.limit
	if hasMore {
		rows = rows[:input.limit]
	}
	page := &SegmentPage{Items: make([]SegmentSummary, 0, len(rows))}
	for _, row := range rows {
		if row.InternalID <= 0 || row.Code == "" || row.NameZhCn == "" || row.NameEnUs == "" ||
			row.Status != "active" || row.Number == "" ||
			!validOptionalSegmentNo(row.Number, segmentPrefix(input.kind)) {
			return nil, gerror.New("customer segment projection is inconsistent")
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
		var err error
		page.NextCursor, err = encodeSegmentCursor(segmentCursorBoundary{
			internalID: last.InternalID, number: last.Number, sortName: sortName,
		}, input)
		if err != nil {
			return nil, err
		}
	}
	return page, nil
}

func segmentPrefix(kind SegmentKind) string {
	if kind == SegmentKindGroup {
		return "GRP_"
	}
	return "TAG_"
}

func encodeSegmentCursor(boundary segmentCursorBoundary, input normalizedSegmentSearch) (string, error) {
	digest := segmentFilterDigest(input)
	document := segmentCursorDocument{
		Version: segmentCursorVersion, ID: boundary.internalID, Number: boundary.number,
		SortName: boundary.sortName, Digest: hex.EncodeToString(digest[:16]),
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return "", gerror.Wrap(err, "encode customer segment cursor")
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeSegmentCursor(encoded string, input normalizedSegmentSearch) (*segmentCursorBoundary, error) {
	if encoded == "" || len(encoded) > maximumSegmentCursorLength {
		return nil, errInvalidCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errInvalidCursor
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var document segmentCursorDocument
	if decoder.Decode(&document) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		document.Version != segmentCursorVersion || document.ID <= 0 || document.SortName == "" ||
		!utf8.ValidString(document.SortName) || utf8.RuneCountInString(document.SortName) > 128 ||
		document.Number == "" ||
		!validOptionalSegmentNo(document.Number, segmentPrefix(input.kind)) {
		return nil, errInvalidCursor
	}
	expected := segmentFilterDigest(input)
	actual, err := hex.DecodeString(document.Digest)
	if err != nil || len(actual) != 16 || subtle.ConstantTimeCompare(actual, expected[:16]) != 1 {
		return nil, errInvalidCursor
	}
	return &segmentCursorBoundary{
		internalID: document.ID, number: document.Number, sortName: document.SortName,
	}, nil
}

func segmentFilterDigest(input normalizedSegmentSearch) [sha256.Size]byte {
	return sha256.Sum256([]byte(string(input.kind) + "\x00" + string(input.locale) + "\x00" +
		input.namePrefix + "\x00" + strconv.Itoa(input.limit)))
}

func validateSegmentPage(page *SegmentPage, maximumPageSize int, maximumResponseBytes int) error {
	if page == nil || page.Items == nil || len(page.Items) > maximumPageSize ||
		len(page.NextCursor) > maximumSegmentCursorLength {
		return gerror.New("customer segment page projection is invalid")
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		return gerror.Wrap(err, "measure customer segment page")
	}
	if len(encoded) > maximumResponseBytes {
		return gerror.New("customer segment page exceeds response safety bound")
	}
	return nil
}
