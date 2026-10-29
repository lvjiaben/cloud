package customerlogin

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"strconv"
)

// Sync refreshes one fixed-size page through the current administrator's real
// public user-list permission and data scope. It never stores that bearer token.
// A second read under row locks prevents overwriting a concurrent contact save.
func (s *serviceImpl) Sync(ctx context.Context, page int, repairCustomerAccess bool) (*customerlogincap.SyncResult, error) {
	current := s.bizctx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 || page < 1 || page > 100000 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	path := "/user?pageSize=100&pageNum=" + strconv.Itoa(page) + "&orderBy=id&orderDirection=asc&tenantId=0"
	var first hostPage
	if err := s.request(ctx, path, "", &first); err != nil {
		return nil, err
	}
	if len(first.List) > 100 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	ids := make([]int, 0, len(first.List))
	for _, p := range first.List {
		if p.TenantID == 0 && p.ID > 0 {
			ids = append(ids, p.ID)
		}
	}
	if len(ids) == 0 {
		return &customerlogincap.SyncResult{Total: first.Total, RepairCandidateUserIDs: []int{}}, nil
	}
	var owners []ownedIdentity
	if err := s.owners(ctx).WhereIn("b.sys_user_id", ids).Fields("b.sys_user_id,c.customer_no,b.source_code,c.status_code AS customer_status").Scan(&owners); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(owners) == 0 {
		return &customerlogincap.SyncResult{Total: first.Total, RepairCandidateUserIDs: []int{}}, nil
	}
	byID := make(map[int]ownedIdentity, len(owners))
	placeholders := make([]do.CustomerLoginContact, 0, len(owners))
	lockedIDs := make([]int, 0, len(owners))
	for _, owner := range owners {
		byID[owner.UserID] = owner
		lockedIDs = append(lockedIDs, owner.UserID)
		placeholders = append(placeholders, do.CustomerLoginContact{TenantId: 0, UserId: owner.UserID, IdentityCiphertext: ""})
	}
	result := &customerlogincap.SyncResult{Total: first.Total, RepairCandidateUserIDs: []int{}}
	err := s.database.Transaction(ctx, func(tx context.Context, _ gdb.TX) error {
		if _, err := s.database.Model(table).Ctx(tx).Data(placeholders).InsertIgnore(); err != nil {
			return bizerr.WrapCode(err, CodeUnavailable)
		}
		var stored []entity.CustomerLoginContact
		if err := s.database.Model(table).Ctx(tx).Where("tenant_id", 0).WhereIn("user_id", lockedIDs).OrderAsc("user_id").LockUpdate().Scan(&stored); err != nil {
			return bizerr.WrapCode(err, CodeUnavailable)
		}
		rows := make(map[int]entity.CustomerLoginContact, len(stored))
		for _, r := range stored {
			rows[r.UserId] = r
		}
		var second hostPage
		if err := s.request(tx, path, "", &second); err != nil {
			return err
		}
		if !sameProfilePage(first.List, second.List) {
			return bizerr.NewCode(CodeChanged)
		}
		candidates, err := s.repairAccess(tx, second.List, byID, false)
		if err != nil {
			return err
		}
		result.RepairCandidateUserIDs = candidates
		result.AccessRepaired = false
		mutations := make([]do.CustomerLoginContact, 0, len(owners))
		for _, profile := range second.List {
			owner, exists := byID[profile.ID]
			if !exists || profile.TenantID != 0 {
				continue
			}
			row := rows[profile.ID]
			identity := &customerlogincap.Identity{UserID: profile.ID, CustomerNo: owner.CustomerNo, Username: profile.Username, Email: profile.Email, Phone: profile.Phone, Revision: row.Revision + 1}
			if row.IdentityCiphertext != "" {
				old, err := s.decrypt(row.IdentityCiphertext)
				if err != nil {
					return err
				}
				if old.Username == identity.Username && old.Email == identity.Email && old.Phone == identity.Phone && old.CustomerNo == owner.CustomerNo {
					result.Refreshed++
					continue
				}
			}
			ciphertext, err := s.encrypt(identity)
			if err != nil {
				return bizerr.WrapCode(err, CodeUnavailable)
			}
			mutations = append(mutations, do.CustomerLoginContact{TenantId: 0, UserId: profile.ID, EmailDigest: s.digest(customerlogincap.Email, profile.Email), PhoneDigest: s.digest(customerlogincap.Phone, profile.Phone), IdentityCiphertext: ciphertext, Revision: identity.Revision})
			result.Refreshed++
		}
		if len(mutations) == 0 {
			return nil
		}
		_, err = s.database.Model(table).Ctx(tx).Data(mutations).OnConflict("tenant_id", "user_id").OnDuplicate("email_digest", "phone_digest", "identity_ciphertext", "revision", "updated_at").Save()
		return err
	})
	if err != nil {
		return nil, err
	}
	// The public host assignment is the last business write, after the derived
	// index commits. A failed index write must never leave permissions changed.
	if repairCustomerAccess && len(result.RepairCandidateUserIDs) > 0 {
		candidates, err := s.repairCommittedAccess(ctx, path, first, ids)
		if err != nil {
			return nil, err
		}
		result.RepairCandidateUserIDs = candidates
		result.AccessRepaired = len(candidates) > 0
	}
	return result, nil
}
