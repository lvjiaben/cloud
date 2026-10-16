// This file proves channel file principals are explicit owner-issued context
// evidence, rather than a fallback to whatever browser identity is present.
package backend

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestChannelFilePrincipalRequiresExplicitOwnedBinding(t *testing.T) {
	reader := channelFilePrincipal{}
	if _, err := reader.Current(t.Context()); err == nil {
		t.Fatal("unbound API request acquired an uploader")
	}
	for _, audience := range []principal.Audience{{UserID: 9}, {CustomerNo: "customer"}, {UserID: 9, CustomerNo: "customer"}, {UserID: 9, CustomerNo: "customer", Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive}} {
		ctx := context.WithValue(t.Context(), channelFilePrincipalKey{}, audience)
		got, err := reader.Current(ctx)
		if audience.UserID > 0 && audience.CustomerNo != "" && audience.Code == principal.AudienceCustomer && audience.Status == principal.AudienceStatusActive {
			if err != nil || got.UserID != 9 || got.CustomerNo != "customer" {
				t.Fatal("explicit owner binding was lost")
			}
		} else if err == nil {
			t.Fatal("partial uploader identity was accepted")
		}
	}
}
