package serveroperation

import (
	"context"
	"encoding/json"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

const (
	PermissionIPChangePolicyRead  = "idc:admin:resource:ip-change-policy:read"
	PermissionIPChangePolicyWrite = "idc:admin:resource:ip-change-policy:write"
	ipChangeQuoteTTL              = 15 * time.Minute
)

var (
	CodeIPChangeNotAccessible = bizerr.MustDefine("IDC_IP_CHANGE_NOT_ACCESSIBLE",
		"The server IP-change policy is not accessible", gcode.CodeNotAuthorized)
	CodeIPChangeInvalid = bizerr.MustDefine("IDC_IP_CHANGE_INVALID",
		"The server IP-change request is invalid", gcode.CodeInvalidParameter)
	CodeIPChangeUnavailable = bizerr.MustDefine("IDC_IP_CHANGE_UNAVAILABLE",
		"Server IP change is currently unavailable", gcode.CodeInvalidOperation)
	CodeIPChangeConflict = bizerr.MustDefine("IDC_IP_CHANGE_CONFLICT",
		"The server IP-change quote changed; reload it and try again", gcode.CodeInvalidOperation)
	CodeIPChangeFrequencyExceeded = bizerr.MustDefine("IDC_IP_CHANGE_FREQUENCY_EXCEEDED",
		"The server IP-change frequency limit has been reached", gcode.CodeInvalidOperation)
)

type IPChangePolicyInput struct {
	CurrencyCode              string
	Enabled                   bool
	FreeChangesPerWindow      int
	WindowDays                int
	MaxChangesPerDay          int
	AmountMinor               int64
	MaximumSnapshotAgeSeconds int
	ExpectedVersion           int64
}

type IPChangePolicy struct {
	PolicyNo                  string
	CurrencyCode              string
	Enabled                   bool
	FreeChangesPerWindow      int
	WindowDays                int
	MaxChangesPerDay          int
	AmountMinor               int64
	MaximumSnapshotAgeSeconds int
	Version                   int64
	PublishedAt               time.Time
}

type IPChangeQuoteInput struct {
	ServiceNo      string
	IdempotencyKey string
}

type IPChangeQuote struct {
	QuoteNo                   string
	ServiceNo                 string
	CurrencyCode              string
	IPQuantity                int
	FreeChange                bool
	AmountMinor               int64
	PolicyNo                  string
	PolicyVersion             int64
	ResourceReservationNo     string
	ResourceSnapshotNo        string
	MaximumSnapshotAgeSeconds int
	ExpiresAt                 time.Time
	Reused                    bool
}

type IPChangeCheckoutInput struct {
	QuoteNo        string
	IdempotencyKey string
	ProofToken     string
}

type IPChangeCheckout struct {
	OperationNo     string
	OrderNo         string
	InvoiceNo       string
	ServiceNo       string
	Status          string
	AmountMinor     int64
	CurrencyCode    string
	PaymentDueAt    time.Time
	PaymentRequired bool
	Reused          bool
}

type ipChangePolicyRow struct {
	PolicyNo                  string    `orm:"policy_no"`
	CurrencyCode              string    `orm:"currency_code"`
	Enabled                   bool      `orm:"enabled"`
	FreeChangesPerWindow      int       `orm:"free_changes_per_window"`
	WindowDays                int       `orm:"window_days"`
	MaxChangesPerDay          int       `orm:"max_changes_per_day"`
	AmountMinor               int64     `orm:"amount_minor"`
	MaximumSnapshotAgeSeconds int       `orm:"maximum_snapshot_age_seconds"`
	Version                   int64     `orm:"policy_version"`
	PublishedAt               time.Time `orm:"published_at"`
}

func (s *accessoryPricingImpl) ListIPChangePolicies(ctx context.Context) ([]IPChangePolicy, error) {
	if _, err := s.authorize(ctx, PermissionIPChangePolicyRead); err != nil {
		return nil, bizerr.NewCode(CodeIPChangeNotAccessible)
	}
	var rows []ipChangePolicyRow
	if err := s.database.GetScan(ctx, &rows, `SELECT policy.policy_no,policy.currency_code,
version.enabled,version.free_changes_per_window,version.window_days,version.max_changes_per_day,
version.amount_minor,version.maximum_snapshot_age_seconds,version.policy_version,version.published_at
FROM plugin_idc_core_server_ip_change_policy policy
JOIN plugin_idc_core_server_ip_change_policy_version version
 ON version.policy_no=policy.policy_no AND version.policy_version=policy.current_version
WHERE policy.tenant_id=0 ORDER BY policy.currency_code LIMIT 100`); err != nil {
		return nil, bizerr.WrapCode(err, CodeIPChangeUnavailable)
	}
	result := make([]IPChangePolicy, len(rows))
	for index := range rows {
		result[index] = projectIPChangePolicy(rows[index])
	}
	return result, nil
}

func (s *accessoryPricingImpl) PublishIPChangePolicy(ctx context.Context,
	input IPChangePolicyInput) (*IPChangePolicy, error) {
	input.CurrencyCode = strings.ToUpper(strings.TrimSpace(input.CurrencyCode))
	if !validIPChangePolicyInput(input) {
		return nil, bizerr.NewCode(CodeIPChangeInvalid)
	}
	actor, err := s.authorize(ctx, PermissionIPChangePolicyWrite)
	if err != nil {
		return nil, bizerr.NewCode(CodeIPChangeNotAccessible)
	}
	policyNo, _ := deterministicReference("ipc_", input.CurrencyCode)
	var result *IPChangePolicy
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var current *struct {
			PolicyNo string `orm:"policy_no"`
			Version  int64  `orm:"current_version"`
		}
		if loadErr := s.database.GetScan(ctx, &current, `SELECT policy_no,current_version
FROM plugin_idc_core_server_ip_change_policy
WHERE tenant_id=0 AND currency_code=? FOR UPDATE`, input.CurrencyCode); loadErr != nil {
			return loadErr
		}
		currentVersion := int64(0)
		if current != nil {
			policyNo, currentVersion = current.PolicyNo, current.Version
		}
		if currentVersion != input.ExpectedVersion {
			return bizerr.NewCode(CodeIPChangeConflict)
		}
		nextVersion := currentVersion + 1
		digest, digestErr := digestValue(struct {
			Contract, PolicyNo, Currency string
			Enabled                      bool
			Free, Window, Daily, Fresh   int
			Amount, Version              int64
		}{"server-ip-change-policy/v1", policyNo, input.CurrencyCode, input.Enabled,
			input.FreeChangesPerWindow, input.WindowDays, input.MaxChangesPerDay,
			input.MaximumSnapshotAgeSeconds, input.AmountMinor, nextVersion})
		if digestErr != nil {
			return digestErr
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if current == nil {
			if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_ip_change_policy
(tenant_id,policy_no,currency_code,current_version,created_at,updated_at)
VALUES (0,?,?,?,?,?)`, policyNo, input.CurrencyCode, nextVersion, now, now); insertErr != nil {
				return insertErr
			}
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_ip_change_policy_version
(tenant_id,policy_no,policy_version,currency_code,enabled,free_changes_per_window,window_days,
max_changes_per_day,amount_minor,maximum_snapshot_age_seconds,content_digest,published_by,published_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?)`, policyNo, nextVersion, input.CurrencyCode, input.Enabled,
			input.FreeChangesPerWindow, input.WindowDays, input.MaxChangesPerDay, input.AmountMinor,
			input.MaximumSnapshotAgeSeconds, digest, actor, now); insertErr != nil {
			return insertErr
		}
		if current != nil {
			if updateErr := execOne(ctx, s.database, `UPDATE plugin_idc_core_server_ip_change_policy
SET current_version=?,updated_at=? WHERE tenant_id=0 AND policy_no=? AND current_version=?`,
				nextVersion, now, policyNo, currentVersion); updateErr != nil {
				return bizerr.NewCode(CodeIPChangeConflict)
			}
		}
		if auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "server.ip_change.policy.publish",
			IdempotencyKey:   policyNo + ":" + strconv.FormatInt(nextVersion, 10),
			OperationCode:    "server.ip_change.policy.publish", TargetType: "server_ip_change_policy",
			TargetNo: policyNo, ResultCode: "published", ReasonCode: "staff_policy_publication",
			RiskCode: "high", RequiredPermission: PermissionIPChangePolicyWrite,
			BeforeSnapshot: map[string]any{"version": currentVersion},
			AfterSnapshot: map[string]any{"version": nextVersion, "currencyCode": input.CurrencyCode,
				"enabled": input.Enabled, "freeChangesPerWindow": input.FreeChangesPerWindow,
				"windowDays": input.WindowDays, "maxChangesPerDay": input.MaxChangesPerDay,
				"amountMinor":               input.AmountMinor,
				"maximumSnapshotAgeSeconds": input.MaximumSnapshotAgeSeconds},
		}); auditErr != nil {
			return auditErr
		}
		projected := projectIPChangePolicy(ipChangePolicyRow{PolicyNo: policyNo,
			CurrencyCode: input.CurrencyCode, Enabled: input.Enabled,
			FreeChangesPerWindow: input.FreeChangesPerWindow, WindowDays: input.WindowDays,
			MaxChangesPerDay: input.MaxChangesPerDay, AmountMinor: input.AmountMinor,
			MaximumSnapshotAgeSeconds: input.MaximumSnapshotAgeSeconds,
			Version:                   nextVersion, PublishedAt: now})
		result = &projected
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeIPChangeUnavailable)
	}
	return result, nil
}

