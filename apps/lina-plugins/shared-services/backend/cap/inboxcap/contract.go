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

type PublishResult struct {
	MessageNo   string    `json:"messageNo"`
	EvidenceNo  string    `json:"evidenceNo"`
	PublishedAt time.Time `json:"publishedAt"`
	Reused      bool      `json:"reused"`
}

type Service interface {
	Publish(context.Context, PublishRequest) (*PublishResult, error)
}
