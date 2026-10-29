// This file verifies bounded image validation, actor denial and governed upload behavior without database fixtures.

package customeravatar

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"strings"
	"testing"
)

type testContext struct{ userID int }

func (s testContext) Current(context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: s.userID}
}

type testPrincipal struct{ principal.Reader }

func (testPrincipal) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{UserID: 81, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "CUS_0123456789abcdef0123456789abcdef"}, nil
}

type testFiles struct {
	storagecap.Service
	uploads int
	path    string
	data    []byte
}

func (f *testFiles) Put(_ context.Context, input storagecap.PutInput) (*storagecap.PutOutput, error) {
	f.uploads++
	f.path = input.Path
	if !strings.HasPrefix(input.Path, "customer-avatars/") || input.ContentType != "image/png" || input.Overwrite {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var err error
	f.data, err = io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	return &storagecap.PutOutput{}, nil
}
func (f *testFiles) Get(_ context.Context, input storagecap.GetInput) (*storagecap.GetOutput, error) {
	if input.Path != f.path {
		return &storagecap.GetOutput{Found: false}, nil
	}
	return &storagecap.GetOutput{Found: true, Body: io.NopCloser(bytes.NewReader(f.data))}, nil
}
func imageBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestReadImageRejectsInvalidContent(t *testing.T) {
	valid := imageBytes(t)
	for _, tc := range []struct {
		name   string
		reader io.Reader
	}{
		{"missing", nil}, {"fake", bytes.NewBufferString("<svg onload=alert(1)>")}, {"oversized", bytes.NewReader(make([]byte, maxAvatarBytes+1))}, {"truncated", bytes.NewReader(valid[:len(valid)-12])},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := readImage(tc.reader); !bizerr.Is(err, CodeInvalid) {
				t.Fatalf("expected invalid image, got %v", err)
			}
		})
	}
	data, format, err := readImage(bytes.NewReader(valid))
	if err != nil || format != "png" || !bytes.Equal(data, valid) {
		t.Fatalf("valid PNG rejected: %v", err)
	}
}
func TestUploadUsesCurrentCustomerAndUploadedFileOnly(t *testing.T) {
	for _, userID := range []int{0, 81} {
		t.Run(map[int]string{0: "anonymous", 81: "customer"}[userID], func(t *testing.T) {
			files := &testFiles{}
			guard, err := customer.NewAccessGuard(testContext{userID: userID}, testPrincipal{})
			if err != nil {
				t.Fatal(err)
			}
			service, err := New(guard, files)
			if err != nil {
				t.Fatal(err)
			}
			url, err := service.Upload(context.Background(), bytes.NewReader(imageBytes(t)))
			if userID == 0 {
				if err == nil || files.uploads != 0 {
					t.Fatal("anonymous actor reached file storage")
				}
				return
			}
			if err != nil || !strings.HasPrefix(url, "/x/idc-core/api/v1/public/customer-avatars/") || files.uploads != 1 {
				t.Fatalf("unexpected result: %q %v %+v", url, err, files)
			}
			key := strings.TrimPrefix(url, "/x/idc-core/api/v1/public/customer-avatars/")
			opened, err := service.Open(context.Background(), key)
			if err != nil || !opened.Found {
				t.Fatalf("public avatar unavailable: %v", err)
			}
			if err = opened.Body.Close(); err != nil {
				t.Fatal(err)
			}
			for _, bad := range []string{"../../config.yaml", "81.png", strings.Repeat("a", 48) + ".svg"} {
				if _, err = service.Open(context.Background(), bad); err == nil {
					t.Fatal("invalid object key accepted")
				}
			}

		})
	}
}
