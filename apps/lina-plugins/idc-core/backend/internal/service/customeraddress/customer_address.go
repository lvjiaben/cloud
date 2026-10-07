// Package customeraddress owns customer-managed ordinary addresses.
package customeraddress

import (
	"context"
	"time"
)

type Status string
type Type string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
	TypeContact    Type   = "contact"
	TypeBilling    Type   = "billing"
	TypeRegistered Type   = "registered"
	TypeService    Type   = "service"
)

type Address struct {
	AddressNo, Label, RecipientName, RecipientPhone, CountryCode     string
	Province, City, District, AddressLine1, AddressLine2, PostalCode string
	Type                                                             Type
	Default                                                          bool
	Status                                                           Status
	Version                                                          int64
	CreatedAt, UpdatedAt                                             time.Time
}

type SaveInput struct {
	AddressNo, Label, RecipientName, RecipientPhone, CountryCode     string
	Province, City, District, AddressLine1, AddressLine2, PostalCode string
	Type                                                             Type
	Default                                                          bool
	ExpectedVersion                                                  int64
	IdempotencyKey                                                   string
}

type ArchiveInput struct {
	AddressNo       string
	ExpectedVersion int64
	IdempotencyKey  string
}

type Service interface {
	List(context.Context) ([]Address, error)
	Save(context.Context, SaveInput) (*Address, error)
	Archive(context.Context, ArchiveInput) (*Address, error)
}