func validIPChangePolicyInput(input IPChangePolicyInput) bool {
	if len(input.CurrencyCode) != 3 || input.ExpectedVersion < 0 ||
		input.FreeChangesPerWindow < 0 || input.FreeChangesPerWindow > 100 ||
		input.WindowDays < 1 || input.WindowDays > 365 || input.MaxChangesPerDay < 1 || input.MaxChangesPerDay > 20 ||
		input.AmountMinor < 0 || input.AmountMinor > 1000000000 ||
		input.MaximumSnapshotAgeSeconds < 60 || input.MaximumSnapshotAgeSeconds > 86400 {
		return false
	}
	for _, character := range input.CurrencyCode {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func projectIPChangePolicy(row ipChangePolicyRow) IPChangePolicy {
	return IPChangePolicy{PolicyNo: row.PolicyNo, CurrencyCode: row.CurrencyCode,
		Enabled: row.Enabled, FreeChangesPerWindow: row.FreeChangesPerWindow,
		WindowDays: row.WindowDays, MaxChangesPerDay: row.MaxChangesPerDay,
		AmountMinor: row.AmountMinor, MaximumSnapshotAgeSeconds: row.MaximumSnapshotAgeSeconds,
		Version: row.Version, PublishedAt: row.PublishedAt.UTC()}
}

type ipChangePreviewRow struct {
	ServiceNo                 string
	CustomerNo                string
	CurrencyCode              string
	PoolNo                    string
	ChannelNo                 string
	EntitlementStatusCode     string
	RuntimeStatusCode         string
	ProviderTaskStatus        string `orm:"provider_task_status_code"`
	Version                   int64
	CurrentPriceSnapshotNo    string `orm:"current_price_snapshot_no"`
	CustomerSubjectVersion    int64  `orm:"customer_subject_version"`
	PolicyNo                  string `orm:"policy_no"`
	PolicyVersion             int64  `orm:"policy_version"`
	PolicyEnabled             bool   `orm:"enabled"`
	FreeChangesPerWindow      int    `orm:"free_changes_per_window"`
	WindowDays                int    `orm:"window_days"`
	MaxChangesPerDay          int    `orm:"max_changes_per_day"`
	AmountMinor               int64  `orm:"amount_minor"`
	MaximumSnapshotAgeSeconds int    `orm:"maximum_snapshot_age_seconds"`
}

func (row *ipChangePreviewRow) service() *serviceRow {
	if row == nil {
		return nil
	}
	return &serviceRow{ServiceNo: row.ServiceNo, CustomerNo: row.CustomerNo,
		CurrencyCode: row.CurrencyCode, PoolNo: row.PoolNo, ChannelNo: row.ChannelNo,
		EntitlementStatusCode: row.EntitlementStatusCode, RuntimeStatusCode: row.RuntimeStatusCode,
		ProviderTaskStatus: row.ProviderTaskStatus, Version: row.Version,
		CurrentPriceSnapshotNo: row.CurrentPriceSnapshotNo,
		CustomerSubjectVersion: row.CustomerSubjectVersion}
}

func (s *accessoryPricingImpl) PreviewIPChangeCurrentCustomer(ctx context.Context,
	input IPChangeQuoteInput) (*IPChangeQuote, error) {
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) ||
		!validReference(input.ServiceNo, 40) || len(input.IdempotencyKey) < 8 ||
		len(input.IdempotencyKey) > 256 || input.IdempotencyKey != strings.TrimSpace(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeIPChangeInvalid)
	}
	idempotencyDigest, _ := digestValue(struct{ CustomerNo, Key string }{subject.CustomerNo, input.IdempotencyKey})
	requestDigest, _ := digestValue(struct{ CustomerNo, ServiceNo string }{subject.CustomerNo, input.ServiceNo})
	if replay, replayErr := s.loadIPChangeQuoteReplay(ctx, subject.CustomerNo, idempotencyDigest, requestDigest); replayErr != nil || replay != nil {
		return replay, replayErr
	}
	return s.buildIPChangeQuote(ctx, subject.CustomerNo, input, idempotencyDigest, requestDigest)
}

