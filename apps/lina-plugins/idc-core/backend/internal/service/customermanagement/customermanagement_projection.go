// This file loads customer list decorations in a fixed number of batch queries.

package customermanagement

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

type batchProjection struct {
	codeByCustomer         map[string]string
	phoneByCustomer        map[string]string
	verificationByCustomer map[string]customer.VerificationStatus
	levelByCustomer        map[string]*LevelSummary
	tagCountByCustomer     map[string]int
}

type tagCountRow struct {
	CustomerNo string `orm:"customer_no"`
	TagCount   int    `orm:"tag_count"`
}

func loadBatchProjection(ctx context.Context, rows []*entity.CustomerSubject) (batchProjection, error) {
	projection := batchProjection{
		codeByCustomer:         make(map[string]string, len(rows)),
		phoneByCustomer:        make(map[string]string, len(rows)),
		verificationByCustomer: make(map[string]customer.VerificationStatus, len(rows)),
		levelByCustomer:        make(map[string]*LevelSummary, len(rows)),
		tagCountByCustomer:     make(map[string]int, len(rows)),
	}
	if len(rows) == 0 {
		return projection, nil
	}
	individualNos := make([]string, 0, len(rows))
	enterpriseNos := make([]string, 0, len(rows))
	customerNos := make([]string, 0, len(rows))
	for _, row := range rows {
		if row == nil || !validPersistedCustomerNo(row.CustomerNo) {
			return batchProjection{}, gerror.New("customer management batch input is inconsistent")
		}
		customerNos = append(customerNos, row.CustomerNo)
		projection.tagCountByCustomer[row.CustomerNo] = 0
		switch customer.Type(row.CustomerTypeCode) {
		case customer.TypeIndividual:
			individualNos = append(individualNos, row.CustomerNo)
		case customer.TypeEnterprise:
			enterpriseNos = append(enterpriseNos, row.CustomerNo)
		default:
			return batchProjection{}, gerror.New("customer management customer type is inconsistent")
		}
	}
	if err := loadVerificationProjection(ctx, individualNos, enterpriseNos, projection.verificationByCustomer); err != nil {
		return batchProjection{}, err
	}
	if err := loadLevelProjection(ctx, customerNos, projection.levelByCustomer); err != nil {
		return batchProjection{}, err
	}
	if err := loadTagCountProjection(ctx, customerNos, projection.tagCountByCustomer); err != nil {
		return batchProjection{}, err
	}
	if err := loadCustomerIdentityProjection(ctx, customerNos, &projection); err != nil {
		return batchProjection{}, err
	}
	return projection, nil
}

type customerIdentityProjectionRow struct {
	CustomerNo       string `orm:"customer_no"`
	CustomerCode     string `orm:"customer_code"`
	PhoneCountryCode string `orm:"phone_country_code"`
	Phone            string `orm:"phone"`
}

// loadCustomerIdentityProjection reads short numbers and primary phones in one bounded query.
func loadCustomerIdentityProjection(ctx context.Context, customerNos []string, projection *batchProjection) error {
	var rows []customerIdentityProjectionRow
	err := dao.CustomerSubject.Ctx(ctx).
		LeftJoin(dao.CustomerContact.Table()+" contact", "contact.customer_no="+dao.CustomerSubject.Table()+".customer_no AND contact.tenant_id=0 AND contact.status_code='active' AND EXISTS (SELECT 1 FROM "+dao.CustomerContactRole.Table()+" contact_role WHERE contact_role.tenant_id=0 AND contact_role.customer_no=contact.customer_no AND contact_role.contact_no=contact.contact_no AND contact_role.role_code='primary' AND contact_role.status_code='active')").
		Fields(dao.CustomerSubject.Table()+".customer_no", dao.CustomerSubject.Table()+".customer_code", "COALESCE(contact.phone_country_code,'') AS phone_country_code", "COALESCE(contact.phone,'') AS phone").
		WhereIn(dao.CustomerSubject.Table()+".customer_no", customerNos).
		Scan(&rows)
	if err != nil {
		return gerror.Wrap(err, "load management customer short numbers and phones")
	}
	for _, row := range rows {
		if row.CustomerNo == "" || row.CustomerCode == "" {
			return gerror.New("customer identity projection is inconsistent")
		}
		projection.codeByCustomer[row.CustomerNo] = row.CustomerCode
		projection.phoneByCustomer[row.CustomerNo] = row.PhoneCountryCode + row.Phone
	}
	if len(projection.codeByCustomer) != len(customerNos) {
		return gerror.New("customer identity projection is incomplete")
	}
	return nil
}

