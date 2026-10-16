// This file defines the same-system renewal envelope bound to frozen local
// entitlement and provider facts, without credentials or caller-selected owners.
package renewalownercap

const (
	// PeerOwnerID is the managed connector that implements this renewal protocol.
	PeerOwnerID = "idc-peer"
	// PeerRenewMethod submits one idempotent upstream renewal.
	PeerRenewMethod = "renew"
	// PeerRenewQueryMethod only reconciles that original request.
	PeerRenewQueryMethod = "renew.query"
)

// PeerRequest is built by the core from immutable local evidence. The guest
// must resolve ExternalReference unambiguously to the exact upstream service,
// verify the period and original expiry, and never pick the first of many.
type PeerRequest struct {
	Request           TaskRequest `json:"request"`
	ExternalReference string      `json:"externalReference"`
	UpstreamServiceNo string      `json:"upstreamServiceNo"`
	PurchaseCycles    int         `json:"purchaseCycles"`
	CurrencyCode      string      `json:"currencyCode"`
	PeriodUnitCode    string      `json:"periodUnitCode"`
	PeriodCount       int         `json:"periodCount"`
}

// ValidatePeerRequest fails closed before any network request is permitted.
func ValidatePeerRequest(input PeerRequest) error {
	if ValidateTaskRequest(input.Request) != nil || input.Request.OwnerID != PeerOwnerID || input.Request.PluginID != PeerOwnerID || input.Request.RenewalMethod != PeerRenewMethod || !validReference(input.ExternalReference, 128) || !validReference(input.UpstreamServiceNo, 40) || input.PurchaseCycles < 1 || input.PurchaseCycles > 120 || len(input.CurrencyCode) != 3 || input.PeriodCount < 1 {
		return ErrTaskConflict
	}
	switch input.PeriodUnitCode {
	case "hour", "day", "month", "year":
	default:
		return ErrTaskConflict
	}
	return nil
}
