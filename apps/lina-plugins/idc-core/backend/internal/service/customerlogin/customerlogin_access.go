// This file obtains the host public registration role through the actual
// operator permission. Customers receive self visibility, never staff scope.
package customerlogin

import (
	"context"
	"lina-core/pkg/bizerr"
	"net/http"
	"strconv"
)

const (
	adminDirectSource    = "admin_direct"
	activeCustomerStatus = "active"
	customerRoleKey      = "user"
	selfDataScope        = 4
	enabledRoleStatus    = 1
)

// DefaultRoleID requires one enabled self-only role matching public registration.
func (s *serviceImpl) DefaultRoleID(ctx context.Context) (int, error) {
	var page struct {
		List []struct {
			ID        int    `json:"id"`
			Key       string `json:"key"`
			Status    int    `json:"status"`
			DataScope int    `json:"dataScope"`
		}
		Total int `json:"total"`
	}
	if err := s.request(ctx, "/role?key=user&status=1&page=1&size=100", "", &page); err != nil {
		return 0, err
	}
	if page.Total > 100 {
		return 0, bizerr.NewCode(CodeUnavailable)
	}
	id := 0
	for _, item := range page.List {
		if item.Key == customerRoleKey && item.Status == enabledRoleStatus && item.DataScope == selfDataScope && item.ID > 0 {
			if id != 0 {
				return 0, bizerr.NewCode(CodeUnavailable)
			}
			id = item.ID
		}
	}
	if id <= 0 {
		return 0, bizerr.NewCode(CodeUnavailable)
	}
	return id, nil
}

// repairAccess is explicit and excludes existing role allocations, frozen host
// users and every binding not created by the previously roleless admin flow.
// At most one public batch assignment is performed for the bounded page.
func (s *serviceImpl) repairAccess(ctx context.Context, profiles []hostProfile, owners map[int]ownedIdentity, repair bool) ([]int, error) {
	candidates := make([]int, 0)
	for _, profile := range profiles {
		owner, exists := owners[profile.ID]
		if exists && owner.SourceCode == adminDirectSource && owner.CustomerStatus == activeCustomerStatus && profile.Status == enabledRoleStatus && profile.TenantID == 0 && len(profile.RoleIDs) == 0 {
			candidates = append(candidates, profile.ID)
		}
	}
	if !repair || len(candidates) == 0 {
		return candidates, nil
	}
	roleID, err := s.DefaultRoleID(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.exchange(ctx, http.MethodPost, "/role/"+strconv.Itoa(roleID)+"/users", "", struct {
		UserIDs []int `json:"userIds"`
	}{candidates}, nil); err != nil {
		return nil, err
	}
	return candidates, nil
}

// repairCommittedAccess revalidates the actual page and ownership after the
// derived-index transaction succeeds, then performs one final public batch write.
func (s *serviceImpl) repairCommittedAccess(ctx context.Context, path string, first hostPage, ids []int) ([]int, error) {
	var fresh hostPage
	if err := s.request(ctx, path, "", &fresh); err != nil {
		return nil, err
	}
	if !sameProfilePage(first.List, fresh.List) {
		return nil, bizerr.NewCode(CodeChanged)
	}
	var currentOwners []ownedIdentity
	if err := s.owners(ctx).WhereIn("b.sys_user_id", ids).Fields("b.sys_user_id,c.customer_no,b.source_code,c.status_code AS customer_status").Scan(&currentOwners); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	actual := make(map[int]ownedIdentity, len(currentOwners))
	for _, owner := range currentOwners {
		actual[owner.UserID] = owner
	}
	return s.repairAccess(ctx, fresh.List, actual, true)
}

func sameProfilePage(first, next []hostProfile) bool {
	if len(first) != len(next) {
		return false
	}
	for i := range next {
		if first[i].ID != next[i].ID {
			return false
		}
	}
	return true
}