func (s *accessoryPricingImpl) buildIPChangeQuote(ctx context.Context, customerNo string,
	input IPChangeQuoteInput, idempotencyDigest, requestDigest string) (*IPChangeQuote, error) {
	row, err := s.loadIPChangePreview(ctx, customerNo, input.ServiceNo)
	if err != nil || row == nil || !row.PolicyEnabled || row.ChannelNo == "" || row.PoolNo == "" ||
		!allowedAction(row.service(), serveroperationresourcecap.ActionChangeIP) {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	directory, err := s.directory.GetAccessories(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: row.ServiceNo, ChannelNo: row.ChannelNo, PoolNo: row.PoolNo})
	if err != nil || directory == nil || directory.ServiceNo != row.ServiceNo || directory.SnapshotNo == "" ||
		directory.ExpiresAt.Before(time.Now().UTC()) || !containsAction(directory.Actions, serveroperationresourcecap.ActionChangeIP) {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	addresses, err := currentIPAddresses(directory.Items)
	if err != nil {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	oldDigest, _ := digestValue(struct {
		Domain, ServiceNo string
		Addresses         []string
	}{"server-ip-change-old-address-set/v1", row.ServiceNo, addresses})
	windowCount, dayCount, err := s.ipChangeFrequency(ctx, row.ServiceNo, row.WindowDays)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeIPChangeUnavailable)
	}
	if dayCount >= row.MaxChangesPerDay {
		return nil, bizerr.NewCode(CodeIPChangeFrequencyExceeded)
	}
	amount := row.AmountMinor
	if windowCount < row.FreeChangesPerWindow {
		amount = 0
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeIPChangeUnavailable)
	}
	expiresAt := now.Add(ipChangeQuoteTTL)
	reservationRequest := serveroperationresourcecap.IPReservationRequest{
		Route: serveroperationresourcecap.CapabilityRoute{ServiceNo: row.ServiceNo,
			ChannelNo: row.ChannelNo, PoolNo: row.PoolNo}, Quantity: len(addresses),
		MaximumCapacityAgeSeconds: row.MaximumSnapshotAgeSeconds, ExpiresAt: expiresAt,
		IdempotencyKey: "ip-change:" + idempotencyDigest,
	}
	reservationRequest.RequestDigest, err = serveroperationresourcecap.IPReservationRequestDigest(reservationRequest)
	if err != nil {
		return nil, bizerr.NewCode(CodeIPChangeInvalid)
	}
	reservation, err := s.ipCapacity.ReserveIP(ctx, reservationRequest)
	if err != nil || reservation == nil || reservation.Status != serveroperationresourcecap.IPReservationReserved ||
		reservation.Quantity != len(addresses) || reservation.ExpiresAt.Before(expiresAt) {
		return nil, bizerr.NewCode(CodeIPChangeUnavailable)
	}
	quoteNo, _ := deterministicReference("ipq_", struct{ CustomerNo, Key string }{customerNo, input.IdempotencyKey})
	quote := &IPChangeQuote{QuoteNo: quoteNo, ServiceNo: row.ServiceNo,
		CurrencyCode: row.CurrencyCode, IPQuantity: len(addresses), FreeChange: amount == 0,
		AmountMinor: amount, PolicyNo: row.PolicyNo, PolicyVersion: row.PolicyVersion,
		ResourceReservationNo: reservation.ReservationNo, ResourceSnapshotNo: reservation.CapacityObservationNo,
		MaximumSnapshotAgeSeconds: row.MaximumSnapshotAgeSeconds, ExpiresAt: expiresAt}
	persistErr := s.persistIPChangeQuote(ctx, customerNo, input.IdempotencyKey,
		idempotencyDigest, requestDigest, oldDigest, row, directory, quote, now)
	if persistErr != nil {
		_, _ = s.ipCapacity.ReleaseIP(ctx, serveroperationresourcecap.IPReservationReleaseRequest{
			ReservationNo: reservation.ReservationNo, ReasonCode: "core_quote_failed"})
		if replay, replayErr := s.loadIPChangeQuoteReplay(ctx, customerNo, idempotencyDigest, requestDigest); replayErr == nil && replay != nil {
			return replay, nil
		}
		return nil, persistErr
	}
	return quote, nil
}

