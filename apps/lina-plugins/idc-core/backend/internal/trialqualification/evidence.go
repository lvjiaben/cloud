// Package trialqualification owns canonical, secret-free digest material shared
// by commerce trial holds and billing trial settlement verification.
package trialqualification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

// EntitlementFact identifies one trial right bound to one immutable product
// quantity unit. The unit sequence has no product-type meaning.
type EntitlementFact struct {
	EntitlementNo, ScopeNo, CustomerNo, ProductNo, VersionNo, BillingRuleNo string
	OrderNo, InvoiceNo, OrderItemNo, CurrencyCode                           string
	UnitSequence                                                            int
	TrialLimitQuantity                                                      int
	HoldExpiresAt                                                           time.Time
}

// MovementEvidenceFact binds a qualification movement to its exact rights.
type MovementEvidenceFact struct {
	MovementType, OrderNo, ScopeNo string
	Entitlements                   []string
	IdentityEvidenceDigest         string
}

// MovementFact seals one append-only projection transition.
type MovementFact struct {
	MovementNo, ScopeNo, CustomerNo, ProductNo, OrderNo, MovementType string
	Quantity, HeldBefore, HeldAfter, ConsumedBefore, ConsumedAfter    int
	VersionBefore, VersionAfter                                       int64
	EvidenceDigest                                                    string
	CreatedAt                                                         time.Time
}

// SettlementEvidenceFact binds a trial financial source to all held and
// consumed qualification evidence without exposing identity material.
type SettlementEvidenceFact struct {
	OrderNo, InvoiceNo, CustomerNo, IdentityEvidenceDigest string
	Entitlements                                           []string
	HoldMovements                                          []string
	ConsumeMovements                                       []string
}

// Digest returns the canonical SHA-256 digest of one fact.
func Digest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// EntitlementDigest normalizes timestamps before sealing a server right.
func EntitlementDigest(fact EntitlementFact) (string, error) {
	fact.HoldExpiresAt = fact.HoldExpiresAt.UTC()
	return Digest(fact)
}

// MovementEvidenceDigest seals the ordered entitlement digest set.
func MovementEvidenceDigest(fact MovementEvidenceFact) (string, error) {
	fact.Entitlements = append([]string(nil), fact.Entitlements...)
	sort.Strings(fact.Entitlements)
	return Digest(fact)
}

// MovementDigest normalizes timestamps before sealing a projection movement.
func MovementDigest(fact MovementFact) (string, error) {
	fact.CreatedAt = fact.CreatedAt.UTC()
	return Digest(fact)
}

// SettlementEvidenceDigest seals all ordered trial evidence owned by billing.
func SettlementEvidenceDigest(fact SettlementEvidenceFact) (string, error) {
	fact.Entitlements = append([]string(nil), fact.Entitlements...)
	fact.HoldMovements = append([]string(nil), fact.HoldMovements...)
	fact.ConsumeMovements = append([]string(nil), fact.ConsumeMovements...)
	sort.Strings(fact.Entitlements)
	sort.Strings(fact.HoldMovements)
	sort.Strings(fact.ConsumeMovements)
	return Digest(fact)
}
