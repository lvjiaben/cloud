package reconciliation

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1" // #nosec G505 -- required only to verify provider-declared legacy bill hashes.
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
)

const maxStatementBytes = 16 << 20

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type ProviderSource struct {
	database gdb.DB
	channels *channelconfig.Service
	http     HTTPDoer
}

func NewProviderSource(database gdb.DB, channels *channelconfig.Service, client HTTPDoer) (*ProviderSource, error) {
	if database == nil || channels == nil || client == nil {
		return nil, gerror.New("reconciliation source dependencies are required")
	}
	return &ProviderSource{database: database, channels: channels, http: client}, nil
}

func (s *ProviderSource) Load(ctx context.Context, channelNo string, day time.Time, limit int) ([]SourceRecord, string, error) {
	row, err := s.database.GetOne(ctx, `SELECT capability_snapshot FROM plugin_idc_payment_channel WHERE channel_no=? AND status_code='enabled' AND deleted_at IS NULL`, channelNo)
	if err != nil || row.IsEmpty() {
		return nil, "", gerror.New("reconciliation channel is unavailable")
	}
	capabilities := map[string]bool{}
	if err = row["capability_snapshot"].Scan(&capabilities); err != nil {
		return nil, "", gerror.New("reconciliation channel capability is invalid")
	}
	if capabilities[string(paymentcap.OperationStatement)] {
		return s.loadStatement(ctx, channelNo, day, limit)
	}
	if capabilities[string(paymentcap.OperationQuery)] {
		return s.loadQueries(ctx, channelNo, day, limit)
	}
	return nil, "", paymentcap.ErrUnsupported
}

func (s *ProviderSource) loadStatement(ctx context.Context, channelNo string, day time.Time, limit int) ([]SourceRecord, string, error) {
	var records []SourceRecord
	var digest string
	err := s.channels.WithCredentials(ctx, channelNo, func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
		if !adapter.Capabilities().Operations[paymentcap.OperationStatement] {
			return paymentcap.ErrUnsupported
		}
		file, err := adapter.Statement(ctx, credentials, paymentcap.StatementRequest{BusinessDate: day})
		if err != nil {
			return err
		}
		body, err := s.download(ctx, file)
		if err != nil {
			return err
		}
		digest = sha256Digest(body)
		if file.EvidenceDigest != "" {
			digest = stableDigest(file.EvidenceDigest, digest)
		}
		records, err = parseStatement(adapter.Provider(), body, day, limit)
		return err
	})
	return records, digest, err
}

