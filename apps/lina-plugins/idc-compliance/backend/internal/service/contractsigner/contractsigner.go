// Package contractsigner owns the enterprise contract-signer directory.
package contractsigner

import (
	"context"
	"crypto/rand"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/internal/service/personalverification"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/securitycodecap"
)

const (
	profileTable              = "plugin_idc_compliance_contract_signer_profile"
	attemptTable              = "plugin_idc_compliance_contract_signer_identity_attempt"
	PhonePurpose              = "contract_signer_phone"
	ConsentPolicyVersion      = "1"
	PermissionContractCreate  = "idc:admin:compliance:contract:create"
	PermissionContractReissue = "idc:admin:compliance:contract:reissue"
)

type CreateInput struct {
	CertType             string
	CertName             []byte
	CertNo               []byte
	Phone                []byte
	ConsentAccepted      bool
	ConsentPolicyVersion string
	IdempotencyKey       string
}

type Signer struct {
	SignerNo                    string
	MaskedName                  string
	MaskedIdentityNo            string
	MaskedPhone                 string
	IdentityStatus              string
	IdentityAttemptNo           string
	IdentityVerificationVersion int64
	IdentityVerifiedAt          *time.Time
	PhoneStatus                 string
	PhoneVerificationNo         string
	PhoneVerificationVersion    int64
	PhoneVerifiedAt             *time.Time
	Status                      string
	AuthorizationVersion        int64
	Version                     int64
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

type IdentityStart struct {
	SignerNo        string
	AttemptNo       string
	Status          string
	ContinuationURL string
	ExpiresAt       *time.Time
}

type Candidate struct {
	SignerNo                    string
	MaskedName                  string
	MaskedPhone                 string
	IdentityVerificationVersion int64
	IdentityVerifiedAt          time.Time
	PhoneVerificationVersion    int64
	PhoneVerifiedAt             time.Time
	AuthorizationVersion        int64
	Version                     int64
}

type FrozenSigner struct {
	SignerNo                    string
	CustomerNo                  string
	Name                        []byte
	Phone                       []byte
	IdentityVerificationVersion int64
	IdentityVerifiedAt          time.Time
	PhoneVerificationNo         string
	PhoneVerificationVersion    int64
	PhoneVerifiedAt             time.Time
	AuthorizationVersion        int64
}

type FrozenSignerConsumer func(FrozenSigner) error

type Service interface {
	ListCurrent(context.Context) ([]Signer, error)
	Create(context.Context, CreateInput) (*Signer, error)
	StartIdentity(context.Context, string, string) (*IdentityStart, error)
	QueryIdentity(context.Context, string, string) (*Signer, error)
	CreatePhoneChallenge(context.Context, string, []byte, string, string) (*securitycodecap.HumanChallenge, error)
	CompletePhoneChallenge(context.Context, string, []byte, string, string, []byte, []byte) (*securitycodecap.HumanProof, error)
	SendPhoneCode(context.Context, string, []byte, string, string, string, []byte) (*securitycodecap.Verification, error)
	VerifyPhone(context.Context, string, []byte, string, string, []byte) (*Signer, error)
	Disable(context.Context, string, int64, string) (*Signer, error)
	ListCandidates(context.Context, string) ([]Candidate, error)
	ListReissueCandidates(context.Context, string, string) ([]Candidate, error)
	WithCandidate(context.Context, string, string, FrozenSignerConsumer) error
}

type serviceImpl struct {
	database        gdb.DB
	identities      identitycap.Service
	configurations  providerconfig.RuntimeSource
	clients         personalverification.ClientFactory
	profileAccessor secretcap.Accessor
	profileManager  secretcap.Manager
	securityCodes   securitycodecap.Service
	authorizer      managementcap.Authorizer
	audit           idccap.BusinessAuditWriter
	now             func() time.Time
}

var _ Service = (*serviceImpl)(nil)

func New(database gdb.DB, identities identitycap.Service, configurations providerconfig.RuntimeSource,
	clients personalverification.ClientFactory, profileAccessor secretcap.Accessor, profileManager secretcap.Manager,
	securityCodes securitycodecap.Service, authorizer managementcap.Authorizer, audit idccap.BusinessAuditWriter) (Service, error) {
	for _, dependency := range []any{database, identities, configurations, clients, profileAccessor, profileManager, securityCodes, authorizer, audit} {
		if nilDependency(dependency) {
			return nil, gerror.New("contract signer dependencies are invalid")
		}
	}
	return &serviceImpl{database: database, identities: identities, configurations: configurations, clients: clients,
		profileAccessor: profileAccessor, profileManager: profileManager, securityCodes: securityCodes,
		authorizer: authorizer, audit: audit, now: time.Now}, nil
}

func nilDependency(value any) bool {
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

func randomBytes(size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		clear(value)
		return nil, err
	}
	return value, nil
}
