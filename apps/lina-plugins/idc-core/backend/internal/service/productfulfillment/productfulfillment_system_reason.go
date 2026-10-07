// This file owns the immutable bilingual reasons frozen by automatic
// product-fulfillment state transitions.
package productfulfillment

import (
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	SystemReasonOrderCancelled                = "order_cancelled"
	SystemReasonCancellationRequiresAttention = "cancellation_requires_attention"
	SystemReasonNewBusinessStopped            = "new_business_stopped"
)

var requiredSystemTransitionReasonCodes = [...]string{
	SystemReasonOrderCancelled,
	SystemReasonCancellationRequiresAttention,
	SystemReasonNewBusinessStopped,
}

// SystemTransitionReason is the bilingual text frozen into owner-private
// history when an automatic system transition occurs.
type SystemTransitionReason struct {
	LabelZhCN string
	LabelEnUS string
}

// SystemTransitionReasonCatalog resolves immutable bilingual system reasons.
// Implementations must fail closed for missing or unknown reason codes.
type SystemTransitionReasonCatalog interface {
	ResolveSystemTransitionReason(string) (SystemTransitionReason, error)
}

type systemTransitionReasonCatalog struct {
	reasons map[string]SystemTransitionReason
}

// RequiredSystemTransitionReasonCodes returns a copy of the complete resource
// contract used by the plugin startup loader.
func RequiredSystemTransitionReasonCodes() []string {
	result := make([]string, len(requiredSystemTransitionReasonCodes))
	copy(result, requiredSystemTransitionReasonCodes[:])
	return result
}

// NewSystemTransitionReasonCatalog validates and freezes the complete
// bilingual resource set. Extra and missing entries are rejected so startup
// cannot silently publish an incomplete history contract.
func NewSystemTransitionReasonCatalog(input map[string]SystemTransitionReason) (SystemTransitionReasonCatalog, error) {
	if len(input) != len(requiredSystemTransitionReasonCodes) {
		return nil, errors.New("product fulfillment system reason resources are incomplete")
	}
	reasons := make(map[string]SystemTransitionReason, len(input))
	for _, code := range requiredSystemTransitionReasonCodes {
		reason, exists := input[code]
		if !exists || !validSystemTransitionReason(reason) {
			return nil, errors.New("product fulfillment system reason resource is invalid")
		}
		reasons[code] = SystemTransitionReason{
			LabelZhCN: strings.TrimSpace(reason.LabelZhCN),
			LabelEnUS: strings.TrimSpace(reason.LabelEnUS),
		}
	}
	return &systemTransitionReasonCatalog{reasons: reasons}, nil
}

func (c *systemTransitionReasonCatalog) ResolveSystemTransitionReason(code string) (SystemTransitionReason, error) {
	if c == nil {
		return SystemTransitionReason{}, errors.New("product fulfillment system reason catalog is unavailable")
	}
	reason, exists := c.reasons[code]
	if !exists || !validSystemTransitionReason(reason) {
		return SystemTransitionReason{}, errors.New("product fulfillment system reason is unavailable")
	}
	return reason, nil
}

func validSystemTransitionReason(reason SystemTransitionReason) bool {
	return validSystemTransitionReasonLabel(reason.LabelZhCN) && validSystemTransitionReasonLabel(reason.LabelEnUS)
}

func validSystemTransitionReasonLabel(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > 128 {
		return false
	}
	for _, current := range trimmed {
		if current < 0x20 || current == 0x7f {
			return false
		}
	}
	return true
}
