// Package securitycodecap defines the source-only, caller-bound security-code contract.
// Raw destinations are accepted only by the source implementation and are never
// exposed through a dynamic-plugin bridge.
package securitycodecap

import (
	"context"
	"time"
)

const (
	OwnerPluginID   = "idc-notify"
	ContractVersion = "v1"
)

type Binding struct {
	Purpose      string
	SubjectNo    string
	BrowserNonce []byte
	Destination  []byte
	Locale       string
}

type HumanChallengeInput struct {
	Binding
	PresentationKind string
}

type HumanChallenge struct {
	ChallengeNo      string
	ProviderCode     string
	ProviderAction   string
	Kind             string
	ImageDataURL     string
	AccessiblePrompt string
	SiteKey          string
	ExpiresAt        time.Time
}

type HumanCompletionInput struct {
	Binding
	ChallengeNo   string
	Answer        []byte
	ProviderToken []byte
}

type HumanProof struct {
	ProofToken string
	ExpiresAt  time.Time
}

type SendInput struct {
	Binding
	ProofToken     string
	IdempotencyKey string
	ClientIP       []byte
}

type Verification struct {
	VerificationNo    string
	MaskedDestination string
	DeliveryState     string
	ExpiresAt         time.Time
	RetryAfterAt      time.Time
}

type VerifyInput struct {
	Binding
	VerificationNo string
	Code           []byte
}

type Verified struct {
	VerificationNo string
	Version        int64
	VerifiedAt     time.Time
}

type Service interface {
	CreateHumanChallenge(context.Context, HumanChallengeInput) (*HumanChallenge, error)
	CompleteHumanChallenge(context.Context, HumanCompletionInput) (*HumanProof, error)
	Send(context.Context, SendInput) (*Verification, error)
	Verify(context.Context, VerifyInput) (*Verified, error)
}
