// This file manages sales ownership and manual referral assignments while
// preserving previous relationships and never writing host user data.
package salesmanagement

import (
	"context"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-core/pkg/statusflag"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// Ownership is the current staff and referral relationship for one customer.
type Ownership struct {
	CustomerNo, CustomerName, CustomerCode, ReferrerCustomerNo, ReferrerName, ReferrerCode, StaffName string
	StaffUserID                                                                                       int
	Version                                                                                           int64
}

// OwnershipPage contains the current page and exact customer count.
type OwnershipPage struct {
	Items []Ownership
	Total int
}

// OwnershipInput replaces both relationships; zero staff and empty referrer clear them.
type OwnershipInput struct {
	CustomerNo, ReferrerCustomerNo string
	StaffUserID                    int
	ExpectedVersion                int64
}

func (s *serviceImpl) ListOwnership(ctx context.Context, input PageInput) (OwnershipPage, error) {
	result := OwnershipPage{Items: []Ownership{}}
	if _, err := s.authorize(ctx, permissionOverview, true); err != nil {
		return result, err
	}
	input, err := normalizePage(input)
	if err != nil {
		return result, err
	}
	query := s.database.Model(dao.CustomerSubject.Table()+" c").Ctx(ctx).
		LeftJoin(dao.SalesCustomerAssignment.Table()+" a", "a.tenant_id=c.tenant_id AND a.customer_no=c.customer_no AND a.status_code='active'").
		LeftJoin(dao.SalesGroupMember.Table()+" m", "m.tenant_id=a.tenant_id AND m.membership_no=a.membership_no").
		LeftJoin(dao.SalesCustomerReferrer.Table()+" r", "r.tenant_id=c.tenant_id AND r.customer_no=c.customer_no AND NOT EXISTS (SELECT 1 FROM plugin_idc_core_sales_customer_referrer newer WHERE newer.customer_no=r.customer_no AND newer.version>r.version)").
		LeftJoin(dao.CustomerSubject.Table()+" ref", "ref.tenant_id=r.tenant_id AND ref.customer_no=r.referrer_customer_no").Where("c.tenant_id", 0)
	if input.Search != "" {
		query = query.Where("c.display_name LIKE ? OR CAST(c.customer_code AS TEXT) LIKE ?", "%"+input.Search+"%", "%"+input.Search+"%")
	}
	result.Total, err = query.Clone().Count()
	if err != nil {
		return result, managementError(err)
	}
	rows, err := query.Fields("c.customer_no,c.customer_code,c.display_name,m.staff_user_id,r.referrer_customer_no,ref.display_name AS referrer_name,ref.customer_code AS referrer_code,r.version").OrderDesc("c.id").Page(input.Page, input.PageSize).All()
	if err != nil {
		return result, managementError(err)
	}
	ids := []usercap.UserID{}
	for _, row := range rows {
		if id := row["staff_user_id"].Int(); id > 0 {
			ids = append(ids, usercap.UserID(strconv.Itoa(id)))
		}
	}
	names := map[int]string{}
	if len(ids) > 0 {
		users, err := s.users.BatchGet(ctx, ids)
		if err != nil {
			return result, managementError(err)
		}
		if users != nil {
			for id, user := range users.Items {
				if user != nil {
					number, _ := strconv.Atoi(string(id))
					names[number] = user.Nickname
					if names[number] == "" {
						names[number] = user.Username
					}
				}
			}
		}
	}
	for _, row := range rows {
		staff := row["staff_user_id"].Int()
		result.Items = append(result.Items, Ownership{CustomerNo: row["customer_no"].String(), CustomerCode: row["customer_code"].String(), CustomerName: row["display_name"].String(), StaffUserID: staff, StaffName: names[staff], ReferrerCustomerNo: row["referrer_customer_no"].String(), ReferrerName: row["referrer_name"].String(), ReferrerCode: row["referrer_code"].String(), Version: row["version"].Int64()})
	}
	return result, nil
}
func (s *serviceImpl) SaveOwnership(ctx context.Context, input OwnershipInput) (Ownership, error) {
	actor, err := s.authorize(ctx, permissionAssignment, true)
	if err != nil {
		return Ownership{}, err
	}
	if input.CustomerNo == "" || input.CustomerNo == input.ReferrerCustomerNo || input.StaffUserID < 0 {
		return Ownership{}, bizerr.NewCode(codeInvalid)
	}
	if err := s.validateOwnershipStaff(ctx, input.StaffUserID); err != nil {
		return Ownership{}, err
	}
	var result Ownership
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var customer *entity.CustomerSubject
		if err := s.database.Model(dao.CustomerSubject.Table()).Ctx(ctx).Where(do.CustomerSubject{TenantId: 0, CustomerNo: input.CustomerNo}).LockUpdate().Scan(&customer); err != nil {
			return err
		}
		if customer == nil {
			return bizerr.NewCode(codeInvalid)
		}
		// Effective time starts after this customer's prior ownership write commits.
		now := time.Now().UTC()
		if input.ReferrerCustomerNo != "" {
			count, err := s.database.Model(dao.CustomerSubject.Table()).Ctx(ctx).Where(do.CustomerSubject{TenantId: 0, CustomerNo: input.ReferrerCustomerNo, StatusCode: statusActive}).Count()
			if err != nil {
				return err
			}
			if count != 1 {
				return bizerr.NewCode(codeInvalid)
			}
		}
		var prior *entity.SalesCustomerReferrer
		if err := s.database.Model(dao.SalesCustomerReferrer.Table()).Ctx(ctx).Where(do.SalesCustomerReferrer{TenantId: 0, CustomerNo: input.CustomerNo}).OrderDesc("version").Limit(1).Scan(&prior); err != nil {
			return err
		}
		version := int64(1)
		if prior != nil {
			if prior.Version != input.ExpectedVersion {
				return bizerr.NewCode(codeConflict)
			}
			version = prior.Version + 1
		} else if input.ExpectedVersion != 0 {
			return bizerr.NewCode(codeConflict)
		}
		var current *entity.SalesCustomerAssignment
		if err := s.database.Model(dao.SalesCustomerAssignment.Table()).Ctx(ctx).Where(do.SalesCustomerAssignment{TenantId: 0, CustomerNo: input.CustomerNo, StatusCode: statusActive}).LockUpdate().Scan(&current); err != nil {
			return err
		}
		sameStaff := false
		if current != nil && input.StaffUserID > 0 {
			count, err := s.database.Model(dao.SalesGroupMember.Table()).Ctx(ctx).Where(do.SalesGroupMember{TenantId: 0, MembershipNo: current.MembershipNo, StaffUserId: input.StaffUserID, StatusCode: statusActive}).Count()
			if err != nil {
				return err
			}
			sameStaff = count == 1
		}
		if current != nil && !sameStaff {
			if current.ProtectionExpiresAt != nil && now.Before(*current.ProtectionExpiresAt) {
				return bizerr.NewCode(codeConflict)
			}
			if _, err := s.database.Model(dao.SalesCustomerAssignment.Table()).Ctx(ctx).Where(do.SalesCustomerAssignment{TenantId: 0, AssignmentNo: current.AssignmentNo}).Data(do.SalesCustomerAssignment{StatusCode: statusEnded, EndedAt: &now}).Update(); err != nil {
				return err
			}
		}
		if input.StaffUserID > 0 && !sameStaff {
			// The default team is an internal container, not a separate operator workflow.
			if _, err := s.database.Model(dao.SalesGroup.Table()).Ctx(ctx).Data(do.SalesGroup{TenantId: 0, GroupNo: "sales_default", GroupCode: "sales_default", NameZhCn: "销售团队", NameEnUs: "Sales team", StatusCode: statusActive, Version: 1}).OnConflict("group_no").InsertIgnore(); err != nil {
				return err
			}
			var member *entity.SalesGroupMember
			if err := s.database.Model(dao.SalesGroupMember.Table()).Ctx(ctx).Where(do.SalesGroupMember{TenantId: 0, GroupNo: "sales_default", StaffUserId: input.StaffUserID, StatusCode: statusActive}).Scan(&member); err != nil {
				return err
			}
			membership := "SGM_" + guid.S()
			if member == nil {
				if _, err := s.database.Model(dao.SalesGroupMember.Table()).Ctx(ctx).Data(do.SalesGroupMember{TenantId: 0, MembershipNo: membership, GroupNo: "sales_default", StaffUserId: input.StaffUserID, RoleCode: "member", StatusCode: statusActive, EffectiveAt: &now}).Insert(); err != nil {
					return err
				}
			} else {
				membership = member.MembershipNo
			}
			if _, err := s.database.Model(dao.SalesCustomerAssignment.Table()).Ctx(ctx).Data(do.SalesCustomerAssignment{TenantId: 0, AssignmentNo: "SAS_" + guid.S(), CustomerNo: input.CustomerNo, MembershipNo: membership, EffectiveAt: &now, StatusCode: statusActive, SourceCode: "manual"}).Insert(); err != nil {
				return err
			}
		}
		referral := do.SalesCustomerReferrer{TenantId: 0, CustomerNo: input.CustomerNo, Version: version, EffectiveAt: &now, CreatedBy: actor}
		if input.ReferrerCustomerNo != "" {
			referral.ReferrerCustomerNo = input.ReferrerCustomerNo
		}
		if _, err := s.database.Model(dao.SalesCustomerReferrer.Table()).Ctx(ctx).Data(referral).Insert(); err != nil {
			return err
		}
		result = Ownership{CustomerNo: input.CustomerNo, CustomerCode: strconv.FormatInt(customer.CustomerCode, 10), CustomerName: customer.DisplayName, StaffUserID: input.StaffUserID, ReferrerCustomerNo: input.ReferrerCustomerNo, Version: version}
		return s.auditChange(ctx, permissionAssignment, "ownership", input.CustomerNo, input.CustomerNo, version, current, result)
	})
	return result, managementError(err)
}

func (s *serviceImpl) validateOwnershipStaff(ctx context.Context, staffUserID int) error {
	if staffUserID > 0 {
		user, err := s.users.Get(ctx, usercap.UserID(strconv.Itoa(staffUserID)))
		if err != nil || user == nil || user.Status != statusflag.EnabledValue || string(user.TenantID) != "0" {
			return bizerr.NewCode(codeInvalid)
		}
		count, err := s.database.Model(dao.PrincipalAudience.Table()).Ctx(ctx).Where(do.PrincipalAudience{TenantId: 0, SysUserId: staffUserID, AudienceCode: "staff", StatusCode: statusActive}).Count()
		if err != nil {
			return managementError(err)
		}
		if count != 1 {
			return bizerr.NewCode(codeInvalid)
		}
	}
	return nil
}
