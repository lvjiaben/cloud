// This file implements tamper-evident keyset cursors for the review directory.

package enterpriseverification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// enterpriseCaseCursorPrefix versions the signed management cursor format.
const enterpriseCaseCursorPrefix = "ec1"

// enterpriseCaseCursor binds pagination position to the exact filter set.
type enterpriseCaseCursor struct {
	Version     int    `json:"v"`
	StatusCode  string `json:"status"`
	CustomerNo  string `json:"customer"`
	SubmittedAt int64  `json:"submittedAt"`
	CaseNo      string `json:"caseNo"`
}

// encodeCaseCursor signs one microsecond-precision keyset position.
func (s *managementServiceImpl) encodeCaseCursor(
	ctx context.Context,
	input CaseListInput,
	item CaseListItem,
) (string, error) {
	keyNo, err := ensureEnterpriseKey(ctx, s.keyManager)
	if err != nil {
		return "", err
	}
	payload := enterpriseCaseCursor{
		Version: 1, StatusCode: input.StatusCode, CustomerNo: input.CustomerNo,
		SubmittedAt: item.SubmittedAt.UTC().UnixMicro(), CaseNo: item.CaseNo,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", gerror.New("enterprise verification cursor is unavailable")
	}
	payloadSegment := base64.RawURLEncoding.EncodeToString(encoded)
	var signature []byte
	err = accessEnterpriseKey(ctx, s.keyAccessor, keyNo, func(key []byte) error {
		mac := hmac.New(sha256.New, key)
		if _, writeErr := mac.Write(caseCursorMessage(keyNo, payloadSegment)); writeErr != nil {
			return gerror.Wrap(writeErr, "sign enterprise verification cursor")
		}
		signature = mac.Sum(nil)
		return nil
	})
	if err != nil {
		return "", err
	}
	token := strings.Join([]string{
		enterpriseCaseCursorPrefix, keyNo, payloadSegment,
		base64.RawURLEncoding.EncodeToString(signature),
	}, ".")
	clearBytes(signature)
	if len(token) > maximumCursorBytes {
		return "", gerror.New("enterprise verification cursor is unavailable")
	}
	return token, nil
}

// decodeCaseCursor authenticates canonical cursor bytes and exact filter binding.
func (s *managementServiceImpl) decodeCaseCursor(
	ctx context.Context,
	input CaseListInput,
) (*enterpriseCaseCursor, error) {
	if input.Cursor == "" {
		return nil, nil
	}
	if len(input.Cursor) > maximumCursorBytes || input.Cursor != strings.TrimSpace(input.Cursor) {
		return nil, gerror.New("enterprise verification cursor is invalid")
	}
	parts := strings.Split(input.Cursor, ".")
	if len(parts) != 4 || parts[0] != enterpriseCaseCursorPrefix ||
		!validBusinessNo(parts[1], 64) || parts[2] == "" || parts[3] == "" {
		return nil, gerror.New("enterprise verification cursor is invalid")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(payloadBytes) > maximumCursorBytes ||
		base64.RawURLEncoding.EncodeToString(payloadBytes) != parts[2] {
		return nil, gerror.New("enterprise verification cursor is invalid")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil || len(signature) != sha256.Size ||
		base64.RawURLEncoding.EncodeToString(signature) != parts[3] {
		return nil, gerror.New("enterprise verification cursor is invalid")
	}
	defer clearBytes(signature)
	var verified bool
	err = accessEnterpriseKey(ctx, s.keyAccessor, parts[1], func(key []byte) error {
		mac := hmac.New(sha256.New, key)
		if _, writeErr := mac.Write(caseCursorMessage(parts[1], parts[2])); writeErr != nil {
			return gerror.Wrap(writeErr, "verify enterprise verification cursor")
		}
		verified = hmac.Equal(mac.Sum(nil), signature)
		return nil
	})
	if err != nil || !verified {
		return nil, gerror.New("enterprise verification cursor is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(payloadBytes))
	decoder.DisallowUnknownFields()
	var cursor enterpriseCaseCursor
	if err = decoder.Decode(&cursor); err != nil || !decoderEOF(decoder) {
		return nil, gerror.New("enterprise verification cursor is invalid")
	}
	canonical, err := json.Marshal(cursor)
	if err != nil || !bytes.Equal(canonical, payloadBytes) || cursor.Version != 1 ||
		cursor.StatusCode != input.StatusCode || cursor.CustomerNo != input.CustomerNo ||
		cursor.SubmittedAt <= 0 || !validBusinessNo(cursor.CaseNo, 64) {
		return nil, gerror.New("enterprise verification cursor is invalid")
	}
	return &cursor, nil
}

// caseCursorMessage creates the domain-separated HMAC input.
func caseCursorMessage(keyNo, payload string) []byte {
	return []byte("idc/enterprise-case-cursor/v1\x00" + keyNo + "\x00" + payload)
}

// decoderEOF rejects trailing JSON tokens after the signed cursor object.
func decoderEOF(decoder *json.Decoder) bool {
	var trailing any
	return decoder.Decode(&trailing) == io.EOF
}

// cursorSubmittedAt restores the exact database keyset timestamp.
func cursorSubmittedAt(cursor *enterpriseCaseCursor) time.Time {
	if cursor == nil {
		return time.Time{}
	}
	return time.UnixMicro(cursor.SubmittedAt).UTC()
}

// managementTarget builds one fixed-length, unambiguous authorization target.
func managementTarget(caseNo string, materialNo string) string {
	if materialNo == "" {
		return caseNo
	}
	return "enterprise-material:" + sha256Hex([]byte(caseNo+"\x00"+materialNo))
}

// actorPrincipalNo converts the trusted staff identifier into an opaque audit principal.
func actorPrincipalNo(actorUserID int) string {
	return "staff:" + strconv.Itoa(actorUserID)
}
