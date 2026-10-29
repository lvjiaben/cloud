// Package notificationpolicy implements the core-owned current-contact and
// pre-send facts consumed by the IDC notification source plugin.
package notificationpolicy

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/notificationcap"
)

var notificationIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)

type contactResolver struct{ database gdb.DB }

type contactRow struct {
	Ordinal           int    `orm:"ordinal"`
	CustomerAvailable bool   `orm:"customer_available"`
	ContactActive     bool   `orm:"contact_active"`
	ActivationAllowed bool   `orm:"activation_allowed"`
	PreferredLocale   string `orm:"preferred_locale"`
	Email             []byte `orm:"email"`
	EmailVerified     bool   `orm:"email_verified"`
	PhoneCountryCode  []byte `orm:"phone_country_code"`
	Phone             []byte `orm:"phone"`
	PhoneVerified     bool   `orm:"phone_verified"`
	ContactVersion    int64  `orm:"contact_version"`
	PreferencePresent bool   `orm:"preference_present"`
	PreferenceEnabled bool   `orm:"preference_enabled"`
	PreferenceVersion int64  `orm:"preference_version"`
}

var _ notificationcap.ContactResolver = (*contactResolver)(nil)

func NewContactResolver(database gdb.DB) (notificationcap.ContactResolver, error) {
	if database == nil {
		return nil, gerror.New("notification contact resolver requires a database")
	}
	return &contactResolver{database: database}, nil
}

func (s *contactResolver) ResolveBatch(
	ctx context.Context,
	requests []notificationcap.ContactRequest,
	consume notificationcap.ContactConsumer,
) error {
	normalized, err := normalizeContactRequests(requests)
	if err != nil {
		return err
	}
	if ctx == nil || ctx.Err() != nil || consume == nil {
		return gerror.New("notification contact resolution is unavailable")
	}
	values := make([]string, len(normalized))
	arguments := make([]any, 0, len(normalized)*5)
	for index, request := range normalized {
		values[index] = `(?, ?, ?, ?, ?)`
		arguments = append(arguments, index, request.CustomerNo, request.ContactNo,
			string(request.Channel), string(request.Topic))
	}
	query := `
WITH requested(ordinal, customer_no, contact_no, channel_code, topic_code) AS (
    VALUES ` + strings.Join(values, ",") + `
)
SELECT r.ordinal,
       (customer.customer_no IS NOT NULL AND (
          customer.status_code = 'active' OR
          (customer.status_code = 'pending_activation' AND r.topic_code = 'security' AND EXISTS (
             SELECT 1
             FROM plugin_idc_core_customer_activation_invitation invitation
             WHERE invitation.tenant_id = 0
               AND invitation.customer_no = r.customer_no
               AND invitation.contact_no = r.contact_no
               AND invitation.status_code = 'active'
               AND invitation.expires_at > CURRENT_TIMESTAMP
          ))
       )) AS customer_available,
       (contact.contact_no IS NOT NULL) AS contact_active,
       (customer.status_code = 'pending_activation' AND r.topic_code = 'security' AND EXISTS (
          SELECT 1
          FROM plugin_idc_core_customer_activation_invitation invitation
          WHERE invitation.tenant_id = 0
            AND invitation.customer_no = r.customer_no
            AND invitation.contact_no = r.contact_no
            AND invitation.status_code = 'active'
            AND invitation.expires_at > CURRENT_TIMESTAMP
       )) AS activation_allowed,
       coalesce(contact.preferred_locale, '') AS preferred_locale,
       coalesce(contact.email, '') AS email,
       (contact.email_verified_at IS NOT NULL) AS email_verified,
       coalesce(contact.phone_country_code, '') AS phone_country_code,
       coalesce(contact.phone, '') AS phone,
       (contact.phone_verified_at IS NOT NULL) AS phone_verified,
       coalesce(contact.version, 0) AS contact_version,
       (preference.id IS NOT NULL) AS preference_present,
       coalesce(preference.enabled, false) AS preference_enabled,
       coalesce(preference.version, 0) AS preference_version
FROM requested r
LEFT JOIN plugin_idc_core_customer_subject customer
  ON customer.tenant_id = 0 AND customer.customer_no = r.customer_no
 AND customer.status_code IN ('active', 'pending_activation')
LEFT JOIN plugin_idc_core_customer_contact contact
  ON contact.tenant_id = 0 AND contact.customer_no = r.customer_no
 AND contact.contact_no = r.contact_no AND contact.status_code = 'active'
LEFT JOIN plugin_idc_core_customer_contact_preference preference
  ON preference.tenant_id = 0 AND preference.customer_no = r.customer_no
 AND preference.contact_no = r.contact_no AND preference.channel_code = r.channel_code
 AND preference.topic_code = r.topic_code
ORDER BY r.ordinal ASC`
	rows := make([]contactRow, 0, len(normalized))
	if err = s.database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return gerror.Wrap(err, "resolve notification contacts")
	}
	defer clearContactRows(rows)
	if len(rows) != len(normalized) {
		return gerror.New("notification contact resolution is unavailable")
	}
	for index := range normalized {
		if rows[index].Ordinal != index || !rows[index].CustomerAvailable ||
			!rows[index].ContactActive || rows[index].ContactVersion < 1 {
			return gerror.New("notification contact is unavailable")
		}
		projection, destination, projectionErr := projectContact(normalized[index], rows[index])
		if projectionErr != nil {
			return projectionErr
		}
		if consumeErr := consume(index, projection, destination); consumeErr != nil {
			clear(destination)
			return consumeErr
		}
		clear(destination)
	}
	return nil
}

