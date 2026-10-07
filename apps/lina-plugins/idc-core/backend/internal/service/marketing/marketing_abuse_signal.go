// This file records privacy-safe registration signals and creates review-only abuse cases.

package marketing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type RegistrationSignalRecorder interface {
	RecordRegistration(context.Context, RegistrationSignalInput) (*RegistrationSignalResult, error)
}

type RegistrationSignalInput struct {
	CustomerNo string
	DeviceID   string
	SourceIP   string
}

type RegistrationSignalResult struct {
	CreatedCaseCount int
	Reused           bool
}

type registrationSignalRecorderImpl struct {
	database gdb.DB
	key      []byte
}

func NewRegistrationSignalRecorder(database gdb.DB, key []byte) (RegistrationSignalRecorder, error) {
	if database == nil || len(key) != sha256.Size {
		return nil, gerror.New("marketing registration signal recorder requires database and installation key")
	}
	return &registrationSignalRecorderImpl{database: database, key: append([]byte(nil), key...)}, nil
}

type registrationSignalRow struct {
	DeviceDigest string `orm:"device_digest"`
	IPDigest     string `orm:"ip_digest"`
}

type registrationSignalWindow struct {
	ObservedCount int       `orm:"observed_count"`
	WindowStart   time.Time `orm:"window_start"`
	WindowEnd     time.Time `orm:"window_end"`
	EvidenceSeed  string    `orm:"evidence_seed"`
}

func (s *registrationSignalRecorderImpl) RecordRegistration(ctx context.Context, input RegistrationSignalInput) (*RegistrationSignalResult, error) {
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	address, parseErr := netip.ParseAddr(strings.TrimSpace(input.SourceIP))
	if input.CustomerNo == "" || len(input.DeviceID) < 16 || len(input.DeviceID) > 128 || parseErr != nil {
		return nil, bizerr.NewCode(CodeInsightsInvalid)
	}
	deviceDigest := s.digest("registration-device", input.DeviceID)
	ipDigest := s.digest("registration-ip", address.Unmap().String())
	result := &RegistrationSignalResult{}
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		lockKeys := []string{"marketing-registration-device:" + deviceDigest, "marketing-registration-ip:" + ipDigest}
		sort.Strings(lockKeys)
		for _, lockKey := range lockKeys {
			if _, lockErr := s.database.GetValue(txCtx, `SELECT pg_advisory_xact_lock(hashtextextended(?, 62100410))`, lockKey); lockErr != nil {
				return lockErr
			}
		}
		var existing *registrationSignalRow
		if readErr := s.database.GetScan(txCtx, &existing, `SELECT device_digest, ip_digest
FROM plugin_idc_core_marketing_registration_signal
WHERE tenant_id=0 AND customer_no=? FOR UPDATE`, input.CustomerNo); readErr != nil {
			return readErr
		}
		if existing != nil && existing.DeviceDigest != "" {
			result.Reused = true
			return nil
		}
		if _, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_marketing_registration_signal
 (tenant_id, customer_no, device_digest, ip_digest) VALUES (0, ?, ?, ?)`, input.CustomerNo, deviceDigest, ipDigest); insertErr != nil {
			return insertErr
		}
		for _, dimension := range []struct {
			column       string
			digest       string
			evidenceCode string
		}{
			{column: "device_digest", digest: deviceDigest, evidenceCode: "same_device_24h"},
			{column: "ip_digest", digest: ipDigest, evidenceCode: "same_ip_24h"},
		} {
			window, windowErr := s.registrationWindow(txCtx, dimension.column, dimension.digest)
			if windowErr != nil {
				return windowErr
			}
			if window.ObservedCount < 3 {
				continue
			}
			evidenceDigest := s.digest("registration-abuse-evidence", dimension.column+":"+dimension.digest+":"+window.EvidenceSeed)
			caseNo := "mab_" + evidenceDigest[:32]
			insertResult, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_marketing_abuse_case
 (tenant_id, case_no, campaign_no, customer_no, signal_code, related_account_count,
  evidence_summary_code, evidence_digest, window_started_at, window_ended_at, observed_count)
VALUES (0, ?, NULL, ?, 'bulk_registration', ?, ?, ?, ?, ?, ?)
ON CONFLICT DO NOTHING`, caseNo, input.CustomerNo, window.ObservedCount, dimension.evidenceCode,
				evidenceDigest, window.WindowStart, window.WindowEnd, window.ObservedCount)
			if insertErr != nil {
				return insertErr
			}
			created, rowsErr := insertResult.RowsAffected()
			if rowsErr != nil {
				return rowsErr
			}
			result.CreatedCaseCount += int(created)
		}
		return nil
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeInsightsUnavailable)
	}
	return result, nil
}

func (s *registrationSignalRecorderImpl) registrationWindow(ctx context.Context, column string, digest string) (registrationSignalWindow, error) {
	if column != "device_digest" && column != "ip_digest" {
		return registrationSignalWindow{}, gerror.New("unsupported registration signal dimension")
	}
	var result registrationSignalWindow
	query := `SELECT count(*) AS observed_count, min(registered_at) AS window_start,
 max(registered_at) AS window_end, string_agg(customer_no, ',' ORDER BY customer_no) AS evidence_seed
FROM plugin_idc_core_marketing_registration_signal
WHERE tenant_id=0 AND ` + column + `=? AND registered_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours'`
	if err := s.database.GetScan(ctx, &result, query, digest); err != nil {
		return registrationSignalWindow{}, err
	}
	return result, nil
}

func (s *registrationSignalRecorderImpl) digest(purpose string, value string) string {
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte("idc/marketing/" + purpose + "/v1\x00"))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}
