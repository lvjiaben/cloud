// This file serializes lifecycle mutations for one plugin across all host
// instances so admission cannot reopen between drain checks and disable or
// uninstall commit.

package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gogf/gf/v2/util/guid"

	"lina-core/internal/service/locker"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
)

const (
	pluginLifecycleLockLease         = 5 * time.Minute
	pluginLifecycleLockRenewInterval = pluginLifecycleLockLease / 3
	pluginLifecycleLockRenewTimeout  = 10 * time.Second
	pluginLifecycleLockUnlockTimeout = 5 * time.Second
	pluginLifecycleLockNameMaxLength = 64
)

func (s *serviceImpl) withPluginLifecycleLock(
	ctx context.Context,
	pluginID string,
	operation string,
	action func(context.Context) error,
) error {
	if s.lifecycleLockSvc == nil {
		return bizerr.NewCode(CodePluginLifecycleLockUnavailable, bizerr.P("pluginId", pluginID))
	}
	lockName := pluginLifecycleLockName(pluginID)
	holder := "plugin-lifecycle:" + guid.S()
	instance, acquired, err := s.lifecycleLockSvc.Lock(
		ctx,
		lockName,
		holder,
		strings.TrimSpace(operation),
		pluginLifecycleLockLease,
	)
	if err != nil {
		return bizerr.WrapCode(err, CodePluginLifecycleLockUnavailable, bizerr.P("pluginId", pluginID))
	}
	if !acquired || instance == nil {
		return bizerr.NewCode(CodePluginLifecycleAlreadyRunning, bizerr.P("pluginId", pluginID))
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), pluginLifecycleLockUnlockTimeout)
		defer cancel()
		if unlockErr := instance.Unlock(unlockCtx); unlockErr != nil {
			logger.Warningf(ctx, "release plugin lifecycle lock failed plugin=%s operation=%s err=%v", pluginID, operation, unlockErr)
		}
	}()
	actionCtx, cancelAction := context.WithCancelCause(ctx)
	defer cancelAction(nil)
	renewCtx, cancelRenew := context.WithCancel(ctx)
	defer cancelRenew()
	renewInterval := s.lifecycleLockRenewInterval
	if renewInterval <= 0 {
		renewInterval = pluginLifecycleLockRenewInterval
	}
	renewTimeout := s.lifecycleLockRenewTimeout
	if renewTimeout <= 0 {
		renewTimeout = pluginLifecycleLockRenewTimeout
	}
	renew := s.lifecycleLockRenew
	if renew == nil {
		renew = func(callCtx context.Context, held *locker.Instance) error {
			return held.Renew(callCtx)
		}
	}
	actionFinished := make(chan struct{})
	renewalResult := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-actionFinished:
				renewalResult <- nil
				return
			case <-ticker.C:
				renewCallCtx, cancelRenewCall := context.WithTimeout(renewCtx, renewTimeout)
				renewErr := renew(renewCallCtx, instance)
				cancelRenewCall()
				if renewErr != nil {
					if renewCtx.Err() != nil {
						renewalResult <- nil
						return
					}
					cancelAction(renewErr)
					renewalResult <- renewErr
					return
				}
			}
		}
	}()
	actionErr := action(actionCtx)
	cancelRenew()
	close(actionFinished)
	renewErr := <-renewalResult
	if actionErr != nil && renewErr != nil {
		return bizerr.WrapCode(renewErr, CodePluginLifecycleLockUnavailable, bizerr.P("pluginId", pluginID))
	}
	return actionErr
}

func pluginLifecycleLockName(pluginID string) string {
	const prefix = "plugin-lifecycle:"
	normalizedID := strings.TrimSpace(pluginID)
	lockName := prefix + normalizedID
	if len(lockName) <= pluginLifecycleLockNameMaxLength {
		return lockName
	}
	digest := sha256.Sum256([]byte(normalizedID))
	return prefix + hex.EncodeToString(digest[:])[:pluginLifecycleLockNameMaxLength-len(prefix)]
}