func containsAction(actions []serveroperationresourcecap.Action, target serveroperationresourcecap.Action) bool {
	for _, action := range actions {
		if action == target {
			return true
		}
	}
	return false
}

func currentIPAddresses(items []serveroperationresourcecap.AccessoryItem) ([]string, error) {
	addresses := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.Kind != serveroperationresourcecap.AccessoryKindIP || item.State != "active" {
			continue
		}
		var projection struct {
			Address string `json:"address"`
		}
		if json.Unmarshal(item.Projection, &projection) != nil {
			return nil, ErrInvalidInput
		}
		address := net.ParseIP(strings.TrimSpace(projection.Address))
		if address == nil {
			return nil, ErrInvalidInput
		}
		normalized := address.String()
		if _, duplicated := seen[normalized]; duplicated {
			return nil, ErrInvalidInput
		}
		seen[normalized] = struct{}{}
		addresses = append(addresses, normalized)
	}
	if len(addresses) < 1 || len(addresses) > 64 {
		return nil, ErrInvalidInput
	}
	sort.Strings(addresses)
	return addresses, nil
}

func (s *accessoryPricingImpl) loadIPChangePreview(ctx context.Context,
	customerNo, serviceNo string) (*ipChangePreviewRow, error) {
	var row *ipChangePreviewRow
	err := s.database.GetScan(ctx, &row, `SELECT service.service_no,service.customer_no,service.currency_code,
service.pool_no,COALESCE(service.channel_no,'') AS channel_no,service.entitlement_status_code,
service.runtime_status_code,service.provider_task_status_code,service.version,
service.current_price_snapshot_no,customer.version AS customer_subject_version,
policy.policy_no,version.policy_version,version.enabled,version.free_changes_per_window,
version.window_days,version.max_changes_per_day,version.amount_minor,version.maximum_snapshot_age_seconds
FROM plugin_idc_core_service_instance service
JOIN plugin_idc_core_customer_subject customer
 ON customer.tenant_id=service.tenant_id AND customer.customer_no=service.customer_no
JOIN plugin_idc_core_server_ip_change_policy policy
 ON policy.tenant_id=service.tenant_id AND policy.currency_code=service.currency_code
JOIN plugin_idc_core_server_ip_change_policy_version version
 ON version.policy_no=policy.policy_no AND version.policy_version=policy.current_version
WHERE service.tenant_id=0 AND service.service_no=? AND service.customer_no=?`, serviceNo, customerNo)
	return row, err
}