func loadVerificationProjection(
	ctx context.Context,
	individualNos []string,
	enterpriseNos []string,
	destination map[string]customer.VerificationStatus,
) error {
	if len(individualNos) > 0 {
		columns := dao.CustomerIndividualProfile.Columns()
		var profiles []*entity.CustomerIndividualProfile
		if err := dao.CustomerIndividualProfile.Ctx(ctx).
			Fields(columns.CustomerNo, columns.IdentityStatusCode).
			Where(do.CustomerIndividualProfile{TenantId: idccap.DefaultTenantID}).
			WhereIn(columns.CustomerNo, individualNos).Scan(&profiles); err != nil {
			return gerror.Wrap(err, "load management individual verification statuses")
		}
		for _, profile := range profiles {
			if profile == nil || !validPersistedCustomerNo(profile.CustomerNo) ||
				!validPersistedVerification(customer.VerificationStatus(profile.IdentityStatusCode)) ||
				destination[profile.CustomerNo] != "" {
				return gerror.New("individual verification projection is inconsistent")
			}
			destination[profile.CustomerNo] = customer.VerificationStatus(profile.IdentityStatusCode)
		}
	}
	if len(enterpriseNos) > 0 {
		columns := dao.CustomerEnterpriseProfile.Columns()
		var profiles []*entity.CustomerEnterpriseProfile
		if err := dao.CustomerEnterpriseProfile.Ctx(ctx).
			Fields(columns.CustomerNo, columns.VerificationStatusCode).
			Where(do.CustomerEnterpriseProfile{TenantId: idccap.DefaultTenantID}).
			WhereIn(columns.CustomerNo, enterpriseNos).Scan(&profiles); err != nil {
			return gerror.Wrap(err, "load management enterprise verification statuses")
		}
		for _, profile := range profiles {
			if profile == nil || !validPersistedCustomerNo(profile.CustomerNo) ||
				!validPersistedVerification(customer.VerificationStatus(profile.VerificationStatusCode)) ||
				destination[profile.CustomerNo] != "" {
				return gerror.New("enterprise verification projection is inconsistent")
			}
			destination[profile.CustomerNo] = customer.VerificationStatus(profile.VerificationStatusCode)
		}
	}
	if len(destination) != len(individualNos)+len(enterpriseNos) {
		return gerror.New("verification projection is incomplete")
	}
	return nil
}

func loadLevelProjection(ctx context.Context, customerNos []string, destination map[string]*LevelSummary) error {
	assignmentColumns := dao.CustomerLevelAssignment.Columns()
	var assignments []*entity.CustomerLevelAssignment
	if err := dao.CustomerLevelAssignment.Ctx(ctx).
		Fields(assignmentColumns.CustomerNo, assignmentColumns.LevelNo).
		Where(do.CustomerLevelAssignment{TenantId: idccap.DefaultTenantID, StatusCode: "active"}).
		WhereIn(assignmentColumns.CustomerNo, customerNos).Scan(&assignments); err != nil {
		return gerror.Wrap(err, "load management customer level assignments")
	}
	levelNos := make([]string, 0, len(assignments))
	levelNoByCustomer := make(map[string]string, len(assignments))
	for _, assignment := range assignments {
		if assignment == nil || !validPersistedCustomerNo(assignment.CustomerNo) ||
			assignment.LevelNo == "" || levelNoByCustomer[assignment.CustomerNo] != "" {
			return gerror.New("customer level assignment projection is inconsistent")
		}
		levelNoByCustomer[assignment.CustomerNo] = assignment.LevelNo
		levelNos = append(levelNos, assignment.LevelNo)
	}
	if len(levelNos) == 0 {
		return nil
	}
	levelColumns := dao.CustomerLevel.Columns()
	var levels []*entity.CustomerLevel
	if err := dao.CustomerLevel.Ctx(ctx).
		Fields(levelColumns.LevelNo, levelColumns.LevelCode, levelColumns.NameZhCn,
			levelColumns.NameEnUs, levelColumns.Priority).
		Where(do.CustomerLevel{TenantId: idccap.DefaultTenantID}).
		WhereIn(levelColumns.LevelNo, levelNos).Scan(&levels); err != nil {
		return gerror.Wrap(err, "load management customer levels")
	}
	levelByNo := make(map[string]*LevelSummary, len(levels))
	for _, level := range levels {
		if level == nil || level.LevelNo == "" || level.LevelCode == "" ||
			level.NameZhCn == "" || level.NameEnUs == "" || levelByNo[level.LevelNo] != nil {
			return gerror.New("customer level projection is inconsistent")
		}
		levelByNo[level.LevelNo] = &LevelSummary{
			LevelNo: level.LevelNo, Code: level.LevelCode, NameZhCn: level.NameZhCn,
			NameEnUs: level.NameEnUs, Priority: level.Priority,
		}
	}
	for customerNo, levelNo := range levelNoByCustomer {
		level := levelByNo[levelNo]
		if level == nil {
			return gerror.New("customer level definition is missing")
		}
		copyValue := *level
		destination[customerNo] = &copyValue
	}
	return nil
}

func loadTagCountProjection(ctx context.Context, customerNos []string, destination map[string]int) error {
	columns := dao.CustomerTagAssignment.Columns()
	var counts []tagCountRow
	if err := dao.CustomerTagAssignment.Ctx(ctx).
		Fields(columns.CustomerNo, "COUNT(*) AS tag_count").
		Where(do.CustomerTagAssignment{TenantId: idccap.DefaultTenantID, StatusCode: "active"}).
		WhereIn(columns.CustomerNo, customerNos).Group(columns.CustomerNo).Scan(&counts); err != nil {
		return gerror.Wrap(err, "load management customer tag counts")
	}
	for _, count := range counts {
		if !validPersistedCustomerNo(count.CustomerNo) || count.TagCount <= 0 {
			return gerror.New("customer tag count projection is inconsistent")
		}
		if _, ok := destination[count.CustomerNo]; !ok {
			return gerror.New("customer tag count is outside the requested page")
		}
		destination[count.CustomerNo] = count.TagCount
	}
	return nil
}
