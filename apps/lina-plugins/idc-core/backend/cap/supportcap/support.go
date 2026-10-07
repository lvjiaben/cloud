// Package supportcap defines the source-only anonymous presales creation
// boundary consumed by the IDC notification plugin after OTP verification.
package supportcap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	OwnerPluginID              = "idc-core"
	AllowedCallerPluginID      = "idc-notify"
	ContractVersion            = "v1"
	AnonymousAccessSecretScope = "core.support-anonymous-access"

	maximumOperationNoBytes = 64
	maximumCategoryNoBytes  = 40
	maximumSubjectBytes     = 256
	maximumBodyBytes        = 65536
	maximumCustomFields     = 100
	maximumSelectedOptions  = 50
)

var (
	referencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$`)
	categoryPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)
	digestPattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	ticketPattern    = regexp.MustCompile(`^TKT_[A-Za-z0-9_-]{1,36}$`)
	tokenPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

// DestinationKind identifies the verified delivery address class. The raw
// email address or phone number never crosses this boundary.
type DestinationKind string

const (
	DestinationEmail DestinationKind = "email"
	DestinationSMS   DestinationKind = "sms"
)

// CreateAnonymousPresalesRequest contains only the already-verified,
// normalized destination digest and bounded public ticket content.
type CreateAnonymousPresalesRequest struct {
	DestinationKind   DestinationKind    `json:"destinationKind"`
	DestinationDigest string             `json:"destinationDigest"`
	CategoryNo        string             `json:"categoryNo"`
	Subject           string             `json:"subject"`
	Body              string             `json:"body"`
	CustomFields      []CustomFieldValue `json:"customFields"`
	OperationNo       string             `json:"operationNo"`
}

// CustomFieldValue is a typed, bounded submission. The core owner resolves
// FieldNo against the current public presales definition and rejects every
// value member that does not match that definition.
type CustomFieldValue struct {
	FieldNo      string   `json:"fieldNo"`
	TextValue    *string  `json:"textValue,omitempty"`
	NumberValue  *string  `json:"numberValue,omitempty"`
	DateValue    *string  `json:"dateValue,omitempty"`
	BooleanValue *bool    `json:"booleanValue,omitempty"`
	OptionNos    []string `json:"optionNos,omitempty"`
}

// CreateAnonymousPresalesResult is returned only to notify so it can send the
// access link to the destination it has already verified. AccessToken must not
// be logged, persisted by notify, or returned through an unrelated endpoint.
type CreateAnonymousPresalesResult struct {
	TicketNo    string    `json:"ticketNo"`
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// RenewAnonymousPresalesAccessRequest contains only the ticket identity and
// the destination digest that notify has just verified again. It cannot
// reactivate an expired token or reveal whether an unrelated ticket exists.
type RenewAnonymousPresalesAccessRequest struct {
	DestinationKind   DestinationKind `json:"destinationKind"`
	DestinationDigest string          `json:"destinationDigest"`
	TicketNo          string          `json:"ticketNo"`
	OperationNo       string          `json:"operationNo"`
}

// RenewAnonymousPresalesAccessResult is one newly signed two-hour token. The
// prior token remains unchanged and can never become valid again.
type RenewAnonymousPresalesAccessResult struct {
	TicketNo    string    `json:"ticketNo"`
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// Service atomically creates or recovers one anonymous presales ticket and its
// fixed two-hour, single-ticket access token.
type Service interface {
	CreateAnonymousPresales(
		context.Context,
		CreateAnonymousPresalesRequest,
	) (*CreateAnonymousPresalesResult, error)
	RenewAnonymousPresalesAccess(
		context.Context,
		RenewAnonymousPresalesAccessRequest,
	) (*RenewAnonymousPresalesAccessResult, error)
}

// ValidateCreateRequest rejects plaintext destinations, ambiguous strings,
// unbounded content, and malformed digests before owner persistence runs.
func ValidateCreateRequest(request CreateAnonymousPresalesRequest) error {
	if request.DestinationKind != DestinationEmail && request.DestinationKind != DestinationSMS {
		return gerror.New("supportcap: destination kind is invalid")
	}
	if !digestPattern.MatchString(request.DestinationDigest) ||
		!categoryPattern.MatchString(request.CategoryNo) ||
		!validReference(request.OperationNo, maximumOperationNoBytes) ||
		!validText(request.Subject, maximumSubjectBytes) ||
		!validText(request.Body, maximumBodyBytes) {
		return gerror.New("supportcap: anonymous presales request is invalid")
	}
	if len(request.CustomFields) > maximumCustomFields {
		return gerror.New("supportcap: anonymous presales custom fields are invalid")
	}
	seenFields := make(map[string]struct{}, len(request.CustomFields))
	for _, field := range request.CustomFields {
		if !categoryPattern.MatchString(field.FieldNo) {
			return gerror.New("supportcap: anonymous presales custom fields are invalid")
		}
		if _, duplicate := seenFields[field.FieldNo]; duplicate {
			return gerror.New("supportcap: anonymous presales custom fields are invalid")
		}
		seenFields[field.FieldNo] = struct{}{}
		members := 0
		if field.TextValue != nil {
			members++
			if !validText(*field.TextValue, maximumBodyBytes) {
				return gerror.New("supportcap: anonymous presales custom fields are invalid")
			}
		}
		if field.NumberValue != nil {
			members++
			if !validText(*field.NumberValue, 64) {
				return gerror.New("supportcap: anonymous presales custom fields are invalid")
			}
		}
		if field.DateValue != nil {
			members++
			if !validText(*field.DateValue, 10) {
				return gerror.New("supportcap: anonymous presales custom fields are invalid")
			}
		}
		if field.BooleanValue != nil {
			members++
		}
		if len(field.OptionNos) > 0 {
			members++
			if len(field.OptionNos) > maximumSelectedOptions {
				return gerror.New("supportcap: anonymous presales custom fields are invalid")
			}
			seenOptions := make(map[string]struct{}, len(field.OptionNos))
			for _, optionNo := range field.OptionNos {
				if !categoryPattern.MatchString(optionNo) {
					return gerror.New("supportcap: anonymous presales custom fields are invalid")
				}
				if _, duplicate := seenOptions[optionNo]; duplicate {
					return gerror.New("supportcap: anonymous presales custom fields are invalid")
				}
				seenOptions[optionNo] = struct{}{}
			}
		}
		if members != 1 {
			return gerror.New("supportcap: anonymous presales custom fields are invalid")
		}
	}
	return nil
}

// ValidateCreateResult ensures callers never accept a malformed owner result.
func ValidateCreateResult(result CreateAnonymousPresalesResult) error {
	if !validAccessResult(result.TicketNo, result.AccessToken, result.ExpiresAt) {
		return gerror.New("supportcap: anonymous presales result is invalid")
	}
	return nil
}

// ValidateRenewRequest accepts only the normalized digest produced by notify
// after a fresh verification and a bounded anonymous ticket number.
func ValidateRenewRequest(request RenewAnonymousPresalesAccessRequest) error {
	if request.DestinationKind != DestinationEmail && request.DestinationKind != DestinationSMS {
		return gerror.New("supportcap: destination kind is invalid")
	}
	if !digestPattern.MatchString(request.DestinationDigest) ||
		!ticketPattern.MatchString(request.TicketNo) ||
		!validReference(request.OperationNo, maximumOperationNoBytes) {
		return gerror.New("supportcap: anonymous presales renewal request is invalid")
	}
	return nil
}

// ValidateRenewResult applies the same strict token shape to renewed access.
func ValidateRenewResult(result RenewAnonymousPresalesAccessResult) error {
	if !validAccessResult(result.TicketNo, result.AccessToken, result.ExpiresAt) {
		return gerror.New("supportcap: anonymous presales renewal result is invalid")
	}
	return nil
}

// OperationDigest returns the purpose-separated digest persisted as the
// idempotency lookup. The original operation number is never stored.
func OperationDigest(operationNo string) (string, error) {
	return operationDigest("create", operationNo)
}

// RenewOperationDigest separates renewal idempotency from ticket creation,
// even if a caller accidentally reuses the same operation number.
func RenewOperationDigest(operationNo string) (string, error) {
	return operationDigest("renew", operationNo)
}

func operationDigest(operation string, operationNo string) (string, error) {
	if !validReference(operationNo, maximumOperationNoBytes) {
		return "", gerror.New("supportcap: operation number is invalid")
	}
	digest := sha256.Sum256([]byte("idc-supportcap-anonymous-operation-v1\x00" + operation + "\x00" + operationNo))
	return hex.EncodeToString(digest[:]), nil
}

func validAccessResult(ticketNo, accessToken string, expiresAt time.Time) bool {
	return ticketPattern.MatchString(ticketNo) && tokenPattern.MatchString(accessToken) && !expiresAt.IsZero()
}

func validReference(value string, maximum int) bool {
	return len(value) <= maximum && value == strings.TrimSpace(value) && referencePattern.MatchString(value)
}

func validText(value string, maximum int) bool {
	return value != "" && value == strings.TrimSpace(value) && utf8.ValidString(value) && len([]byte(value)) <= maximum
}
