// Package inboxcap defines idempotent, rendered in-app message publication.
package inboxcap

import (
	"context"
	"time"
)

const (
	OwnerPluginID          = "shared-services"
	ServiceName            = "inbox"
	SourceContractPackage  = "lina-plugin-shared-services/backend/cap/inboxcap"
	DynamicContractPackage = SourceContractPackage + "/bridge"
	ContractVersion        = "v1"
	MethodPublish          = "message.publish"
	MethodCurrentSync      = "message.currentSync"
	MaximumCurrentSync     = 50
	MaximumPayloadBytes    = 16 * 1024
)

type PublishRequest struct {
	RecipientUserID int64  `json:"recipientUserId"`
	SourceType      string `json:"sourceType"`
	SourceNo        string `json:"sourceNo"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Title           string `json:"title"`
	Body            string `json:"body"`
	Category        string `json:"category"`
	ActionURL       string `json:"actionUrl,omitempty"`
}

// PublishResult acknowledges durable intent only; PublishedAt is its storage time,
// not proof that the host inbox contains the message or that the user read it.
type PublishResult struct {
	MessageNo   string    `json:"messageNo"`
	EvidenceNo  string    `json:"evidenceNo"`
	PublishedAt time.Time `json:"publishedAt"`
	Reused      bool      `json:"reused"`
}

type Service interface {
	// Publish atomically stores caller-bound durable intent and idempotency evidence.
	// It may run in the background and does not acknowledge host inbox delivery.
	Publish(context.Context, PublishRequest) (*PublishResult, error)
}

// CurrentSyncService projects only the authenticated platform user's stored
// messages to the host inbox using the original request context. No recipient
// selector is accepted. Employee data scopes never broaden the self boundary;
// tenant and impersonated contexts fail closed.
type CurrentSyncService interface {
	// CurrentSync delivers at most limit intents (zero selects the bounded default).
	// Each batch and its host delivery evidence commit atomically; failures retain
	// pending intents. Concurrent callers may skip locked rows and retry later.
	CurrentSync(context.Context, int) (*CurrentSyncResult, error)
}

type CurrentSyncRequest struct {
	Limit int `json:"limit"`
}

// CurrentSyncResult separates actual host deliveries from still-pending intents.
type CurrentSyncResult struct {
	SyncedCount  int `json:"syncedCount"`
	PendingCount int `json:"pendingCount"`
}
