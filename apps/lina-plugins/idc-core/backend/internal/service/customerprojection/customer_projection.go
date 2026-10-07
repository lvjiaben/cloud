// Package customerprojection loads bounded, display-safe customer summaries
// after the owning domain has already applied staff permission and data scope.
package customerprojection

import (
	"context"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

type scanner interface {
	GetScan(context.Context, any, string, ...any) error
}

// Summary contains only fields that may be shown in staff business pages.
type Summary struct {
	CustomerCode string
	DisplayName  string
	PrimaryPhone string
}

type row struct {
	CustomerNo       string `orm:"customer_no"`
	CustomerCode     int64  `orm:"customer_code"`
	DisplayName      string `orm:"display_name"`
	PhoneCountryCode string `orm:"phone_country_code"`
	Phone            string `orm:"phone"`
}

// Load returns one fixed-query projection for the distinct customers already
// present in an authorized page or detail response.
func Load(ctx context.Context, database scanner, customerNos []string) (map[string]Summary, error) {
	distinct := make([]string, 0, len(customerNos))
	seen := make(map[string]struct{}, len(customerNos))
	for _, customerNo := range customerNos {
		customerNo = strings.TrimSpace(customerNo)
		if customerNo == "" {
			return nil, gerror.New("customer projection input is invalid")
		}
		if _, ok := seen[customerNo]; ok {
			continue
		}
		seen[customerNo] = struct{}{}
		distinct = append(distinct, customerNo)
	}
	result := make(map[string]Summary, len(distinct))
	if len(distinct) == 0 {
		return result, nil
	}
	var rows []row
	err := database.GetScan(ctx, &rows, `
SELECT subject.customer_no,subject.customer_code,subject.display_name,
       COALESCE(contact.phone_country_code,'') AS phone_country_code,
       COALESCE(contact.phone,'') AS phone
FROM plugin_idc_core_customer_subject subject
LEFT JOIN LATERAL (
  SELECT value.phone_country_code,value.phone
  FROM plugin_idc_core_customer_contact value
  JOIN plugin_idc_core_customer_contact_role role
    ON role.tenant_id=value.tenant_id AND role.customer_no=value.customer_no
   AND role.contact_no=value.contact_no AND role.role_code='primary'
   AND role.status_code='active'
  WHERE value.tenant_id=0 AND value.customer_no=subject.customer_no
    AND value.status_code='active'
  ORDER BY value.id LIMIT 1
) contact ON TRUE
WHERE subject.tenant_id=0 AND subject.customer_no IN(?)`, distinct)
	if err != nil {
		return nil, gerror.Wrap(err, "load display-safe customer projection")
	}
	for _, value := range rows {
		if value.CustomerNo == "" || value.CustomerCode < 10000 || strings.TrimSpace(value.DisplayName) == "" {
			return nil, gerror.New("customer projection is incomplete")
		}
		result[value.CustomerNo] = Summary{
			CustomerCode: strconv.FormatInt(value.CustomerCode, 10),
			DisplayName:  strings.TrimSpace(value.DisplayName),
			PrimaryPhone: value.PhoneCountryCode + value.Phone,
		}
	}
	if len(result) != len(distinct) {
		return nil, gerror.New("customer projection is incomplete")
	}
	return result, nil
}
