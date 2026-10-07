// This file stores and reads only authenticated ciphertext through plugin-scoped private storage.

package electroniccontracttemplate

import (
	"bytes"
	"context"
	"crypto/subtle"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/storagecap"
)

func (s *serviceImpl) putOrConfirmContentEnvelope(
	ctx context.Context,
	path string,
	envelope []byte,
) error {
	found, err := s.confirmContentEnvelope(ctx, path, envelope)
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	if _, err = s.storage.Put(ctx, storagecap.PutInput{
		Path: path, Body: bytes.NewReader(envelope), Size: int64(len(envelope)),
		ContentType: "application/octet-stream", Overwrite: false,
	}); err != nil {
		found, confirmErr := s.confirmContentEnvelope(ctx, path, envelope)
		if confirmErr != nil {
			return confirmErr
		}
		if found {
			return nil
		}
		return err
	}
	found, err = s.confirmContentEnvelope(ctx, path, envelope)
	if err != nil || !found {
		return gerror.New("contract template private object confirmation failed")
	}
	return nil
}

func (s *serviceImpl) confirmContentEnvelope(
	ctx context.Context,
	path string,
	expected []byte,
) (bool, error) {
	if s == nil || nilDependency(s.storage) || path == "" || len(expected) == 0 {
		return false, gerror.New("contract template private object confirmation input is invalid")
	}
	result, err := s.storage.Get(ctx, storagecap.GetInput{Path: path})
	if err != nil {
		return false, err
	}
	if result == nil || !result.Found {
		return false, nil
	}
	if result.Object == nil || result.Body == nil ||
		result.Object.Visibility != storagecap.VisibilityPrivate ||
		result.Object.Size != int64(len(expected)) ||
		result.Object.ContentType != "application/octet-stream" {
		if result.Body != nil {
			_ = result.Body.Close()
		}
		return false, gerror.New("contract template private object metadata is inconsistent")
	}
	actual, readErr := io.ReadAll(io.LimitReader(result.Body, int64(len(expected))+1))
	closeErr := result.Body.Close()
	defer clearContentBytes(actual)
	if readErr != nil || closeErr != nil || len(actual) != len(expected) ||
		subtle.ConstantTimeCompare(actual, expected) != 1 {
		return false, gerror.New("contract template private object integrity check failed")
	}
	return true, nil
}

func (s *serviceImpl) readContentEnvelope(
	ctx context.Context,
	path string,
	maximum int64,
) ([]byte, error) {
	if s == nil || nilDependency(s.storage) || path == "" || maximum <= 0 {
		return nil, gerror.New("contract template private object read input is invalid")
	}
	result, err := s.storage.Get(ctx, storagecap.GetInput{Path: path})
	if err != nil || result == nil || !result.Found || result.Object == nil || result.Body == nil ||
		result.Object.Visibility != storagecap.VisibilityPrivate ||
		result.Object.ContentType != "application/octet-stream" || result.Object.Size <= 0 ||
		result.Object.Size > maximum+128 {
		if result != nil && result.Body != nil {
			_ = result.Body.Close()
		}
		return nil, gerror.New("contract template private object is unavailable")
	}
	value, readErr := io.ReadAll(io.LimitReader(result.Body, maximum+129))
	closeErr := result.Body.Close()
	if readErr != nil || closeErr != nil || int64(len(value)) != result.Object.Size {
		clearContentBytes(value)
		return nil, gerror.New("contract template private object read failed")
	}
	return value, nil
}
