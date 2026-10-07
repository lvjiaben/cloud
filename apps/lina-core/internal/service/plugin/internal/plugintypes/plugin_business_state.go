// This file defines the host-owned plugin new-business admission state.

package plugintypes

// NewBusinessState separates new sales and bindings from runtime enablement.
type NewBusinessState string

// Supported new-business states.
const (
	NewBusinessStateAccepting NewBusinessState = "accepting"
	NewBusinessStateStopped   NewBusinessState = "stopped"
)

// String returns the persisted state value.
func (value NewBusinessState) String() string { return string(value) }

// NormalizeNewBusinessState returns the safe persisted state. Empty values on
// pre-migration projections mean accepting; unknown values fail closed.
func NormalizeNewBusinessState(value string) NewBusinessState {
	switch NewBusinessState(value) {
	case "", NewBusinessStateAccepting:
		return NewBusinessStateAccepting
	case NewBusinessStateStopped:
		return NewBusinessStateStopped
	default:
		return NewBusinessStateStopped
	}
}
