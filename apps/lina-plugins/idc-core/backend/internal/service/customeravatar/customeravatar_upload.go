// This file validates image content before storing it in the plugin's dedicated avatar prefix after current-customer authorization.

package customeravatar

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	_ "golang.org/x/image/webp" // Register WebP decoding for avatar validation.
	"image"
	_ "image/jpeg" // Register JPEG decoding for avatar validation.
	_ "image/png"  // Register PNG decoding for avatar validation.
	"io"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"regexp"
)

const (
	maxAvatarBytes  int64 = 5 * 1024 * 1024
	maxAvatarPixels int64 = 16 * 1024 * 1024
)

func (s *serviceImpl) Upload(ctx context.Context, content io.Reader) (string, error) {
	if _, err := s.access.Resolve(ctx); err != nil {
		return "", err
	}
	data, format, err := readImage(content)
	if err != nil {
		return "", err
	}
	var randomKey [24]byte
	if _, err = rand.Read(randomKey[:]); err != nil {
		return "", bizerr.WrapCode(err, CodeUnavailable)
	}
	key := hex.EncodeToString(randomKey[:]) + "." + format
	if _, err = s.storage.Put(ctx, storagecap.PutInput{Path: "customer-avatars/" + key, Body: bytes.NewReader(data), Size: int64(len(data)), ContentType: "image/" + format}); err != nil {
		return "", bizerr.WrapCode(err, CodeUnavailable)
	}
	return "/x/idc-core/api/v1/public/customer-avatars/" + key, nil
}

// readImage bounds memory before decoding and checks actual content instead of trusting client MIME or filenames.
func readImage(content io.Reader) ([]byte, string, error) {
	if content == nil {
		return nil, "", bizerr.NewCode(CodeInvalid)
	}
	data, err := io.ReadAll(io.LimitReader(content, maxAvatarBytes+1))
	if err != nil {
		return nil, "", bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(data) == 0 || int64(len(data)) > maxAvatarBytes {
		return nil, "", bizerr.NewCode(CodeInvalid)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg" && format != "webp") || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxAvatarPixels {
		return nil, "", bizerr.NewCode(CodeInvalid)
	}
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		return nil, "", bizerr.WrapCode(err, CodeInvalid)
	}
	return data, format, nil
}

// Public avatar keys carry 192 bits of randomness and never contain user IDs or filesystem paths.
var avatarKeyPattern = regexp.MustCompile(`^[a-f0-9]{48}\.(png|jpeg|webp)$`)

func (s *serviceImpl) Open(ctx context.Context, key string) (*storagecap.GetOutput, error) {
	if !avatarKeyPattern.MatchString(key) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	object, err := s.storage.Get(ctx, storagecap.GetInput{Path: "customer-avatars/" + key})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if object == nil || !object.Found || object.Body == nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return object, nil
}
