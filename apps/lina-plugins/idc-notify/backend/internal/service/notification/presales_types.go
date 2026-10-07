package notification

import (
	"time"

	"lina-plugin-idc-core/backend/cap/supportcap"
)

type PresalesBindingInput struct {
	BrowserNonce    []byte
	Destination     []byte
	DestinationKind string
	Locale          string
	Operation       string
	TicketNo        string
}

type PresalesHumanChallengeInput struct {
	PresalesBindingInput
	PresentationKind string
}

type PresalesHumanChallenge struct {
	ChallengeNo      string
	ProviderCode     string
	ProviderAction   string
	Kind             string
	ImageDataURL     string
	AccessiblePrompt string
	SiteKey          string
	ExpiresAt        time.Time
}

type PresalesHumanCompletionInput struct {
	PresalesBindingInput
	ChallengeNo   string
	Answer        []byte
	ProviderToken []byte
}

type PresalesHumanProof struct {
	ProofToken string
	ExpiresAt  time.Time
}

type PresalesVerificationInput struct {
	PresalesBindingInput
	ProofToken     string
	IdempotencyKey string
	ClientIP       []byte
}

type PresalesVerification struct {
	VerificationNo    string
	MaskedDestination string
	DeliveryState     string
	ExpiresAt         time.Time
	RetryAfterAt      time.Time
}

type PresalesTicketInput struct {
	BrowserNonce    []byte
	Destination     []byte
	DestinationKind string
	Locale          string
	OperationNo     string
	VerificationNo  string
	Code            []byte
	CategoryNo      string
	Subject         string
	Body            string
	CustomFields    []supportcap.CustomFieldValue
}

type PresalesRenewalInput struct {
	BrowserNonce    []byte
	Destination     []byte
	DestinationKind string
	Locale          string
	OperationNo     string
	VerificationNo  string
	Code            []byte
	TicketNo        string
}

type PresalesAccess struct {
	TicketNo          string
	AccessToken       string
	ExpiresAt         time.Time
	LinkDeliveryState string
}
