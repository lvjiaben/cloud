// Command idc-compliance-local-gateway-supplier-keyring generates and
// rotates the current installation's private supplier-identity keyring.
package main

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"lina-platform/idc-compliance-gateway/internal/supplieridentity"
)

func main() {
	output := flag.String("out", "", "new supplier identity keyring output file")
	rotate := flag.String("rotate", "", "existing supplier identity keyring to rotate")
	family := flag.String(
		"family", "", "rotation family: subject_encryption, subject_lookup, or identity_operation",
	)
	flag.Parse()
	if err := run(*output, *rotate, *family); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output, rotate, family string) error {
	output = strings.TrimSpace(output)
	rotate = strings.TrimSpace(rotate)
	family = strings.TrimSpace(family)
	if err := validateNewOutput(output); err != nil {
		return err
	}
	var (
		file supplieridentity.File
		err  error
	)
	if rotate == "" {
		if family != "" {
			return fmt.Errorf("-family is only valid with -rotate")
		}
		file, err = supplieridentity.Generate(rand.Reader)
	} else {
		if !filepath.IsAbs(rotate) || filepath.Clean(rotate) == filepath.Clean(output) ||
			strings.ContainsAny(rotate, "\r\n\x00") {
			return fmt.Errorf("-rotate must name a different absolute file")
		}
		purpose, purposeErr := parsePurpose(family)
		if purposeErr != nil {
			return purposeErr
		}
		file, err = readKeyring(rotate)
		if err == nil {
			file, err = supplieridentity.Rotate(file, purpose, rand.Reader)
		}
	}
	if err != nil {
		return fmt.Errorf("prepare supplier identity keyring: %w", err)
	}
	content, err := supplieridentity.Marshal(file)
	if err != nil {
		return err
	}
	defer clearBytes(content)
	if err = writePrivateFile(output, content); err != nil {
		return fmt.Errorf("write supplier identity keyring: %w", err)
	}
	fmt.Printf("wrote installation-local supplier identity keyring %s\n", output)
	return nil
}

func parsePurpose(value string) (supplieridentity.Purpose, error) {
	switch supplieridentity.Purpose(value) {
	case supplieridentity.PurposeSubjectEncryption:
		return supplieridentity.PurposeSubjectEncryption, nil
	case supplieridentity.PurposeSubjectLookup:
		return supplieridentity.PurposeSubjectLookup, nil
	case supplieridentity.PurposeIdentityOperation:
		return supplieridentity.PurposeIdentityOperation, nil
	default:
		return "", fmt.Errorf("-family must select one supported key family")
	}
}

func validateNewOutput(path string) error {
	if path == "" || !filepath.IsAbs(path) || strings.ContainsAny(path, "\r\n\x00") {
		return fmt.Errorf("-out must be an absolute file path")
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refusing to overwrite existing output file")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output file: %w", err)
	}
	return nil
}

func readKeyring(path string) (result supplieridentity.File, returnError error) {
	file, err := os.Open(path)
	if err != nil {
		return supplieridentity.File{}, err
	}
	defer func() {
		if closeError := file.Close(); closeError != nil && returnError == nil {
			returnError = closeError
		}
	}()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() ||
		runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return supplieridentity.File{}, fmt.Errorf("keyring must be a private regular file")
	}
	content, err := io.ReadAll(io.LimitReader(file, supplieridentity.MaximumKeyringSize+1))
	if err != nil {
		clearBytes(content)
		return supplieridentity.File{}, err
	}
	defer clearBytes(content)
	if int64(len(content)) > supplieridentity.MaximumKeyringSize {
		return supplieridentity.File{}, fmt.Errorf("keyring file is too large")
	}
	return supplieridentity.Parse(content)
}

func writePrivateFile(path string, content []byte) (returnError error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if complete {
			return
		}
		if cleanupError := os.Remove(path); cleanupError != nil && !errors.Is(cleanupError, os.ErrNotExist) {
			returnError = errors.Join(returnError, cleanupError)
		}
	}()
	if _, err = file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err = os.Chmod(path, 0o600); err != nil {
			return err
		}
	}
	complete = true
	return nil
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