func (s *ProviderSource) loadQueries(ctx context.Context, channelNo string, day time.Time, limit int) ([]SourceRecord, string, error) {
	rows, err := s.database.GetAll(ctx, `SELECT attempt_no,external_trade_reference FROM plugin_idc_payment_attempt WHERE channel_no=? AND created_at>=? AND created_at<? ORDER BY id LIMIT ?`, channelNo, day.UTC(), day.AddDate(0, 0, 1).UTC(), limit)
	if err != nil {
		return nil, "", err
	}
	if len(rows) == limit {
		return nil, "", gerror.New("reconciliation query source limit reached")
	}
	records := make([]SourceRecord, 0, len(rows))
	err = s.channels.WithCredentials(ctx, channelNo, func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
		if !adapter.Capabilities().Operations[paymentcap.OperationQuery] {
			return paymentcap.ErrUnsupported
		}
		for _, row := range rows {
			result, queryErr := adapter.Query(ctx, credentials, paymentcap.QueryRequest{AttemptNo: row["attempt_no"].String(), ExternalTradeReference: row["external_trade_reference"].String()})
			if queryErr != nil {
				return queryErr
			}
			if result == nil || result.AttemptNo != row["attempt_no"].String() || len(result.EvidenceDigest) != 64 {
				return gerror.New("authoritative payment query returned invalid evidence")
			}
			records = append(records, SourceRecord{RecordType: "payment", LocalSubjectNo: result.AttemptNo, ProviderEventReference: choose(result.ProviderEventReference != "", result.ProviderEventReference, result.EvidenceDigest), ExternalReference: result.ExternalTradeReference, CurrencyCode: result.Currency, AmountMinor: result.AmountMinor, StatusCode: result.Status, EvidenceDigest: result.EvidenceDigest, ObservedAt: time.Now().UTC()})
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	parts := make([]string, 0, len(records))
	for _, record := range records {
		parts = append(parts, record.EvidenceDigest)
	}
	return records, stableDigest(parts...), nil
}

func (s *ProviderSource) download(ctx context.Context, file *paymentcap.StatementFile) ([]byte, error) {
	if file == nil {
		return nil, gerror.New("provider statement is missing")
	}
	parsed, err := url.Parse(file.DownloadURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, gerror.New("provider statement URL is invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := s.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, gerror.Newf("provider statement download failed: status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxStatementBytes+1))
	if err != nil || len(body) == 0 || len(body) > maxStatementBytes {
		return nil, gerror.New("provider statement size is invalid")
	}
	if file.HashValue != "" {
		var actual string
		switch strings.ToUpper(file.HashType) {
		case "SHA256":
			sum := sha256.Sum256(body)
			actual = hex.EncodeToString(sum[:])
		case "SHA1":
			sum := sha1.Sum(body)
			actual = hex.EncodeToString(sum[:])
		default:
			return nil, gerror.New("provider statement hash type is unsupported")
		}
		if !strings.EqualFold(actual, file.HashValue) {
			return nil, gerror.New("provider statement hash mismatch")
		}
	}
	return body, nil
}

func parseStatement(provider paymentcap.ProviderCode, body []byte, day time.Time, limit int) ([]SourceRecord, error) {
	files, err := statementCSVFiles(body)
	if err != nil {
		return nil, err
	}
	result := make([]SourceRecord, 0)
	for _, content := range files {
		reader := csv.NewReader(bytes.NewReader(content))
		reader.FieldsPerRecord = -1
		reader.LazyQuotes = true
		reader.TrimLeadingSpace = true
		rows, readErr := reader.ReadAll()
		if readErr != nil || len(rows) < 2 {
			continue
		}
		headerIndex := map[string]int{}
		for index, value := range rows[0] {
			headerIndex[cleanField(value)] = index
		}
		for _, row := range rows[1:] {
			if len(result) >= limit {
				return nil, gerror.New("provider statement record limit reached")
			}
			parsed := parseStatementRow(provider, headerIndex, row, day)
			result = append(result, parsed...)
		}
	}
	if len(result) == 0 {
		return nil, gerror.New("provider statement contains no recognized records")
	}
	return result, nil
}

func statementCSVFiles(body []byte) ([][]byte, error) {
	if len(body) >= 4 && bytes.Equal(body[:4], []byte{'P', 'K', 3, 4}) {
		archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			return nil, err
		}
		result := make([][]byte, 0, len(archive.File))
		for _, file := range archive.File {
			if file.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(file.Name), ".csv") || file.UncompressedSize64 > maxStatementBytes {
				continue
			}
			opened, openErr := file.Open()
			if openErr != nil {
				return nil, openErr
			}
			content, readErr := io.ReadAll(io.LimitReader(opened, maxStatementBytes+1))
			opened.Close()
			if readErr != nil || len(content) > maxStatementBytes {
				return nil, gerror.New("provider statement archive is invalid")
			}
			result = append(result, content)
		}
		if len(result) == 0 {
			return nil, gerror.New("provider statement archive has no CSV file")
		}
		return result, nil
	}
	return [][]byte{body}, nil
}