func projectContact(
	request notificationcap.ContactRequest,
	row contactRow,
) (notificationcap.ContactProjection, []byte, error) {
	projection := notificationcap.ContactProjection{
		CustomerNo: request.CustomerNo, ContactNo: request.ContactNo,
		Channel: request.Channel, Topic: request.Topic, PreferredLocale: row.PreferredLocale,
		ContactVersion: row.ContactVersion, PreferencePresent: row.PreferencePresent,
		PreferenceEnabled: row.PreferenceEnabled, PreferenceVersion: row.PreferenceVersion,
	}
	var destination []byte
	switch request.Channel {
	case notificationcap.ChannelEmail:
		if (!row.EmailVerified && !row.ActivationAllowed) || len(row.Email) == 0 {
			return notificationcap.ContactProjection{}, nil, gerror.New("notification contact is unavailable")
		}
		projection.AddressOwner = notificationcap.AddressOwnerCoreContact
		destination = append(destination, row.Email...)
	case notificationcap.ChannelSMS:
		if (!row.PhoneVerified && !row.ActivationAllowed) || len(row.Phone) == 0 {
			return notificationcap.ContactProjection{}, nil, gerror.New("notification contact is unavailable")
		}
		projection.AddressOwner = notificationcap.AddressOwnerCoreContact
		destination = append(destination, row.PhoneCountryCode...)
		destination = append(destination, row.Phone...)
	default:
		return notificationcap.ContactProjection{}, nil, gerror.New("notification contact is unavailable")
	}
	return projection, destination, nil
}

func normalizeContactRequests(
	requests []notificationcap.ContactRequest,
) ([]notificationcap.ContactRequest, error) {
	if len(requests) == 0 || len(requests) > notificationcap.MaximumContactBatch {
		return nil, gerror.New("notification contact request batch is invalid")
	}
	normalized := append([]notificationcap.ContactRequest(nil), requests...)
	seen := make(map[string]struct{}, len(normalized))
	for index := range normalized {
		normalized[index].CustomerNo = strings.TrimSpace(normalized[index].CustomerNo)
		normalized[index].ContactNo = strings.TrimSpace(normalized[index].ContactNo)
		if !notificationIdentifierPattern.MatchString(normalized[index].CustomerNo) ||
			!notificationIdentifierPattern.MatchString(normalized[index].ContactNo) ||
			!validChannel(normalized[index].Channel) || !validTopic(normalized[index].Topic) {
			return nil, gerror.New("notification contact request is invalid")
		}
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s", normalized[index].CustomerNo,
			normalized[index].ContactNo, normalized[index].Channel, normalized[index].Topic)
		if _, duplicate := seen[key]; duplicate {
			return nil, gerror.New("notification contact request is duplicated")
		}
		seen[key] = struct{}{}
	}
	return normalized, nil
}

func validChannel(value notificationcap.Channel) bool {
	switch value {
	case notificationcap.ChannelEmail, notificationcap.ChannelSMS:
		return true
	default:
		return false
	}
}

func validTopic(value notificationcap.Topic) bool {
	switch value {
	case notificationcap.TopicAccount, notificationcap.TopicBilling, notificationcap.TopicService,
		notificationcap.TopicSecurity, notificationcap.TopicSupport, notificationcap.TopicMarketing:
		return true
	default:
		return false
	}
}

func clearContactRows(rows []contactRow) {
	for index := range rows {
		clear(rows[index].Email)
		clear(rows[index].PhoneCountryCode)
		clear(rows[index].Phone)
	}
}
