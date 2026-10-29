// Package channelcap defines the source-only notification delivery adapter SPI.
package channelcap

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

const MaximumConfigurationFields = 32

type Kind string

const (
	KindEmail Kind = "email"
	KindSMS   Kind = "sms"
)

// Field describes one adapter-owned setup field without carrying a value.
type Field struct {
	Key       string
	LabelKey  string
	Secret    bool
	Required  bool
	MaxLength int
}

// Descriptor is frozen startup metadata for one exact provider implementation.
type Descriptor struct {
	ProviderCode string
	Kind         Kind
	Fields       []Field
	Factory      Factory
}

// Factory constructs an adapter from one callback-only credential accessor.
type Factory func(secretcap.Accessor) (Adapter, error)

type Delivery struct {
	Test               bool
	TaskNo             string
	IdempotencyKey     string
	EventNo            string
	EventType          string
	EventVersion       int64
	OccurredAt         time.Time
	Credential         secretcap.Handle
	Destination        []byte
	Subject            string
	Body               string
	Locale             string
	ProviderTemplateID string
	ProviderParameters []string
	ProviderData       map[string]string
	NonSecretConfig    map[string]string
}

type ResultClass string

const (
	ResultAccepted                       ResultClass = "accepted"
	ResultDelivered                      ResultClass = "delivered"
	ResultDefinitiveNotReceivedTemporary ResultClass = "definitive_not_received_temporary"
	ResultDefinitiveNotReceivedPermanent ResultClass = "definitive_not_received_permanent"
	ResultInvalidRecipient               ResultClass = "invalid_recipient"
	ResultInsufficientBalance            ResultClass = "insufficient_balance"
	ResultConfigurationError             ResultClass = "configuration_error"
	ResultUnknown                        ResultClass = "result_unknown"
)

type Result struct {
	Class                 ResultClass
	ProviderMessageRef    string
	EvidenceDigest        string
	QueryDeadline         *time.Time
	QueryDeadlineSource   string
	CrossedExternalBorder bool
}

type Query struct {
	TaskNo             string
	ProviderMessageRef string
	Credential         secretcap.Handle
	NonSecretConfig    map[string]string
}

// Adapter sends once and performs read-only result queries. It never owns retries.
type Adapter interface {
	ValidateConfiguration(context.Context, map[string]string, map[string][]byte) error
	ValidateDestination([]byte) error
	Deliver(context.Context, Delivery) (Result, error)
	Query(context.Context, Query) (Result, error)
}

var registry = struct {
	sync.Mutex
	frozen bool
	items  map[string]Descriptor
}{items: make(map[string]Descriptor)}

// Declare registers one adapter descriptor during source-plugin initialization.
func Declare(descriptor Descriptor) error {
	registry.Lock()
	defer registry.Unlock()
	if registry.frozen {
		return gerror.New("notification channel adapter registry is frozen")
	}
	if err := ValidateDescriptor(descriptor); err != nil {
		return err
	}
	if _, exists := registry.items[descriptor.ProviderCode]; exists {
		return gerror.New("notification channel adapter provider is already declared")
	}
	descriptor.Fields = append([]Field(nil), descriptor.Fields...)
	registry.items[descriptor.ProviderCode] = descriptor
	return nil
}

// Freeze returns one immutable, deterministically ordered startup snapshot.
func Freeze() ([]Descriptor, error) {
	registry.Lock()
	defer registry.Unlock()
	registry.frozen = true
	items := make([]Descriptor, 0, len(registry.items))
	for _, descriptor := range registry.items {
		descriptor.Fields = append([]Field(nil), descriptor.Fields...)
		items = append(items, descriptor)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ProviderCode < items[j].ProviderCode })
	return items, nil
}

func ValidateDescriptor(descriptor Descriptor) error {
	if !validCode(descriptor.ProviderCode, 64) || !validKind(descriptor.Kind) ||
		descriptor.Factory == nil || len(descriptor.Fields) > MaximumConfigurationFields {
		return gerror.New("notification channel adapter descriptor is invalid")
	}
	seen := make(map[string]struct{}, len(descriptor.Fields))
	for _, field := range descriptor.Fields {
		if !validCode(field.Key, 64) || !validCode(field.LabelKey, 128) ||
			field.MaxLength <= 0 || field.MaxLength > secretcap.MaximumManagedPlaintextBytes {
			return gerror.New("notification channel adapter field is invalid")
		}
		if _, exists := seen[field.Key]; exists {
			return gerror.New("notification channel adapter field is duplicated")
		}
		seen[field.Key] = struct{}{}
	}
	return nil
}

func ValidateAdapter(adapter Adapter) error {
	if nilValue(adapter) {
		return gerror.New("notification channel adapter is unavailable")
	}
	return nil
}

func validKind(kind Kind) bool {
	switch kind {
	case KindEmail, KindSMS:
		return true
	default:
		return false
	}
}

func validCode(value string, maximum int) bool {
	if value == "" || len(value) > maximum || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') &&
			character != '_' && character != '-' && character != '.' {
			return false
		}
	}
	return true
}

func nilValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
