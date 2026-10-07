package serveroperationresourcecap

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrIPCapacityUnavailable = errors.New("server IP capacity is unavailable")
	ErrIPReservationConflict = errors.New("server IP reservation conflicts with its frozen request")
)

type IPReservationStatus string

const (
	IPReservationReserved IPReservationStatus = "reserved"
	IPReservationConsumed IPReservationStatus = "consumed"
	IPReservationReleased IPReservationStatus = "released"
	IPReservationExpired  IPReservationStatus = "expired"
)

// IPReservationRequest freezes the resource route and capacity-freshness rule
// chosen by the server owner. The resource owner serializes reservations on
// the selected pool row.
type IPReservationRequest struct {
	Route                     CapabilityRoute
	Quantity                  int
	MaximumCapacityAgeSeconds int
	ExpiresAt                 time.Time
	IdempotencyKey            string
	RequestDigest             string
}

type IPReservation struct {
	ReservationNo         string
	CapacityObservationNo string
	Quantity              int
	Status                IPReservationStatus
	ExpiresAt             time.Time
	Reused                bool
}

type IPReservationReleaseRequest struct {
	ReservationNo string
	ReasonCode    string
}

type IPReservationConsumeRequest struct {
	ReservationNo  string
	ProviderTaskNo string
}

// IPCapacity is a separate owner boundary so ordinary Provider task sources
// cannot accidentally reserve or release IP capacity.
type IPCapacity interface {
	ReserveIP(context.Context, IPReservationRequest) (*IPReservation, error)
	ReleaseIP(context.Context, IPReservationReleaseRequest) (*IPReservation, error)
	ConsumeIP(context.Context, IPReservationConsumeRequest) (*IPReservation, error)
}

func IPReservationRequestDigest(request IPReservationRequest) (string, error) {
	if !validReference(request.Route.ServiceNo, 64) || !validReference(request.Route.ChannelNo, 64) ||
		!validReference(request.Route.PoolNo, 64) || request.Quantity < 1 || request.Quantity > 64 ||
		request.MaximumCapacityAgeSeconds < 60 || request.MaximumCapacityAgeSeconds > 86400 ||
		request.ExpiresAt.IsZero() || len(request.IdempotencyKey) < 8 || len(request.IdempotencyKey) > 256 ||
		request.IdempotencyKey != strings.TrimSpace(request.IdempotencyKey) {
		return "", ErrIPReservationConflict
	}
	payload, err := json.Marshal(struct {
		ContractVersion                     string
		ServiceNo, ChannelNo, PoolNo        string
		Quantity, MaximumCapacityAgeSeconds int
		ExpiresAt                           int64
		IdempotencyKey                      string
	}{ContractVersion, request.Route.ServiceNo, request.Route.ChannelNo, request.Route.PoolNo,
		request.Quantity, request.MaximumCapacityAgeSeconds, request.ExpiresAt.UTC().UnixMilli(), request.IdempotencyKey})
	if err != nil {
		return "", ErrIPReservationConflict
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func ValidateIPReservationRequest(request IPReservationRequest) error {
	if !digestPattern.MatchString(request.RequestDigest) {
		return ErrIPReservationConflict
	}
	digest, err := IPReservationRequestDigest(request)
	if err != nil || subtle.ConstantTimeCompare([]byte(digest), []byte(request.RequestDigest)) != 1 {
		return ErrIPReservationConflict
	}
	return nil
}

func ValidateIPReservation(reservation IPReservation) error {
	if !validReference(reservation.ReservationNo, 64) || !validReference(reservation.CapacityObservationNo, 64) ||
		reservation.Quantity < 1 || reservation.Quantity > 64 || reservation.ExpiresAt.IsZero() {
		return ErrIPReservationConflict
	}
	switch reservation.Status {
	case IPReservationReserved, IPReservationConsumed, IPReservationReleased, IPReservationExpired:
		return nil
	default:
		return ErrIPReservationConflict
	}
}