func parseStatementRow(provider paymentcap.ProviderCode, header map[string]int, row []string, day time.Time) []SourceRecord {
	value := func(names ...string) string {
		for _, name := range names {
			if index, ok := header[cleanField(name)]; ok && index < len(row) {
				return cleanField(row[index])
			}
		}
		return ""
	}
	attempt := value("\u5546\u6237\u8ba2\u5355\u53f7", "merchantorderno", "outtradeno")
	external := value("\u652f\u4ed8\u5b9d\u4ea4\u6613\u53f7", "\u5fae\u4fe1\u8ba2\u5355\u53f7", "providertradeno", "transactionid")
	currency := strings.ToUpper(value("\u8d27\u5e01\u79cd\u7c7b", "\u5e01\u79cd", "currency"))
	if currency == "" && provider == paymentcap.ProviderAlipay {
		currency = "CNY"
	}
	amountRaw := value("\u8ba2\u5355\u91d1\u989d\uff08\u5143\uff09", "\u8ba2\u5355\u91d1\u989d(\u5143)", "\u5e94\u7ed3\u8ba2\u5355\u91d1\u989d", "totalamount", "amount")
	amount, ok := parseMoneyMinor(amountRaw)
	observed := day.Add(12 * time.Hour).UTC()
	status := strings.ToLower(value("\u4ea4\u6613\u72b6\u6001", "status"))
	if status == "" && attempt != "" && external != "" {
		status = "succeeded"
	}
	if strings.Contains(status, "\u6210\u529f") || status == "success" {
		status = "succeeded"
	}
	result := make([]SourceRecord, 0, 2)
	if attempt != "" && external != "" && currency != "" && ok && status != "" {
		evidence := stableDigest(string(provider), "payment", strings.Join(row, "\x1f"))
		result = append(result, SourceRecord{RecordType: "payment", LocalSubjectNo: attempt, ProviderEventReference: external, ExternalReference: external, CurrencyCode: currency, AmountMinor: amount, StatusCode: status, EvidenceDigest: evidence, ObservedAt: observed})
	}
	refundNo := value("\u5546\u6237\u9000\u6b3e\u5355\u53f7", "\u9000\u6b3e\u6279\u6b21\u53f7/\u8bf7\u6c42\u53f7", "outrefundno")
	externalRefund := value("\u5fae\u4fe1\u9000\u6b3e\u5355\u53f7", "\u9000\u6b3e\u4ea4\u6613\u53f7", "refundid")
	refundRaw := value("\u9000\u6b3e\u91d1\u989d", "\u9000\u6b3e\u91d1\u989d\uff08\u5143\uff09", "\u9000\u6b3e\u91d1\u989d(\u5143)", "refundamount")
	refundAmount, refundOK := parseMoneyMinor(refundRaw)
	refundStatus := strings.ToLower(value("\u9000\u6b3e\u72b6\u6001", "refundstatus"))
	if strings.Contains(refundStatus, "\u6210\u529f") || refundStatus == "success" {
		refundStatus = "refunded"
	}
	if refundNo != "" && externalRefund != "" && currency != "" && refundOK && refundStatus != "" {
		evidence := stableDigest(string(provider), "refund", strings.Join(row, "\x1f"))
		result = append(result, SourceRecord{RecordType: "refund", LocalSubjectNo: refundNo, ProviderEventReference: externalRefund, ExternalReference: externalRefund, CurrencyCode: currency, AmountMinor: refundAmount, StatusCode: refundStatus, EvidenceDigest: evidence, ObservedAt: observed})
	}
	return result
}

func cleanField(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
	value = strings.Trim(value, "`\"")
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "")
	return strings.ToLower(value)
}
func parseMoneyMinor(value string) (int64, bool) {
	value = strings.TrimSpace(strings.Trim(value, "`"))
	if value == "" {
		return 0, false
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return 0, false
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, false
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		fraction = fraction[:2]
	}
	fraction += strings.Repeat("0", 2-len(fraction))
	cents := int64(0)
	if fraction != "" {
		cents, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, false
		}
	}
	total := whole*100 + cents
	return total, total > 0
}
func sha256Digest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
