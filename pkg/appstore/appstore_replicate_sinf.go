package appstore

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"
)

// ErrNoSinfs is returned by ReplicateSinf when the App Store did not return
// any DRM signature for the package. The IPA itself is complete and can be
// decrypted / inspected, but it cannot be installed on a device until Apple
// serves the sinfs again.
var ErrNoSinfs = errors.New("the App Store response did not include any sinf")

type Sinf struct {
	ID   int64  `plist:"id,omitempty"`
	Data []byte `plist:"sinf,omitempty"`
}

type ReplicateSinfInput struct {
	Sinfs       []Sinf
	PackagePath string
}

func (t *appstore) ReplicateSinf(input ReplicateSinfInput) error {
	if len(input.Sinfs) == 0 {
		return ErrNoSinfs
	}

	zipReader, err := zip.OpenReader(input.PackagePath)
	if err != nil {
		return errors.New("failed to open zip reader")
	}

	tmpPath := fmt.Sprintf("%s.tmp", input.PackagePath)
	tmpFile, err := t.os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	zipWriter := zip.NewWriter(tmpFile)

	err = t.replicateZip(zipReader, zipWriter)
	if err != nil {
		return fmt.Errorf("failed to replicate zip: %w", err)
	}

	bundleName, err := t.readBundleName(zipReader)
	if err != nil {
		return fmt.Errorf("failed to read bundle name: %w", err)
	}

	manifest, err := t.readManifestPlist(zipReader)
	if err != nil {
		return fmt.Errorf("failed to read manifest plist: %w", err)
	}

	info, err := t.readInfoPlist(zipReader)
	if err != nil {
		return fmt.Errorf("failed to read info plist: %w", err)
	}

	if manifest != nil && len(manifest.SinfPaths) > 0 {
		err = t.replicateSinfFromManifest(*manifest, zipWriter, input.Sinfs, bundleName)
	} else if info != nil {
		err = t.replicateSinfFromInfo(*info, zipWriter, input.Sinfs, bundleName)
	} else {
		err = errors.New("package has neither SC_Info/Manifest.plist nor Info.plist")
	}

	if err != nil {
		// Do not leave a half-written "<package>.tmp" behind: it is not a
		// valid zip archive and only confuses anyone inspecting the failure.
		zipReader.Close()
		zipWriter.Close()
		tmpFile.Close()
		_ = t.os.Remove(tmpPath)

		return fmt.Errorf("failed to replicate sinf: %w", err)
	}

	zipReader.Close()
	zipWriter.Close()
	tmpFile.Close()

	err = t.os.Remove(input.PackagePath)
	if err != nil {
		return fmt.Errorf("failed to remove original file: %w", err)
	}

	err = t.os.Rename(tmpPath, input.PackagePath)
	if err != nil {
		return fmt.Errorf("failed to remove original file: %w", err)
	}

	return nil
}

type packageManifest struct {
	SinfPaths []string `plist:"SinfPaths,omitempty"`
}

type packageInfo struct {
	BundleExecutable string `plist:"CFBundleExecutable,omitempty"`
}

// sinfTarget is a single "<path inside the .app> <- sinf blob" assignment.
type sinfTarget struct {
	Path string
	Data []byte
}

