// Package challengecap defines reusable, caller-bound human-verification proofs.
package challengecap

import (
	"context"
	"time"
)

const (
	OwnerPluginID          = "shared-services"
	ServiceName            = "challenge"
	SourceContractPackage  = "lina-plugin-shared-services/backend/cap/challengecap"
	DynamicContractPackage = SourceContractPackage + "/bridge"
	ContractVersion        = "v1"

	MaximumPayloadBytes = 32 * 1024
)

type Method string

const (
	MethodCreate   Method = "challenge.create"
	MethodComplete Method = "challenge.complete"
	MethodConsume  Method = "proof.consume"
)

type PresentationKind string

const (
	PresentationImage      PresentationKind = "image"
	PresentationAccessible PresentationKind = "accessible"
	PresentationTurnstile  PresentationKind = "turnstile"
)

type CreateRequest struct {
	Purpose string           `json:"purpose"`
	Binding string           `json:"binding"`
	Kind    PresentationKind `json:"kind"`
}

type Presentation struct {
	ChallengeNo      string           `json:"challengeNo"`
	ProviderCode     string           `json:"providerCode"`
	ProviderAction   string           `json:"providerAction,omitempty"`
	Kind             PresentationKind `json:"kind"`
	ImageMediaType   string           `json:"imageMediaType,omitempty"`
	ImageBytes       []byte           `json:"imageBytes,omitempty"`
	AccessiblePrompt string           `json:"accessiblePrompt,omitempty"`
	SiteKey          string           `json:"siteKey,omitempty"`
	ExpiresAt        time.Time        `json:"expiresAt"`
}

type CompleteRequest struct {
	ChallengeNo   string `json:"challengeNo"`
	Purpose       string `json:"purpose"`
	Binding       string `json:"binding"`
	Answer        string `json:"answer,omitempty"`
	ProviderToken string `json:"providerToken,omitempty"`
}

type Proof struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type ConsumeRequest struct {
	Token   string `json:"token"`
	Purpose string `json:"purpose"`
	Binding string `json:"binding"`
}

type ConsumeResult struct {
	ChallengeNo string    `json:"challengeNo"`
	ConsumedAt  time.Time `json:"consumedAt"`
}

type Service interface {
	Create(context.Context, CreateRequest) (*Presentation, error)
	Complete(context.Context, CompleteRequest) (*Proof, error)
	Consume(context.Context, ConsumeRequest) (*ConsumeResult, error)
}
