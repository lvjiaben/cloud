// This file verifies that managed-service customer access always delegates to
// the authoritative current-customer reader with the original request context.

package serviceaction

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

// currentCustomerProofReader supplies a per-test mutable authoritative result.
type currentCustomerProofReader struct {
	subject  *identitycap.CustomerSubject
	err      error
	contexts []context.Context
}

func (r *currentCustomerProofReader) CurrentCustomer(ctx context.Context) (*identitycap.CustomerSubject, error) {
	r.contexts = append(r.contexts, ctx)
	return r.subject, r.err
}

// TestCustomerAccessRechecksAuthoritativeIdentity covers a grant-time proof
// followed by a new request after unlinking, disabling or changing the customer.
func TestCustomerAccessRechecksAuthoritativeIdentity(t *testing.T) {
	var (
		reader                        = &currentCustomerProofReader{subject: &identitycap.CustomerSubject{CustomerNo: "10000", ActorUserID: 51}}
		access                        = &productionAccess{customers: reader}
		grantContext, cancelGrant     = context.WithCancel(context.Background())
		requestContext, cancelRequest = context.WithCancel(context.Background())
	)
	defer cancelGrant()
	defer cancelRequest()
	actor, err := access.Customer(grantContext)
	if err != nil || actor.UserID != 51 || actor.CustomerNo != "10000" || actor.Audience != AudienceCustomer {
		t.Fatalf("expected owner-proved customer: actor=%#v error=%v", actor, err)
	}
	for _, testCase := range []struct {
		name    string
		subject *identitycap.CustomerSubject
		err     error
	}{
		{name: "unbound"},
		{name: "disabled", subject: reader.subject, err: errors.New("authoritative identity disabled")},
		{name: "no user", subject: &identitycap.CustomerSubject{CustomerNo: "10000"}},
		{name: "no customer", subject: &identitycap.CustomerSubject{ActorUserID: 51}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			reader.subject, reader.err = testCase.subject, testCase.err
			before := len(reader.contexts)
			if _, accessErr := access.Customer(requestContext); !bizerr.Is(accessErr, CodeNotAccessible) {
				t.Fatalf("expected inaccessible after identity change: %v", accessErr)
			}
			if len(reader.contexts) != before+1 || reader.contexts[before] != requestContext {
				t.Fatal("identity proof was cached or received a replacement context")
			}
		})
	}
	if reader.contexts[0] != grantContext {
		t.Fatal("grant proof lost original context")
	}
}
