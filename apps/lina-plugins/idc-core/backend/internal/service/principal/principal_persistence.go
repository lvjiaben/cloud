// This file implements read-only audience lookup and transactional staff binding with generated DAOs.

package principal

import (
	"context"
	"crypto/rand"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// daoPersistence implements principal persistence with plugin-owned DAOs.
type daoPersistence struct {
	now    func() time.Time
	random io.Reader
	audit  idccap.BusinessAuditWriter
}

// newDAOPersistence creates the stateless production persistence implementation.
func newDAOPersistence(auditWriter ...idccap.BusinessAuditWriter) persistence {
	var writer idccap.BusinessAuditWriter
	if len(auditWriter) > 0 {
		writer = auditWriter[0]
	}
	return &daoPersistence{now: time.Now, random: rand.Reader, audit: writer}
}

// current reads one audience and identity binding, then rejects every partial
// or cross-audience combination instead of treating corrupt state as unbound.
func (p *daoPersistence) current(ctx context.Context, userID int) (*Audience, error) {
	audience, err := p.loadAudience(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	binding, err := p.loadIdentityBinding(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	if audience == nil && binding == nil {
		return nil, nil
	}
	if audience == nil {
		return nil, bizerr.NewCode(CodeAudienceInconsistent)
	}
	switch audience.Code {
	case AudienceStaff:
		if binding != nil {
			return nil, bizerr.NewCode(CodeAudienceInconsistent)
		}
	case AudienceCustomer:
		if binding == nil || AudienceStatus(binding.StatusCode) != audience.Status {
			return nil, bizerr.NewCode(CodeAudienceInconsistent)
		}
		customer, err := p.loadCustomerSubject(ctx, binding.CustomerNo, false)
		if err != nil {
			return nil, err
		}
		if customer == nil ||
			!validCustomerType(customer.CustomerTypeCode) ||
			!validCustomerStatus(customer.StatusCode) {
			return nil, bizerr.NewCode(CodeAudienceInconsistent)
		}
		audience.CustomerNo = binding.CustomerNo
	default:
		return nil, bizerr.NewCode(CodeAudienceInconsistent)
	}
	return audience, nil
}

// ensureStaff atomically creates an active staff audience or reuses the same binding.
func (p *daoPersistence) ensureStaff(ctx context.Context, userID int) (*Audience, error) {
	var result *Audience
	err := dao.PrincipalAudience.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		audience, err := p.loadAudience(ctx, userID, true)
		if err != nil {
			return err
		}
		binding, err := p.loadIdentityBinding(ctx, userID, true)
		if err != nil {
			return err
		}
		if binding != nil {
			return bizerr.NewCode(CodeAudienceConflict)
		}
		if audience == nil {
			insertResult, err := dao.PrincipalAudience.Ctx(ctx).Data(do.PrincipalAudience{
				TenantId:     idccap.DefaultTenantID,
				SysUserId:    userID,
				AudienceCode: string(AudienceStaff),
				StatusCode:   string(AudienceStatusActive),
				CreatedBy:    userID,
				UpdatedBy:    userID,
			}).InsertIgnore()
			if err != nil {
				return gerror.Wrap(err, "insert staff audience")
			}
			if _, err = insertResult.RowsAffected(); err != nil {
				return gerror.Wrap(err, "read inserted staff audience result")
			}
			// A concurrent authorized request may have inserted the same staff
			// audience while this transaction was waiting on the unique key.
			// Reload and validate it instead of turning an idempotent race into a
			// conflict. A customer or revoked winner still fails below.
			audience, err = p.loadAudience(ctx, userID, true)
			if err != nil {
				return err
			}
		}
		if audience == nil || audience.Code != AudienceStaff || audience.Status != AudienceStatusActive {
			return bizerr.NewCode(CodeAudienceConflict)
		}
		result = audience
		return nil
	})
	return result, err
}

// loadAudience reads one user audience and optionally locks it for mutation.
func (p *daoPersistence) loadAudience(ctx context.Context, userID int, lock bool) (*Audience, error) {
	var row *entity.PrincipalAudience
	model := dao.PrincipalAudience.Ctx(ctx).Where(do.PrincipalAudience{
		TenantId:  idccap.DefaultTenantID,
		SysUserId: userID,
	})
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load principal audience")
	}
	if row == nil {
		return nil, nil
	}
	return &Audience{
		UserID:  row.SysUserId,
		Code:    AudienceCode(row.AudienceCode),
		Status:  AudienceStatus(row.StatusCode),
		Version: row.Version,
	}, nil
}

// loadIdentityBinding reads one user-to-customer binding and optionally locks it.
func (p *daoPersistence) loadIdentityBinding(
	ctx context.Context,
	userID int,
	lock bool,
) (*entity.CustomerIdentityBinding, error) {
	var row *entity.CustomerIdentityBinding
	model := dao.CustomerIdentityBinding.Ctx(ctx).Where(do.CustomerIdentityBinding{
		TenantId:  idccap.DefaultTenantID,
		SysUserId: userID,
	})
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load customer identity binding")
	}
	return row, nil
}

// loadCustomerSubject reads one customer subject and optionally locks it for
// a replay consistency check. Missing rows are reported to callers as nil.
func (p *daoPersistence) loadCustomerSubject(
	ctx context.Context,
	customerNo string,
	lock bool,
) (*entity.CustomerSubject, error) {
	var row *entity.CustomerSubject
	model := dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{
		TenantId:   idccap.DefaultTenantID,
		CustomerNo: customerNo,
	})
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load customer subject")
	}
	return row, nil
}
