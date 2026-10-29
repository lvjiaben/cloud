package customerlogin

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"strconv"
)

type ownedIdentity struct {
	UserID         int    `orm:"sys_user_id"`
	CustomerNo     string `orm:"customer_no"`
	SourceCode     string `orm:"source_code"`
	CustomerStatus string `orm:"customer_status"`
}

func (s *serviceImpl) owners(ctx context.Context) *gdb.Model {
	return s.database.Model("plugin_idc_core_customer_identity_binding b").Ctx(ctx).
		InnerJoin("plugin_idc_core_customer_subject c", "c.tenant_id=b.tenant_id AND c.customer_no=b.customer_no").
		InnerJoin("plugin_idc_core_principal_audience a", "a.tenant_id=b.tenant_id AND a.sys_user_id=b.sys_user_id").
		Where("b.tenant_id", 0).Where("b.status_code", "active").Where("a.audience_code", string(principal.AudienceCustomer)).Where("a.status_code", string(principal.AudienceStatusActive)).
		WhereIn("c.status_code", []string{string(customer.StatusActive), string(customer.StatusRestricted)})
}
func (s *serviceImpl) customer(ctx context.Context, userID int) (string, error) {
	if userID <= 0 {
		return "", bizerr.NewCode(CodeInvalid)
	}
	var item ownedIdentity
	if err := s.owners(ctx).Where("b.sys_user_id", userID).Fields("b.sys_user_id,c.customer_no").Scan(&item); err != nil {
		return "", bizerr.WrapCode(err, CodeUnavailable)
	}
	if item.UserID != userID || item.CustomerNo == "" {
		return "", bizerr.NewCode(CodeInvalid)
	}
	return item.CustomerNo, nil
}
func (s *serviceImpl) ResolveSubject(ctx context.Context, subject string) (int, error) {
	if len(subject) > 48 || subject == "" {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	var items []ownedIdentity
	if err := s.owners(ctx).Where("c.customer_no", subject).Fields("b.sys_user_id,c.customer_no").Limit(2).Scan(&items); err != nil {
		return 0, bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(items) != 1 {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	return items[0].UserID, nil
}
func (s *serviceImpl) Resolve(ctx context.Context, kind customerlogincap.Kind, value string) (*customerlogincap.Identity, error) {
	if kind != customerlogincap.Email && kind != customerlogincap.Phone {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	digest := s.digest(kind, value)
	if digest == "" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	field := "i.email_digest"
	if kind == customerlogincap.Phone {
		field = "i.phone_digest"
	}
	var rows []entity.CustomerLoginContact
	if err := s.owners(ctx).InnerJoin(table+" i", "i.tenant_id=b.tenant_id AND i.user_id=b.sys_user_id").Where(field, digest).Fields("i.*").Limit(2).Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	identity, err := s.decrypt(rows[0].IdentityCiphertext)
	if err != nil {
		return nil, err
	}
	if identity.UserID != rows[0].UserId {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	identity.Revision = rows[0].Revision
	return identity, nil
}
func (s *serviceImpl) lock(ctx context.Context, userID int, write func(context.Context, *entity.CustomerLoginContact) error) error {
	if userID <= 0 {
		return bizerr.NewCode(CodeInvalid)
	}
	return s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, err := s.database.Model(table).Ctx(txCtx).Data(do.CustomerLoginContact{TenantId: 0, UserId: userID, IdentityCiphertext: ""}).InsertIgnore(); err != nil {
			return bizerr.WrapCode(err, CodeUnavailable)
		}
		var row entity.CustomerLoginContact
		if err := s.database.Model(table).Ctx(txCtx).Where(do.CustomerLoginContact{TenantId: 0, UserId: userID}).LockUpdate().Scan(&row); err != nil {
			return bizerr.WrapCode(err, CodeUnavailable)
		}
		return write(txCtx, &row)
	})
}
func (s *serviceImpl) save(ctx context.Context, profile *hostProfile, row *entity.CustomerLoginContact, customerNo string) (*customerlogincap.Identity, error) {
	identity := &customerlogincap.Identity{UserID: profile.ID, CustomerNo: customerNo, Username: profile.Username, Email: profile.Email, Phone: profile.Phone, Revision: row.Revision}
	if row.IdentityCiphertext != "" {
		old, err := s.decrypt(row.IdentityCiphertext)
		if err != nil {
			return nil, err
		}
		if old.Username == identity.Username && old.Email == identity.Email && old.Phone == identity.Phone && old.CustomerNo == customerNo {
			return identity, nil
		}
	}
	identity.Revision++
	encrypted, err := s.encrypt(identity)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	_, err = s.database.Model(table).Ctx(ctx).Where(do.CustomerLoginContact{TenantId: 0, UserId: profile.ID}).Data(do.CustomerLoginContact{EmailDigest: s.digest(customerlogincap.Email, profile.Email), PhoneDigest: s.digest(customerlogincap.Phone, profile.Phone), IdentityCiphertext: encrypted, Revision: identity.Revision}).Update()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return identity, nil
}
func (s *serviceImpl) Observe(ctx context.Context, token string) (*customerlogincap.Identity, error) {
	profile, err := s.profile(ctx, 0, token)
	if err != nil {
		return nil, err
	}
	var result *customerlogincap.Identity
	err = s.lock(ctx, profile.ID, func(tx context.Context, row *entity.CustomerLoginContact) error {
		current, readErr := s.profile(tx, 0, token)
		if readErr != nil {
			return readErr
		}
		if current.ID != profile.ID {
			return bizerr.NewCode(CodeInvalid)
		}
		owner, readErr := s.customer(tx, current.ID)
		if readErr != nil {
			return readErr
		}
		result, readErr = s.save(tx, current, row, owner)
		return readErr
	})
	return result, err
}
func (s *serviceImpl) RefreshCurrent(ctx context.Context) error {
	current := s.bizctx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 || current.IsImpersonation {
		return bizerr.NewCode(CodeInvalid)
	}
	return s.lock(ctx, current.UserID, func(tx context.Context, row *entity.CustomerLoginContact) error {
		owner, err := s.customer(tx, current.UserID)
		if err != nil {
			return err
		}
		profile, err := s.profile(tx, current.UserID, "")
		if err != nil {
			return err
		}
		_, err = s.save(tx, profile, row, owner)
		return err
	})
}

// RefreshUser records a successful original-route mutation inside its transaction.
// Public contact projections read on another connection still show the old
// committed value, so the already-validated original patch is merged explicitly.
func (s *serviceImpl) RefreshUser(ctx context.Context, userID int, patch customerlogincap.Patch) error {
	current := s.bizctx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 {
		return bizerr.NewCode(CodeInvalid)
	}
	owner, err := s.customer(ctx, userID)
	if err != nil {
		if bizerr.Is(err, CodeInvalid) {
			return nil
		}
		return err
	} // Non-customer host users have no customer login projection.
	return s.lock(ctx, userID, func(tx context.Context, row *entity.CustomerLoginContact) error {
		profile := &hostProfile{ID: userID}
		if userID == current.UserID {
			read, readErr := s.profile(tx, userID, "")
			if readErr != nil {
				return readErr
			}
			profile = read
		} else {
			info, readErr := s.users.Get(tx, usercap.UserID(strconv.Itoa(userID)))
			if readErr != nil {
				return readErr
			}
			if info == nil || string(info.TenantID) != "0" {
				return bizerr.NewCode(CodeInvalid)
			}
			profile.Username = info.Username
			if row.IdentityCiphertext != "" {
				old, decodeErr := s.decrypt(row.IdentityCiphertext)
				if decodeErr != nil {
					return decodeErr
				}
				profile.Email, profile.Phone = old.Email, old.Phone
			}
		}
		if patch.Email != nil {
			profile.Email = *patch.Email
		}
		if patch.Phone != nil {
			profile.Phone = *patch.Phone
		}
		if patch.Username != nil {
			profile.Username = *patch.Username
		}
		_, writeErr := s.save(tx, profile, row, owner)
		return writeErr
	})
}

// RecordProvisioned joins the caller's existing account-creation transaction. It
// cannot create customer ownership or write host identities.
func (s *serviceImpl) RecordProvisioned(ctx context.Context, userID int, username, email, phone string) error {
	owner, err := s.customer(ctx, userID)
	if err != nil {
		return err
	}
	return s.lock(ctx, userID, func(tx context.Context, row *entity.CustomerLoginContact) error {
		_, err := s.save(tx, &hostProfile{ID: userID, Username: username, Email: email, Phone: phone}, row, owner)
		return err
	})
}