// resolveSinfTargets maps the sinfs returned by the App Store onto the
// SinfPaths declared in SC_Info/Manifest.plist.
//
// For most packages both lists have the same length and the mapping is
// positional. Some apps with many embedded binaries (e.g. Google,
// com.google.GoogleMobile) declare more (or fewer) SinfPaths than the number
// of sinfs Apple returns for the purchase. Failing hard in that case used to
// abort the whole download after the package had already been fetched
// ("failed to zip sinfs: slices have different lengths"), even though the
// package is perfectly usable with the sinfs that are available.
//
// When the counts differ the sinf `id` field is used as an index into
// SinfPaths where possible, a single sinf is replicated to every path (this
// is exactly what installd does with SinfReplicationPaths on device), and
// any path that cannot be matched is simply left without a sinf.
func resolveSinfTargets(sinfs []Sinf, paths []string) []sinfTarget {
	targets := make([]sinfTarget, 0, len(paths))

	if len(sinfs) == 0 || len(paths) == 0 {
		return targets
	}

	if len(sinfs) == len(paths) {
		for i, path := range paths {
			targets = append(targets, sinfTarget{Path: path, Data: sinfs[i].Data})
		}

		return targets
	}

	// Length mismatch: prefer the explicit sinf ids when they form a valid,
	// unique set of indexes into SinfPaths.
	byID := make(map[int64]Sinf, len(sinfs))
	idsValid := true

	for _, sinf := range sinfs {
		if sinf.ID < 0 || sinf.ID >= int64(len(paths)) {
			idsValid = false

			break
		}

		if _, dup := byID[sinf.ID]; dup {
			idsValid = false

			break
		}

		byID[sinf.ID] = sinf
	}

	for i, path := range paths {
		var (
			sinf Sinf
			ok   bool
		)

		switch {
		case idsValid:
			sinf, ok = byID[int64(i)]
		case i < len(sinfs):
			sinf, ok = sinfs[i], true
		}

		if !ok && len(sinfs) == 1 {
			// Only one signature for the whole package: replicate it.
			sinf, ok = sinfs[0], true
		}

		if !ok {
			continue
		}

		targets = append(targets, sinfTarget{Path: path, Data: sinf.Data})
	}

	return targets
}

func (*appstore) replicateSinfFromManifest(manifest packageManifest, zip *zip.Writer, sinfs []Sinf, bundleName string) error {
	if len(sinfs) == 0 {
		return ErrNoSinfs
	}

	for _, target := range resolveSinfTargets(sinfs, manifest.SinfPaths) {
		sp := fmt.Sprintf("Payload/%s.app/%s", bundleName, target.Path)

		file, err := zip.Create(sp)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		_, err = file.Write(target.Data)
		if err != nil {
			return fmt.Errorf("failed to write data: %w", err)
		}
	}

	return nil
}

func (t *appstore) replicateSinfFromInfo(info packageInfo, zip *zip.Writer, sinfs []Sinf, bundleName string) error {
	if len(sinfs) == 0 {
		return ErrNoSinfs
	}

	sp := fmt.Sprintf("Payload/%s.app/SC_Info/%s.sinf", bundleName, info.BundleExecutable)

	file, err := zip.Create(sp)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	_, err = file.Write(sinfs[0].Data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}

func (t *appstore) replicateZip(src *zip.ReadCloser, dst *zip.Writer) error {
	for _, file := range src.File {
		err := func() error {
			srcFile, err := file.Open()
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer srcFile.Close()

			header := file.FileHeader
			dstFile, err := dst.CreateHeader(&header)

			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				return fmt.Errorf("failed to copy file: %w", err)
			}

			return nil
		}()
		if err != nil {
			return err
		}
	}

	return nil
}

func (*appstore) readInfoPlist(reader *zip.ReadCloser) (*packageInfo, error) {
	for _, file := range reader.File {
		if strings.Contains(file.Name, ".app/Info.plist") {
			src, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open file: %w", err)
			}

			data := new(bytes.Buffer)
			_, err = io.Copy(data, src)

			if err != nil {
				return nil, fmt.Errorf("failed to copy data: %w", err)
			}

			var info packageInfo
			_, err = plist.Unmarshal(data.Bytes(), &info)

			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal data: %w", err)
			}

			return &info, nil
		}
	}

	return nil, nil
}

func (*appstore) readManifestPlist(reader *zip.ReadCloser) (*packageManifest, error) {
	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, ".app/SC_Info/Manifest.plist") {
			src, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open file: %w", err)
			}

			data := new(bytes.Buffer)
			_, err = io.Copy(data, src)

			if err != nil {
				return nil, fmt.Errorf("failed to copy data: %w", err)
			}

			var manifest packageManifest

			_, err = plist.Unmarshal(data.Bytes(), &manifest)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal data: %w", err)
			}

			return &manifest, nil
		}
	}

	return nil, nil
}

func (*appstore) readBundleName(reader *zip.ReadCloser) (string, error) {
	var bundleName string

	for _, file := range reader.File {
		if strings.Contains(file.Name, ".app/Info.plist") && !strings.Contains(file.Name, "/Watch/") {
			bundleName = filepath.Base(strings.TrimSuffix(file.Name, ".app/Info.plist"))

			break
		}
	}

	if bundleName == "" {
		return "", errors.New("could not read bundle name")
	}

	return bundleName, nil
}
