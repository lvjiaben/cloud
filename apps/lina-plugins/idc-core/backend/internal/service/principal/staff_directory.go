package principal

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
)

// StaffDirectoryState is the smallest principal-owned state needed to prove
// that one already-known host user remains a staff audience. Customer profile
// data and identity-binding details never leave this package.
type StaffDirectoryState struct {
	UserID             int
	Code               AudienceCode
	Status             AudienceStatus
	Version            int64
	HasCustomerBinding bool
}

// StaffDirectoryReader resolves only an exact caller-supplied user set. The
// implementation performs one set query and exposes no enumeration method.
type StaffDirectoryReader interface {
	ResolveBatch(context.Context, []int) (map[int]StaffDirectoryState, error)
}

type staffDirectoryReader struct{}

var _ StaffDirectoryReader = staffDirectoryReader{}

// NewStaffDirectoryReader creates the write-free principal directory used by
// the source-only notification capability.
func NewStaffDirectoryReader() StaffDirectoryReader {
	return staffDirectoryReader{}
}

type staffDirectoryRow struct {
	UserID                int    `orm:"sys_user_id"`
	AudienceCode          string `orm:"audience_code"`
	StatusCode            string `orm:"status_code"`
	Version               int64  `orm:"version"`
	CustomerBindingUserID int    `orm:"customer_binding_user_id"`
}

func (staffDirectoryReader) ResolveBatch(
	ctx context.Context,
	userIDs []int,
) (map[int]StaffDirectoryState, error) {
	result := make(map[int]StaffDirectoryState, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	var rows []staffDirectoryRow
	err := dao.PrincipalAudience.Ctx(ctx).
		As("audience").
		LeftJoin(
			dao.CustomerIdentityBinding.Table(),
			"customer_binding",
			"customer_binding.tenant_id = audience.tenant_id AND customer_binding.sys_user_id = audience.sys_user_id",
		).
		Fields(
			"audience.sys_user_id",
			"audience.audience_code",
			"audience.status_code",
			"audience.version",
			"COALESCE(customer_binding.sys_user_id, 0) AS customer_binding_user_id",
		).
		Where("audience.tenant_id", idccap.DefaultTenantID).
		WhereIn("audience.sys_user_id", userIDs).
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "resolve staff principal batch")
	}
	for _, row := range rows {
		if row.UserID <= 0 {
			return nil, gerror.New("staff principal batch returned an invalid user")
		}
		if _, exists := result[row.UserID]; exists {
			return nil, gerror.New("staff principal batch returned duplicate users")
		}
		result[row.UserID] = StaffDirectoryState{
			UserID: row.UserID, Code: AudienceCode(row.AudienceCode),
			Status: AudienceStatus(row.StatusCode), Version: row.Version,
			HasCustomerBinding: row.CustomerBindingUserID > 0,
		}
	}
	return result, nil
}