func (s *accessoryPricingImpl) ipChangeFrequency(ctx context.Context, serviceNo string,
	windowDays int) (int, int, error) {
	var row struct{ WindowCount, DayCount int }
	err := s.database.GetScan(ctx, &row, `SELECT
COUNT(*) FILTER (WHERE created_at >= CURRENT_TIMESTAMP - (? * INTERVAL '1 day'))::INT AS window_count,
COUNT(*) FILTER (WHERE created_at >= date_trunc('day',CURRENT_TIMESTAMP))::INT AS day_count
FROM plugin_idc_core_business_operation
WHERE tenant_id=0 AND service_no=? AND operation_type_code='ip_change'`, windowDays, serviceNo)
	return row.WindowCount, row.DayCount, err
}

func (s *accessoryPricingImpl) loadIPChangeQuoteReplay(ctx context.Context, customerNo,
	idempotencyDigest, requestDigest string) (*IPChangeQuote, error) {
	var row *struct {
		QuoteNo, ServiceNo, CurrencyCode, PolicyNo, ReservationNo, SnapshotNo, StoredRequestDigest string
		IPQuantity, MaximumSnapshotAgeSeconds                                                      int
		AmountMinor, PolicyVersion                                                                 int64
		ExpiresAt                                                                                  time.Time
	}
	err := s.database.GetScan(ctx, &row, `SELECT quote_no,service_no,currency_code,ip_quantity,
amount_minor,policy_no,policy_version,resource_reservation_no AS reservation_no,
resource_snapshot_no AS snapshot_no,maximum_snapshot_age_seconds,expires_at,
request_digest AS stored_request_digest
FROM plugin_idc_core_server_ip_change_quote
WHERE tenant_id=0 AND customer_no=? AND idempotency_digest=?`, customerNo, idempotencyDigest)
	if err != nil || row == nil {
		return nil, err
	}
	if row.StoredRequestDigest != requestDigest {
		return nil, bizerr.NewCode(CodeIPChangeConflict)
	}
	return &IPChangeQuote{QuoteNo: row.QuoteNo, ServiceNo: row.ServiceNo,
		CurrencyCode: row.CurrencyCode, IPQuantity: row.IPQuantity, FreeChange: row.AmountMinor == 0,
		AmountMinor: row.AmountMinor, PolicyNo: row.PolicyNo, PolicyVersion: row.PolicyVersion,
		ResourceReservationNo: row.ReservationNo, ResourceSnapshotNo: row.SnapshotNo,
		MaximumSnapshotAgeSeconds: row.MaximumSnapshotAgeSeconds,
		ExpiresAt:                 row.ExpiresAt.UTC(), Reused: true}, nil
}

