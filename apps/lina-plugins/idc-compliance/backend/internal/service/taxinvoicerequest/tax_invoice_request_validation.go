// This file normalizes bounded customer input and derives stable idempotency identities.

package taxinvoicerequest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

type normalizedInput struct {
	ProfileNo      string
	Selections     []taxinvoicecap.Selection
	IdempotencyKey string
}

type requestFingerprint struct {
	CustomerNo       string                    `json:"customerNo"`
	IdempotencyScope string                    `json:"idempotencyScope"`
	ProfileNo        string                    `json:"profileNo"`
	Selections       []taxinvoicecap.Selection `json:"selections"`
}

// normalizeInput rejects ambiguous duplicates and sorts selections so retries
// remain byte-for-byte stable regardless of browser array ordering.
func normalizeInput(input CreateInput) (normalizedInput, error) {
	profileNo := strings.TrimSpace(input.ProfileNo)
	if !validBusinessNo(profileNo, 64) || !validIdempotencyKey(input.IdempotencyKey) ||
		len(input.Selections) == 0 || len(input.Selections) > taxinvoicecap.MaximumSelections {
		return normalizedInput{}, gerror.New("tax invoice request input is invalid")
	}
	selections := append([]taxinvoicecap.Selection(nil), input.Selections...)
	for index := range selections {
		selections[index].BillingInvoiceNo = strings.TrimSpace(selections[index].BillingInvoiceNo)
		if !validBusinessNo(selections[index].BillingInvoiceNo, 64) ||
			selections[index].RequestedGrossMinor <= 0 {
			clear(selections)
			return normalizedInput{}, gerror.New("tax invoice request selection is invalid")
		}
	}
	sort.Slice(selections, func(left, right int) bool {
		return selections[left].BillingInvoiceNo < selections[right].BillingInvoiceNo
	})
	for index := 1; index < len(selections); index++ {
		if selections[index-1].BillingInvoiceNo == selections[index].BillingInvoiceNo {
			clear(selections)
			return normalizedInput{}, gerror.New("tax invoice request selection is duplicated")
		}
	}
	return normalizedInput{
		ProfileNo: profileNo, Selections: selections, IdempotencyKey: input.IdempotencyKey,
	}, nil
}

// deriveRequestIdentity scopes the idempotency key to the authenticated
// customer and binds the exact normalized customer input to one request digest.
func deriveRequestIdentity(customerNo string, input normalizedInput) (string, string, error) {
	if !validBusinessNo(customerNo, 64) {
		return "", "", gerror.New("tax invoice request customer is invalid")
	}
	scopeDigest := digestText("idc/tax-invoice-idempotency/v1\x00", customerNo+"\x00"+input.IdempotencyKey)
	fingerprint := requestFingerprint{
		CustomerNo: customerNo, IdempotencyScope: scopeDigest,
		ProfileNo: input.ProfileNo, Selections: input.Selections,
	}
	payload, err := json.Marshal(fingerprint)
	if err != nil {
		return "", "", gerror.Wrap(err, "encode tax invoice request fingerprint")
	}
	defer clear(payload)
	requestDigest := digestBytes("idc/tax-invoice-request/v1\x00", payload)
	return "TINV_" + strings.ToUpper(scopeDigest[:32]), requestDigest, nil
}

func allocationNo(taxInvoiceNo, billingInvoiceNo string) string {
	digest := digestText("idc/tax-invoice-allocation/v1\x00", taxInvoiceNo+"\x00"+billingInvoiceNo)
	return "TIAL_" + strings.ToUpper(digest[:32])
}

func itemNo(taxInvoiceNo, billingInvoiceItemNo string) string {
	digest := digestText("idc/tax-invoice-item/v1\x00", taxInvoiceNo+"\x00"+billingInvoiceItemNo)
	return "TIIT_" + strings.ToUpper(digest[:32])
}

func digestText(scope, value string) string {
	return digestBytes(scope, []byte(value))
}

func digestBytes(scope string, value []byte) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(scope))
	_, _ = hash.Write(value)
	return hex.EncodeToString(hash.Sum(nil))
}

func validBusinessNo(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum {
		return false
	}
	for _, character := range []byte(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validIdempotencyKey(value string) bool {
	if len(value) < 16 || len(value) > 128 || value != strings.TrimSpace(value) || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
