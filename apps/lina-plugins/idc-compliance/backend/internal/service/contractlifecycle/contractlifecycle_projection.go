// This file maps Tencent flow states to monotonic local contract and flow states.

package contractlifecycle

import (
	"regexp"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

var (
	deliveryNoPattern = regexp.MustCompile(`^ldv_[0-9a-f]{32}$`)
	routeNoPattern    = regexp.MustCompile(`^lwr_[0-9a-f]{32}$`)
)

type lockedFlow struct {
	ContractNo          string    `orm:"contract_no"`
	OrderNo             string    `orm:"order_no"`
	CustomerNo          string    `orm:"customer_no"`
	ContractStatus      string    `orm:"contract_status"`
	FlowNo              string    `orm:"flow_no"`
	PurposeCode         string    `orm:"purpose_code"`
	FlowStatus          string    `orm:"flow_status"`
	ProviderStatus      string    `orm:"provider_status"`
	GatewayRouteVersion int64     `orm:"gateway_route_version"`
	DatabaseNow         time.Time `orm:"database_now"`
}

type projection struct {
	FlowStatus     string
	ContractStatus string
	ReasonCode     string
}

func project(providerStatus string, purposeCode string) (projection, error) {
	switch providerStatus {
	case "INIT":
		return projection{FlowStatus: "signing", ContractStatus: signingContractStatus(purposeCode), ReasonCode: "provider_flow_initialized"}, nil
	case "PART":
		return projection{FlowStatus: "signing", ContractStatus: signingContractStatus(purposeCode), ReasonCode: "provider_partially_signed"}, nil
	case "WILLEXPIRE":
		return projection{FlowStatus: "signing", ContractStatus: signingContractStatus(purposeCode), ReasonCode: "provider_signing_will_expire"}, nil
	case "ALL":
		return projection{FlowStatus: "completed", ContractStatus: completedContractStatus(purposeCode), ReasonCode: "provider_all_signed"}, nil
	case "REJECT":
		return projection{FlowStatus: "rejected", ContractStatus: failedContractStatus(purposeCode, "rejected"), ReasonCode: "provider_signer_rejected"}, nil
	case "DEADLINE":
		return projection{FlowStatus: "expired", ContractStatus: failedContractStatus(purposeCode, "expired"), ReasonCode: "provider_signing_deadline_reached"}, nil
	case "CANCEL":
		return projection{FlowStatus: "revoked", ContractStatus: failedContractStatus(purposeCode, "revoked"), ReasonCode: "provider_flow_cancelled"}, nil
	case "INVALID":
		return projection{FlowStatus: "failed", ContractStatus: failedContractStatus(purposeCode, "failed"), ReasonCode: "provider_flow_invalidated"}, nil
	case "EXCEPTION":
		return projection{FlowStatus: "failed", ContractStatus: failedContractStatus(purposeCode, "failed"), ReasonCode: "provider_flow_exception"}, nil
	case "RELIEVED":
		return projection{FlowStatus: "completed", ContractStatus: "terminated", ReasonCode: "provider_contract_relieved"}, nil
	default:
		return projection{}, gerror.New("Tencent contract provider status is invalid")
	}
}

func decide(locked lockedFlow, observation Observation, projected projection) string {
	if observation.RouteVersion <= locked.GatewayRouteVersion {
		return "stale"
	}
	if observation.ProviderStatus == "RELIEVED" && locked.PurposeCode != "termination" {
		return "conflict"
	}
	if terminalFlowStatus(locked.FlowStatus) {
		if locked.FlowStatus == projected.FlowStatus {
			return "stale"
		}
		return "conflict"
	}
	allowedTermination := locked.PurposeCode == "termination" && locked.ContractStatus == "completed" &&
		projected.ContractStatus == "terminated"
	if terminalContractStatus(locked.ContractStatus) && projected.ContractStatus != "" &&
		projected.ContractStatus != locked.ContractStatus && !allowedTermination {
		return "conflict"
	}
	return "applied"
}

func signingContractStatus(purposeCode string) string {
	if purposeCode == "termination" {
		return ""
	}
	return "signing"
}

func completedContractStatus(purposeCode string) string {
	if purposeCode == "termination" {
		return "terminated"
	}
	return "completed"
}

func failedContractStatus(purposeCode string, status string) string {
	if purposeCode == "termination" {
		return ""
	}
	return status
}

func projectedTimelineStatus(projected projection) string {
	if projected.ContractStatus != "" {
		return projected.ContractStatus
	}
	return projected.FlowStatus
}

func terminalFlowStatus(status string) bool {
	switch status {
	case "completed", "rejected", "expired", "revoked", "failed":
		return true
	default:
		return false
	}
}

func terminalFailureStatus(status string) bool {
	return status == "rejected" || status == "expired" || status == "revoked" || status == "failed"
}

func terminalContractStatus(status string) bool {
	switch status {
	case "completed", "rejected", "expired", "revoked", "failed", "terminated":
		return true
	default:
		return false
	}
}

func validObservation(observation Observation) bool {
	if !routeNoPattern.MatchString(observation.RouteNo) || observation.RouteVersion < 2 ||
		(observation.DeliveryNo != "" && !deliveryNoPattern.MatchString(observation.DeliveryNo)) ||
		(observation.SourceKind != "callback" && observation.SourceKind != "active_query") ||
		(observation.GatewayDecisionCode != "applied" && observation.GatewayDecisionCode != "stale" && observation.GatewayDecisionCode != "conflict") ||
		observation.SupplierObservedAt.IsZero() || observation.ObservedAt.IsZero() {
		return false
	}
	_, err := project(observation.ProviderStatus, "issue")
	return err == nil
}

func nullableDelivery(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
