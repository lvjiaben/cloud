// This file validates and canonicalizes bounded batch tag mutation inputs.

package customermanagement

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

const (
	maximumTagBatchCustomers        = 100
	maximumTagBatchTags             = 20
	maximumTagBatchReasonRunes      = 256
	maximumTagBatchIdempotencyRunes = 128
)

type normalizedTagBatch struct {
	action         TagBatchAction
	customerNos    []string
	tagNos         []string
	reason         string
	idempotencyKey string
	targetNo       string
	actorUserID    int
}

func normalizeTagBatch(input TagBatchInput) (normalizedTagBatch, error) {
	if input.Action != TagBatchAssign && input.Action != TagBatchRevoke {
		return normalizedTagBatch{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	customers, valid := canonicalUniqueReferences(
		input.CustomerNos,
		maximumTagBatchCustomers,
		validPersistedCustomerNo,
	)
	if !valid {
		return normalizedTagBatch{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	tags, valid := canonicalUniqueReferences(input.TagNos, maximumTagBatchTags, func(value string) bool {
		return value != "" && validOptionalSegmentNo(value, "TAG_")
	})
	if !valid {
		return normalizedTagBatch{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	reason := strings.Join(strings.Fields(input.Reason), " ")
	if reason == "" || !utf8.ValidString(reason) ||
		utf8.RuneCountInString(reason) > maximumTagBatchReasonRunes ||
		!validTagBatchIdempotencyKey(input.IdempotencyKey) {
		return normalizedTagBatch{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	digest := sha256.Sum256([]byte(string(input.Action) + "\x00" + strings.Join(tags, "\x00")))
	return normalizedTagBatch{
		action: input.Action, customerNos: customers, tagNos: tags, reason: reason,
		idempotencyKey: input.IdempotencyKey,
		targetNo:       "TGB_" + hex.EncodeToString(digest[:16]),
	}, nil
}

func canonicalUniqueReferences(
	values []string,
	maximum int,
	validate func(string) bool,
) ([]string, bool) {
	if len(values) == 0 || len(values) > maximum || validate == nil {
		return nil, false
	}
	normalized := append([]string(nil), values...)
	sort.Strings(normalized)
	for index, value := range normalized {
		if !validate(value) || (index > 0 && value == normalized[index-1]) {
			return nil, false
		}
	}
	return normalized, true
}

func validTagBatchIdempotencyKey(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) ||
		utf8.RuneCountInString(value) < 8 ||
		utf8.RuneCountInString(value) > maximumTagBatchIdempotencyRunes {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') &&
			character != '.' && character != '_' && character != ':' && character != '-' {
			return false
		}
	}
	return true
}