func (s *accessoryPricingImpl) persistIPChangeQuote(ctx context.Context, customerNo, idempotencyKey,
	idempotencyDigest, requestDigest, oldDigest string, row *ipChangePreviewRow,
	directory *serveroperationresourcecap.AccessoryProjection, quote *IPChangeQuote, now time.Time) error {
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		locked, err := s.loadIPChangePreviewForUpdate(ctx, customerNo, row.ServiceNo)
		if err != nil || locked == nil || locked.Version != row.Version ||
			locked.CustomerSubjectVersion != row.CustomerSubjectVersion ||
			locked.PolicyVersion != row.PolicyVersion || !locked.PolicyEnabled {
			return bizerr.NewCode(CodeIPChangeConflict)
		}
		windowCount, dayCount, err := s.ipChangeFrequency(ctx, row.ServiceNo, row.WindowDays)
		if err != nil || dayCount >= row.MaxChangesPerDay ||
			(windowCount < row.FreeChangesPerWindow) != quote.FreeChange {
			return bizerr.NewCode(CodeIPChangeConflict)
		}
		active, err := s.database.GetValue(ctx, `SELECT EXISTS (
SELECT 1 FROM plugin_idc_core_server_operation_execution
 WHERE tenant_id=0 AND service_no=? AND status_code IN ('ready','pending','running','result_unknown')
UNION ALL SELECT 1 FROM plugin_idc_core_business_operation
 WHERE tenant_id=0 AND service_no=? AND status_code IN
 ('awaiting_financial_settlement','financially_settled','ready','executing','manual_review'))`, row.ServiceNo, row.ServiceNo)
		if err != nil || active.Bool() {
			return bizerr.NewCode(CodeIPChangeConflict)
		}
		contentDigest, err := digestValue(struct {
			Contract, QuoteNo, CustomerNo, ServiceNo, Currency, PolicyNo string
			PolicyVersion, ServiceVersion                                int64
			Quantity, Freshness                                          int
			Amount                                                       int64
			ReservationNo, SnapshotNo, OldDigest                         string
		}{"server-ip-change-quote/v1", quote.QuoteNo, customerNo, row.ServiceNo,
			row.CurrencyCode, row.PolicyNo, row.PolicyVersion, row.Version,
			quote.IPQuantity, quote.MaximumSnapshotAgeSeconds, quote.AmountMinor,
			quote.ResourceReservationNo, quote.ResourceSnapshotNo, oldDigest})
		if err != nil {
			return err
		}
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_ip_change_quote
(tenant_id,quote_no,customer_no,service_no,currency_code,ip_quantity,free_change,amount_minor,
policy_no,policy_version,maximum_snapshot_age_seconds,resource_reservation_no,resource_snapshot_no,
customer_subject_version,expected_service_version,old_ip_set_digest,idempotency_digest,request_digest,
content_digest,issued_at,expires_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, quote.QuoteNo, customerNo, row.ServiceNo,
			row.CurrencyCode, quote.IPQuantity, quote.FreeChange, quote.AmountMinor, row.PolicyNo,
			row.PolicyVersion, quote.MaximumSnapshotAgeSeconds, quote.ResourceReservationNo,
			quote.ResourceSnapshotNo, row.CustomerSubjectVersion, row.Version, oldDigest,
			idempotencyDigest, requestDigest, contentDigest, now, quote.ExpiresAt)
		if err != nil {
			return gerror.Wrap(err, "insert IP-change quote")
		}
		return s.persistIPChangeCommerceQuote(ctx, customerNo, idempotencyKey,
			requestDigest, contentDigest, row, directory, quote, now)
	})
}

