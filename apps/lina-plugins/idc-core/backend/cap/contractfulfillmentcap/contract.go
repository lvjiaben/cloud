// Package contractfulfillmentcap defines the narrow fulfillment gate owned by
// the compliance plugin and consumed by the commerce fulfillment coordinator.
package contractfulfillmentcap

import (
	"context"
	"regexp"

	"github.com/gogf/gf/v2/errors/gerror"
)

const MaximumBatchSize = 100

var orderNoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,39}$`)

type ReasonCode string

const (
	ReasonNotRequired ReasonCode = "not_required"
	ReasonCompleted   ReasonCode = "completed"
	ReasonNotReady    ReasonCode = "not_ready"
)

// Requirement carries only the immutable order fact needed by the compliance
// owner. It intentionally contains no product, payment, or inventory details.
type Requirement struct {
	OrderNo          string
	RequiresContract bool
}

// Decision is the bounded result visible to the consumer. Contract numbers,
// supplier states, and signing evidence remain private to the compliance owner.
type Decision struct {
	OrderNo    string
	Ready      bool
	ReasonCode ReasonCode
}

type Source interface {
	BatchEvaluate(context.Context, []Requirement) ([]Decision, error)
}

func ValidateRequirements(values []Requirement) error {
	if len(values) == 0 || len(values) > MaximumBatchSize {
		return gerror.New("contract fulfillment requirement batch is invalid")
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !orderNoPattern.MatchString(value.OrderNo) {
			return gerror.New("contract fulfillment order number is invalid")
		}
		if _, duplicate := seen[value.OrderNo]; duplicate {
			return gerror.New("contract fulfillment requirement batch contains a duplicate")
		}
		seen[value.OrderNo] = struct{}{}
	}
	return nil
}

func ValidateDecisions(requirements []Requirement, decisions []Decision) error {
	if ValidateRequirements(requirements) != nil || len(decisions) != len(requirements) {
		return gerror.New("contract fulfillment decision batch is invalid")
	}
	required := make(map[string]bool, len(requirements))
	for _, value := range requirements {
		required[value.OrderNo] = value.RequiresContract
	}
	seen := make(map[string]struct{}, len(decisions))
	for _, value := range decisions {
		requiresContract, requested := required[value.OrderNo]
		if !requested {
			return gerror.New("contract fulfillment decision contains an unknown order")
		}
		if _, duplicate := seen[value.OrderNo]; duplicate {
			return gerror.New("contract fulfillment decision batch contains a duplicate")
		}
		seen[value.OrderNo] = struct{}{}
		valid := (!requiresContract && value.Ready && value.ReasonCode == ReasonNotRequired) ||
			(requiresContract && value.Ready && value.ReasonCode == ReasonCompleted) ||
			(requiresContract && !value.Ready && value.ReasonCode == ReasonNotReady)
		if !valid {
			return gerror.New("contract fulfillment decision is inconsistent")
		}
	}
	return nil
}
