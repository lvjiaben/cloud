// Package customerlogincap publishes the source-only customer login directory.
// It stores no passwords and never exposes contact projections to guest code.
package customerlogincap

import "context"

const AllowedCallerPluginID = "idc-notify"
const IdentityProviderID = "idc-customer-contact"

type Kind string

const (
	Username Kind = "username"
	Email    Kind = "email"
	Phone    Kind = "phone"
)

// Identity contains an authoritative-index projection for internal login only.
// Callers must validate the actual host-signed profile before returning a login
// or changing a password; a cached contact projection is not proof of control.
type Identity struct {
	UserID     int
	CustomerNo string
	Username   string
	Email      string
	Phone      string
	Revision   int64
}

type Patch struct{ Email, Phone, Username *string }

// Directory binds customer ownership to contact indexes. Resolve is an
// authentication lookup, not a public account search; duplicates fail closed.
// RefreshCurrent/RefreshUser accept only the actual authenticated host request.
// Sync requires a real administrator and forwards that request's bearer token to
// the public, data-scoped paginated user route. It never retains bearer tokens.
type Directory interface {
	Resolve(context.Context, Kind, string) (*Identity, error)
	Validate(context.Context, int) (string, error)
	ResolveSubject(context.Context, string) (int, error)
	Observe(context.Context, string) (*Identity, error)
	RefreshCurrent(context.Context) error
	RefreshUser(context.Context, int, Patch) error
	Sync(context.Context, int, bool) (*SyncResult, error)
}

// SyncResult reports derived indexes and the explicit roleless admin-created
// repair candidates; existing custom role assignments are never changed.
type SyncResult struct {
	Refreshed, Total       int
	RepairCandidateUserIDs []int
	AccessRepaired         bool
}
