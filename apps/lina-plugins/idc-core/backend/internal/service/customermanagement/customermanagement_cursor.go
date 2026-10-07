// This file encodes and validates authoritative customer-page boundaries.

package customermanagement

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strconv"
	"time"
)

const (
	customerCursorVersion      = byte(1)
	customerCursorDigestSize   = 16
	customerCursorPayloadSize  = 1 + 8 + 8 + 36 + customerCursorDigestSize
	maximumEncodedCursorLength = 128
)

var errInvalidCursor = errors.New("invalid customer management cursor")

func encodeCustomerCursor(boundary cursorBoundary, input normalizedSearch) string {
	payload := make([]byte, customerCursorPayloadSize)
	payload[0] = customerCursorVersion
	binary.BigEndian.PutUint64(payload[1:9], uint64(boundary.CreatedAt.UnixMicro()))
	binary.BigEndian.PutUint64(payload[9:17], uint64(boundary.InternalID))
	copy(payload[17:53], boundary.CustomerNo)
	digest := customerCursorFilterDigest(input)
	copy(payload[53:], digest[:customerCursorDigestSize])
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeCustomerCursor(encoded string, input normalizedSearch) (*cursorBoundary, error) {
	if len(encoded) == 0 || len(encoded) > maximumEncodedCursorLength {
		return nil, errInvalidCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(payload) != customerCursorPayloadSize || payload[0] != customerCursorVersion {
		return nil, errInvalidCursor
	}
	microseconds := int64(binary.BigEndian.Uint64(payload[1:9]))
	internalID := int64(binary.BigEndian.Uint64(payload[9:17]))
	customerNo := string(payload[17:53])
	expected := customerCursorFilterDigest(input)
	if microseconds <= 0 || internalID <= 0 || !validPersistedCustomerNo(customerNo) ||
		subtle.ConstantTimeCompare(payload[53:], expected[:customerCursorDigestSize]) != 1 {
		return nil, errInvalidCursor
	}
	return &cursorBoundary{
		CreatedAt: time.UnixMicro(microseconds).UTC(), InternalID: internalID, CustomerNo: customerNo,
	}, nil
}

func customerCursorFilterDigest(input normalizedSearch) [sha256.Size]byte {
	return sha256.Sum256([]byte(input.customerCode + "\x00" + input.namePrefix + "\x00" + input.phone + "\x00" + string(input.typeCode) + "\x00" +
		string(input.status) + "\x00" + input.groupNo + "\x00" + input.tagNo + "\x00" +
		strconv.Itoa(input.limit)))
}
