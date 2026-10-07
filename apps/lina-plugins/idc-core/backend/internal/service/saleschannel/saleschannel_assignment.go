// This file contains the small, deterministic rules for sales ownership changes.
package saleschannel

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/model/entity"
)

type AssignmentBatchItem struct {
	CustomerNo string
	Current    *entity.SalesCustomerAssignment
}

type AssignmentBatchRequest struct {
	Now                time.Time
	SourceMembershipNo string
	TargetMembershipNo string
	Force              bool
	Items              []AssignmentBatchItem
}

type AssignmentBatchDecision struct {
	Allowed    bool
	Processed  int
	Skipped    int
	Items      []AssignmentDecision
	ReasonCode string
}

const (
	AssignmentBatchMaximum = 500
	AssignmentBatchInvalid = "sales_assignment_batch_invalid"
)

// PlanAssignmentBatch evaluates all rows using the same protection rule as a
// single transfer. It never partially mutates assignments; the caller writes
// successful rows and their append-only transfer evidence in one transaction.
func (s *serviceImpl) PlanAssignmentBatch(ctx context.Context, request AssignmentBatchRequest) (AssignmentBatchDecision, error) {
	if ctx == nil {
		return AssignmentBatchDecision{}, gerror.New("sales assignment batch context is required")
	}
	if err := ctx.Err(); err != nil {
		return AssignmentBatchDecision{}, gerror.Wrap(err, "sales assignment batch context is unavailable")
	}
	if request.Now.IsZero() || request.SourceMembershipNo == "" || request.TargetMembershipNo == "" ||
		request.SourceMembershipNo == request.TargetMembershipNo || len(request.Items) == 0 || len(request.Items) > AssignmentBatchMaximum {
		return AssignmentBatchDecision{ReasonCode: AssignmentBatchInvalid}, nil
	}
	seen := make(map[string]struct{}, len(request.Items))
	result := AssignmentBatchDecision{Allowed: true, Items: make([]AssignmentDecision, 0, len(request.Items))}
	for _, item := range request.Items {
		if item.CustomerNo == "" {
			return AssignmentBatchDecision{ReasonCode: AssignmentBatchInvalid}, nil
		}
		if _, exists := seen[item.CustomerNo]; exists {
			return AssignmentBatchDecision{ReasonCode: AssignmentBatchInvalid}, nil
		}
		seen[item.CustomerNo] = struct{}{}
		if item.Current != nil && item.Current.MembershipNo != request.SourceMembershipNo {
			result.Skipped++
			result.Items = append(result.Items, AssignmentDecision{StatusCode: AssignmentProtected, ReasonCode: fmt.Sprintf("%s:source_mismatch", AssignmentProtected)})
			continue
		}
		decision, err := s.Assign(ctx, AssignmentRequest{Now: request.Now, Current: item.Current, NewMembershipNo: request.TargetMembershipNo, Force: request.Force})
		if err != nil {
			return AssignmentBatchDecision{}, err
		}
		if decision.StatusCode == AssignmentProtected {
			result.Skipped++
		} else {
			result.Processed++
		}
		result.Items = append(result.Items, decision)
	}
	return result, nil
}

const (
	// AssignmentActive marks the current owner assignment.
	AssignmentActive = "active"
	// AssignmentEnded marks a historical assignment.
	AssignmentEnded = "ended"
	// AssignmentProtected explains why a transfer is blocked.
	AssignmentProtected = "sales_assignment_protected"
	// AssignmentAllowed means a new assignment may be created.
	AssignmentAllowed = "allowed"
)

// AssignmentRequest contains the current assignment and the requested transfer
// target. The old row is never edited; the caller appends a new row and closes
// the old one in the same transaction.
type AssignmentRequest struct {
	Now             time.Time
	Current         *entity.SalesCustomerAssignment
	NewMembershipNo string
	Force           bool
}

// AssignmentDecision states whether the transfer can proceed.
type AssignmentDecision struct {
	StatusCode      string
	ReasonCode      string
	OldAssignmentNo string
	NewMembershipNo string
}

// Assign validates protection-period and input rules for one customer transfer.
func (s *serviceImpl) Assign(ctx context.Context, request AssignmentRequest) (AssignmentDecision, error) {
	if ctx == nil {
		return AssignmentDecision{}, gerror.New("sales assignment context is required")
	}
	if err := ctx.Err(); err != nil {
		return AssignmentDecision{}, gerror.Wrap(err, "sales assignment context is unavailable")
	}
	if request.Now.IsZero() || request.NewMembershipNo == "" {
		return AssignmentDecision{}, gerror.New("sales assignment request is invalid")
	}
	decision := AssignmentDecision{StatusCode: AssignmentAllowed, NewMembershipNo: request.NewMembershipNo}
	if request.Current == nil {
		return decision, nil
	}
	decision.OldAssignmentNo = request.Current.AssignmentNo
	if request.Current.StatusCode != AssignmentActive {
		return decision, nil
	}
	if request.Current.MembershipNo == request.NewMembershipNo {
		return AssignmentDecision{}, gerror.New("sales assignment target is unchanged")
	}
	if !request.Force && request.Current.ProtectionExpiresAt != nil && request.Now.Before(*request.Current.ProtectionExpiresAt) {
		decision.StatusCode = AssignmentProtected
		decision.ReasonCode = AssignmentProtected
	}
	return decision, nil
}