func (s *accessoryPricingImpl) loadIPChangePreviewForUpdate(ctx context.Context,
	customerNo, serviceNo string) (*ipChangePreviewRow, error) {
	var row *ipChangePreviewRow
	err := s.database.GetScan(ctx, &row, `SELECT service.service_no,service.customer_no,service.currency_code,
service.pool_no,COALESCE(service.channel_no,'') AS channel_no,service.entitlement_status_code,
service.runtime_status_code,service.provider_task_status_code,service.version,
service.current_price_snapshot_no,customer.version AS customer_subject_version,
policy.policy_no,version.policy_version,version.enabled,version.free_changes_per_window,
version.window_days,version.max_changes_per_day,version.amount_minor,version.maximum_snapshot_age_seconds
FROM plugin_idc_core_service_instance service
JOIN plugin_idc_core_customer_subject customer
 ON customer.tenant_id=service.tenant_id AND customer.customer_no=service.customer_no
JOIN plugin_idc_core_server_ip_change_policy policy
 ON policy.tenant_id=service.tenant_id AND policy.currency_code=service.currency_code
JOIN plugin_idc_core_server_ip_change_policy_version version
 ON version.policy_no=policy.policy_no AND version.policy_version=policy.current_version
WHERE service.tenant_id=0 AND service.service_no=? AND service.customer_no=?
FOR UPDATE OF service,policy`, serviceNo, customerNo)
	return row, err
}
