// Package entitlementcreatedcap defines the versioned projection notification
// emitted after a generic long-lived entitlement is committed.
package entitlementcreatedcap

import (
	"context"
	"regexp"
	"strings"
)

const ContractVersion = "v2"

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var referencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:@-]{0,127}$`)

type Event struct {
	Version                 string
	EventNo                 string
	EntitlementNo           string
	IntentNo                string
	CustomerNo              string
	ProductNo               string
	ProductVersionNo        string
	ProductNameZhCN         string
	ProductNameEnUS         string
	ConfigurationSnapshotNo string
	InterfaceInstanceNo     string
	PluginID                string
	PluginReleaseVersion    string
	ContractVersion         string
	DeclarationDigest       string
	// OwnerID is the business owner frozen by the published fulfillment
	// contract. It is deliberately distinct from the replaceable provider
	// connector PluginID.
	OwnerID string
}

type Receipt struct {
	Version        string
	EventNo        string
	OwnerReference string
	EvidenceDigest string
}

type Sink interface {
	Deliver(context.Context, Event) (*Receipt, error)
}

// Router resolves one frozen business owner. Implementations must not guess
// from PluginID or fall back to a default sink.
type Router interface {
	Resolve(string) (Sink, bool)
}

func ValidateEvent(event Event) bool {
	return event.Version == ContractVersion && valid(event.EventNo, 40) &&
		valid(event.EntitlementNo, 40) && valid(event.IntentNo, 40) && valid(event.CustomerNo, 40) &&
		valid(event.ProductNo, 40) && valid(event.ProductVersionNo, 40) &&
		validLabel(event.ProductNameZhCN) && validLabel(event.ProductNameEnUS) &&
		valid(event.ConfigurationSnapshotNo, 40) && valid(event.InterfaceInstanceNo, 40) &&
		valid(event.PluginID, 64) && valid(event.PluginReleaseVersion, 32) &&
		valid(event.ContractVersion, 32) && digestPattern.MatchString(event.DeclarationDigest) &&
		valid(event.OwnerID, 64)
}

func validLabel(value string) bool {
	return value == strings.TrimSpace(value) && len(value) >= 1 && len(value) <= 256
}

func ValidateReceipt(event Event, receipt Receipt) bool {
	return ValidateEvent(event) && receipt.Version == ContractVersion && receipt.EventNo == event.EventNo &&
		valid(receipt.OwnerReference, 128) && digestPattern.MatchString(receipt.EvidenceDigest)
}

func valid(value string, maximum int) bool {
	return value == strings.TrimSpace(value) && len(value) <= maximum && referencePattern.MatchString(value)
}
