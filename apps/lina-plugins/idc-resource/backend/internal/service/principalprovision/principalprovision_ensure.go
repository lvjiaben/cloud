package principalprovision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	usernamePrefix = "lina_"
)

var opaqueReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$`)

func (s *serviceImpl) EnsureForPool(
	ctx context.Context,
	customerNo string,
	poolNo string,
) (bool, error) {
	if s == nil || s.database == nil || s.binder == nil ||
		ctx == nil || ctx.Err() != nil || !validOpaqueReference(customerNo) || !validOpaqueReference(poolNo) {
		return false, provisionresourcecap.ErrSourceUnavailable
	}
	var route struct {
		ChannelNo string `orm:"channel_no"`
		Mapped    bool   `orm:"mapped"`
	}
	err := s.database.GetScan(ctx, &route, `
SELECT pool.channel_no,
       EXISTS (
           SELECT 1 FROM plugin_idc_resource_principal_mapping mapping
           WHERE mapping.tenant_id = ? AND mapping.channel_no = pool.channel_no
             AND mapping.customer_no = ? AND mapping.status_code = 'active'
       ) AS mapped
FROM plugin_idc_resource_pool pool
INNER JOIN plugin_idc_resource_channel channel
  ON channel.tenant_id = pool.tenant_id AND channel.channel_no = pool.channel_no
 AND channel.status_code = 'enabled' AND channel.deleted_at IS NULL
WHERE pool.tenant_id = ? AND pool.pool_no = ?
  AND pool.status_code = 'enabled' AND pool.deleted_at IS NULL`,
		idccap.DefaultTenantID, customerNo, idccap.DefaultTenantID, poolNo,
	)
	if err != nil {
		return false, gerror.Wrap(err, "resolve resource principal channel")
	}
	if route.ChannelNo == "" {
		return false, provisionresourcecap.ErrCapabilityUnavailable
	}
	if route.Mapped {
		return true, nil
	}
	if err = s.ensureTask(ctx, customerNo, route.ChannelNo); err != nil {
		return false, err
	}
	return false, nil
}

func (s *serviceImpl) ensureTask(ctx context.Context, customerNo string, channelNo string) error {
	taskNo := principalTaskNo(customerNo, channelNo)
	manager, err := s.binder(channelNo)
	if err != nil {
		return provisionresourcecap.ErrSourceUnavailable
	}
	usernameResult, err := manager.Create(
		ctx, "resource-principal-username-"+taskNo,
		staticProducer(principalUsername(customerNo, channelNo)),
	)
	if err != nil || usernameResult.Handle.IsZero() {
		return provisionresourcecap.ErrSourceUnavailable
	}
	generated, ok := manager.(secretcap.GeneratedManager)
	if !ok || generated == nil {
		return provisionresourcecap.ErrSourceUnavailable
	}
	passwordResult, err := generated.CreateGenerated(
		ctx, "resource-principal-password-"+taskNo,
		secretcap.GeneratedSecretPolicyURLSafe256,
	)
	if err != nil || passwordResult.Handle.IsZero() {
		return provisionresourcecap.ErrSourceUnavailable
	}
	_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_resource_principal_task
    (tenant_id, task_no, customer_no, channel_no, username_secret_ref, password_secret_ref)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (tenant_id, channel_no, customer_no) DO NOTHING`,
		idccap.DefaultTenantID, taskNo, customerNo, channelNo,
		usernameResult.Handle.OpaqueRef(), passwordResult.Handle.OpaqueRef(),
	)
	if err != nil {
		return gerror.Wrap(err, "persist resource principal task")
	}
	var persisted struct {
		TaskNo, UsernameSecretRef, PasswordSecretRef string
	}
	if err = s.database.GetScan(ctx, &persisted, `
SELECT task_no, username_secret_ref, password_secret_ref
FROM plugin_idc_resource_principal_task
WHERE tenant_id = ? AND channel_no = ? AND customer_no = ?`,
		idccap.DefaultTenantID, channelNo, customerNo,
	); err != nil {
		return gerror.Wrap(err, "verify resource principal task")
	}
	if persisted.TaskNo != taskNo ||
		persisted.UsernameSecretRef != usernameResult.Handle.OpaqueRef() ||
		persisted.PasswordSecretRef != passwordResult.Handle.OpaqueRef() {
		return provisionresourcecap.ErrSourceUnavailable
	}
	return nil
}

func principalTaskNo(customerNo string, channelNo string) string {
	digest := sha256.Sum256([]byte("resource-principal-task/v1\x00" + channelNo + "\x00" + customerNo))
	return "prt_" + hex.EncodeToString(digest[:16])
}

func principalUsername(customerNo string, channelNo string) string {
	digest := sha256.Sum256([]byte("resource-principal-username/v1\x00" + channelNo + "\x00" + customerNo))
	return usernamePrefix + hex.EncodeToString(digest[:16])
}

func staticProducer(value string) secretcap.PlaintextProducer {
	return func(consume secretcap.Consumer) error {
		plaintext := []byte(value)
		defer clear(plaintext)
		return consume(plaintext)
	}
}

func validOpaqueReference(value string) bool {
	return opaqueReferencePattern.MatchString(value)
}

func nextReadAt(taskNo string, attempt int, now time.Time) time.Time {
	shift := attempt
	if shift > 8 {
		shift = 8
	}
	delay := time.Second * time.Duration(1<<shift)
	digest := sha256.Sum256([]byte(taskNo))
	jitter := time.Duration(int(digest[0])%1000) * time.Millisecond
	return now.Add(delay + jitter)
}
