// This file defines durable owner-side providerCapacity observation publication.

package providercapacity

import (
	"context"
	"crypto/rand"
	"errors"
	"io"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var (
	ErrObservationAlreadyCollecting = errors.New("resource providerCapacity observation is already collecting")
	ErrSynchronizationLeaseChanged  = errors.New("resource providerCapacity synchronization lease changed")
)

const (
	// MaximumObservationPageCount bounds one complete upstream traversal.
	MaximumObservationPageCount = 200
	// MaximumObservationItemCount bounds one staged observation independently of page shape.
	MaximumObservationItemCount = 100000
	// AutomaticPageSize is the fixed owner-controlled page size for management-triggered work.
	AutomaticPageSize = 100
)

const (
	TriggerSystem = "system"
	TriggerManual = "manual"
)

// Status is the durable providerCapacity publication lifecycle.
type Status string

const (
	StatusCollecting Status = "collecting"
	StatusComplete   Status = "complete"
	StatusFailed     Status = "failed"
	StatusSuperseded Status = "superseded"
)

// BeginInput starts one bounded full-providerCapacity traversal.
type BeginInput struct {
	ChannelNo string
	Kind      resourcecap.ProviderCapacityKind
	PageSize  int
}

// StageInput persists exactly the page produced by Request.
type StageInput struct {
	ObservationNo string
	Request       resourcecap.ProviderCapacityRequest
	Page          resourcecap.ProviderCapacityPage
	leaseDigest   string
}

// Observation is the non-secret continuation state owned by this service.
type Observation struct {
	ObservationNo string
	Status        Status
	ItemCount     int
	PageCount     int
	PageSize      int
	NextCursor    string
	Fingerprint   string
}

type beginOptions struct {
	TriggerCode string
	RequestedBy int
}

// Service owns durable staging and terminal publication of Provider providerCapacity.
type Service interface {
	// Begin creates one collecting observation for the exact channel, kind, and fixed page size.
	// It returns durable continuation state and rejects invalid input or an existing collector.
	Begin(ctx context.Context, input BeginInput) (Observation, error)
	// Stage atomically persists the exact next Provider page and publishes only a coherent terminal page.
	// It returns updated continuation state and rejects cursor drift, duplicate items, or capacity overflow.
	Stage(ctx context.Context, input StageInput) (Observation, error)
	// Fail terminally closes one collecting observation without replacing the last complete publication.
	// It returns an error for malformed, missing, or non-collecting observation numbers.
	Fail(ctx context.Context, observationNo string) error
}

type serviceImpl struct {
	database gdb.DB
	random   io.Reader
}

// New constructs an providerCapacity observation service against an explicitly injected database.
func New(database gdb.DB) (Service, error) {
	if database == nil {
		return nil, gerror.New("resource providerCapacity observation database is required")
	}
	return &serviceImpl{database: database, random: rand.Reader}, nil
}
