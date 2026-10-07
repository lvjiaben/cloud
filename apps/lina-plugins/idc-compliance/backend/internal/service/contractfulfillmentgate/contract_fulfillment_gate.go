// Package contractfulfillmentgate publishes the compliance-owned authoritative
// contract decision used before fulfillment starts.
package contractfulfillmentgate

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/contractfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const completedStatus = "completed"

type contractRow struct {
	OrderNo                string `json:"order_no" orm:"order_no"`
	RequiredForFulfillment bool   `json:"required_for_fulfillment" orm:"required_for_fulfillment"`
	StatusCode             string `json:"status_code" orm:"status_code"`
}

type source struct {
	loadRows func(context.Context, []string) ([]contractRow, error)
}

var _ contractfulfillmentcap.Source = (*source)(nil)

func New(database gdb.DB) (contractfulfillmentcap.Source, error) {
	if database == nil {
		return nil, gerror.New("contract fulfillment gate requires database")
	}
	return &source{loadRows: func(ctx context.Context, orderNos []string) ([]contractRow, error) {
		var rows []contractRow
		err := database.Model("plugin_idc_compliance_contract").Ctx(ctx).
			Fields("order_no, required_for_fulfillment, status_code").
			Where("tenant_id", idccap.DefaultTenantID).
			WhereIn("order_no", orderNos).
			Scan(&rows)
		if err != nil {
			return nil, gerror.Wrap(err, "load authoritative contract fulfillment states")
		}
		return rows, nil
	}}, nil
}

func (s *source) BatchEvaluate(
	ctx context.Context,
	requirements []contractfulfillmentcap.Requirement,
) ([]contractfulfillmentcap.Decision, error) {
	if s == nil || s.loadRows == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("contract fulfillment gate is unavailable")
	}
	if err := contractfulfillmentcap.ValidateRequirements(requirements); err != nil {
		return nil, err
	}
	requiredOrderNos := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		if requirement.RequiresContract {
			requiredOrderNos = append(requiredOrderNos, requirement.OrderNo)
		}
	}
	rowsByOrder := make(map[string]contractRow, len(requiredOrderNos))
	if len(requiredOrderNos) > 0 {
		rows, err := s.loadRows(ctx, requiredOrderNos)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if _, duplicate := rowsByOrder[row.OrderNo]; duplicate {
				return nil, gerror.New("authoritative contract fulfillment state is ambiguous")
			}
			rowsByOrder[row.OrderNo] = row
		}
	}
	decisions := make([]contractfulfillmentcap.Decision, 0, len(requirements))
	for _, requirement := range requirements {
		decision := contractfulfillmentcap.Decision{
			OrderNo:    requirement.OrderNo,
			Ready:      true,
			ReasonCode: contractfulfillmentcap.ReasonNotRequired,
		}
		if requirement.RequiresContract {
			row, exists := rowsByOrder[requirement.OrderNo]
			decision.Ready = exists && row.RequiredForFulfillment && row.StatusCode == completedStatus
			decision.ReasonCode = contractfulfillmentcap.ReasonNotReady
			if decision.Ready {
				decision.ReasonCode = contractfulfillmentcap.ReasonCompleted
			}
		}
		decisions = append(decisions, decision)
	}
	if err := contractfulfillmentcap.ValidateDecisions(requirements, decisions); err != nil {
		return nil, gerror.Wrap(err, "validate contract fulfillment decisions")
	}
	return decisions, nil
}
