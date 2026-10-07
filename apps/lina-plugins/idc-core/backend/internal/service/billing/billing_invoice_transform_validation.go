// This file canonicalizes bounded invoice merge and whole-item split commands
// before database access. Raw idempotency keys never leave process memory.

package billing

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

const (
	maximumInvoiceTransformInvoiceCount = 100
	maximumInvoiceTransformItemCount    = 10000
)

type invoiceTransformCommand struct {
	Type              InvoiceTransformType
	SourceInvoiceNos  []string
	DestinationGroups [][]string
	IdempotencyDigest string
	RequestDigest     string
	RequestedBy       int
}

func normalizeInvoiceMerge(input InvoiceMergeInput) (invoiceTransformCommand, error) {
	if input.RequestedBy <= 0 || !validInvoiceTransformKey(input.IdempotencyKey) ||
		len(input.InvoiceNos) < 2 || len(input.InvoiceNos) > maximumInvoiceTransformInvoiceCount {
		return invoiceTransformCommand{}, invoiceInvalidError()
	}
	sources, ok := canonicalBusinessReferences(input.InvoiceNos)
	if !ok {
		return invoiceTransformCommand{}, invoiceInvalidError()
	}
	command := invoiceTransformCommand{
		Type: InvoiceTransformMerge, SourceInvoiceNos: sources,
		IdempotencyDigest: invoiceTransformIdempotencyDigest(input.IdempotencyKey),
		RequestedBy:       input.RequestedBy,
	}
	digest, err := invoiceTransformRequestDigest(command)
	if err != nil {
		return invoiceTransformCommand{}, err
	}
	command.RequestDigest = digest
	return command, nil
}

func normalizeInvoiceSplit(input InvoiceSplitInput) (invoiceTransformCommand, error) {
	if input.RequestedBy <= 0 || !validInvoiceTransformKey(input.IdempotencyKey) ||
		!validBusinessReference(input.InvoiceNo, 40) || input.InvoiceNo != strings.TrimSpace(input.InvoiceNo) ||
		len(input.Groups) < 2 || len(input.Groups) > maximumInvoiceTransformInvoiceCount {
		return invoiceTransformCommand{}, invoiceInvalidError()
	}
	groups := make([][]string, 0, len(input.Groups))
	seen := make(map[string]struct{})
	itemCount := 0
	for _, group := range input.Groups {
		if len(group.InvoiceItemNos) == 0 ||
			itemCount > maximumInvoiceTransformItemCount-len(group.InvoiceItemNos) {
			return invoiceTransformCommand{}, invoiceInvalidError()
		}
		canonical, ok := canonicalBusinessReferences(group.InvoiceItemNos)
		if !ok {
			return invoiceTransformCommand{}, invoiceInvalidError()
		}
		for _, itemNo := range canonical {
			if _, duplicate := seen[itemNo]; duplicate {
				return invoiceTransformCommand{}, invoiceInvalidError()
			}
			seen[itemNo] = struct{}{}
		}
		itemCount += len(canonical)
		groups = append(groups, canonical)
	}
	sort.Slice(groups, func(left int, right int) bool {
		return strings.Join(groups[left], "\x1f") < strings.Join(groups[right], "\x1f")
	})
	command := invoiceTransformCommand{
		Type: InvoiceTransformSplit, SourceInvoiceNos: []string{input.InvoiceNo},
		DestinationGroups: groups,
		IdempotencyDigest: invoiceTransformIdempotencyDigest(input.IdempotencyKey),
		RequestedBy:       input.RequestedBy,
	}
	digest, err := invoiceTransformRequestDigest(command)
	if err != nil {
		return invoiceTransformCommand{}, err
	}
	command.RequestDigest = digest
	return command, nil
}

func canonicalBusinessReferences(values []string) ([]string, bool) {
	canonical := append([]string(nil), values...)
	for _, value := range canonical {
		if value != strings.TrimSpace(value) || !validBusinessReference(value, 40) {
			return nil, false
		}
	}
	sort.Strings(canonical)
	for index := 1; index < len(canonical); index++ {
		if canonical[index] == canonical[index-1] {
			return nil, false
		}
	}
	return canonical, true
}

func validInvoiceTransformKey(value string) bool {
	return len(value) >= 16 && value == strings.TrimSpace(value) &&
		validBusinessReference(value, 128)
}

func invoiceTransformIdempotencyDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func invoiceTransformRequestDigest(command invoiceTransformCommand) (string, error) {
	return contentDigest(struct {
		Type              InvoiceTransformType `json:"type"`
		SourceInvoiceNos  []string             `json:"sourceInvoiceNos"`
		DestinationGroups [][]string           `json:"destinationGroups,omitempty"`
		RequestedBy       int                  `json:"requestedBy"`
	}{command.Type, command.SourceInvoiceNos, command.DestinationGroups, command.RequestedBy})
}
