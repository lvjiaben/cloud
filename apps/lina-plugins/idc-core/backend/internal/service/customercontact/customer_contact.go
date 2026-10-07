// Package customercontact owns notification-only contacts for the current
// customer. Contacts never grant login, staff, payment or contract authority.
package customercontact

import (
	"context"
	"time"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

type Contact struct {
	ContactNo        string
	DisplayName      string
	Email            string
	EmailVerified    bool
	PhoneCountryCode string
	Phone            string
	PhoneVerified    bool
	PreferredLocale  string
	Status           Status
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type SaveInput struct {
	ContactNo        string
	DisplayName      string
	Email            string
	PhoneCountryCode string
	Phone            string
	PreferredLocale  string
	ExpectedVersion  int64
	IdempotencyKey   string
}

type ArchiveInput struct {
	ContactNo       string
	ExpectedVersion int64
	IdempotencyKey  string
}

type Service interface {
	List(context.Context) ([]Contact, error)
	Save(context.Context, SaveInput) (*Contact, error)
	Archive(context.Context, ArchiveInput) (*Contact, error)
}
