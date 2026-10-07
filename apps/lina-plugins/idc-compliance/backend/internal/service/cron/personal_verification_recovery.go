// Package cron registers bounded compliance workers with the host scheduler.
package cron

import (
	"context"
	"reflect"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-compliance/backend/internal/service/personalverification"
)

const (
	personalVerificationRecoveryPattern     = "@every 1m"
	personalVerificationRecoveryName        = "idc-compliance-personal-verification-recovery"
	personalVerificationRecoveryDisplayName = "IDC Personal Verification Recovery"
	personalVerificationRecoveryDescription = "Rechecks one bounded batch of issued Aliyun personal verification attempts without replaying initiation."
	personalVerificationRecoveryBatchLimit  = 50
)

// RegisterPersonalVerificationRecovery registers one primary-node bounded worker.
func RegisterPersonalVerificationRecovery(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker personalverification.Service,
) error {
	if ctx == nil || ctx.Err() != nil || nilDependency(registrar) || nilDependency(worker) {
		return gerror.New("personal verification recovery job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, personalVerificationRecoveryPattern, personalVerificationRecoveryName,
		personalVerificationRecoveryDisplayName, personalVerificationRecoveryDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Recover(jobCtx, personalVerificationRecoveryBatchLimit)
			return err
		},
	)
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
