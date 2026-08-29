//go:build windows

package mescal

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// sapsignerEnvVar overrides the auto-detected path of the SAP signer binary
// (the "sapsigner.exe" helper bundled with Signum).
const sapsignerEnvVar = "IPATOOL_SAPSIGNER"

// ErrSapsignerNotFound indicates that no SAP signer binary could be located.
var ErrSapsignerNotFound = errors.New(
	"the Apple SAP signing helper (sapsigner.exe) was not found; install Signum " +
		"(https://altstore.io/) or set IPATOOL_SAPSIGNER to the full path of sapsigner.exe",
)

// overrideSapsignerPath, when non-empty, takes precedence over auto-detection.
// It is a package-level variable so callers (tests, the GUI) can point the
// signer at a specific binary without recompiling.
var overrideSapsignerPath string

// SetSapsignerPath pins the SAP signer binary used on Windows. An empty path
// restores auto-detection.
func SetSapsignerPath(path string) {
	overrideSapsignerPath = path
}

// sapsignerCandidates lists the Signum install locations that are probed, in
// order, when no explicit path is configured. Both the current "v3-legacy"
// layout and the older "v2" layout are checked.
func sapsignerCandidates() []string {
	local := os.Getenv("LOCALAPPDATA")
	base := filepath.Join(local, "Signum", "resources", "apple-tools", "windows-x64")
	return []string{
		filepath.Join(base, "v3-legacy", "sapsigner.exe"),
		filepath.Join(base, "v2", "sapsigner.exe"),
		filepath.Join(base, "sapsigner.exe"),
	}
}

// resolveSapsigner returns the path of the SAP signer binary, honoring the
// IPATOOL_SAPSIGNER environment variable, the package-level override, and
// finally the default Signum install locations.
func resolveSapsigner() (string, error) {
	if env := strings.TrimSpace(os.Getenv(sapsignerEnvVar)); env != "" {
		if info, err := os.Stat(env); err == nil && !info.IsDir() {
			return env, nil
		}

		return "", fmt.Errorf("%w: IPATOOL_SAPSIGNER does not point to a file: %q", ErrSapsignerNotFound, env)
	}

	if overrideSapsignerPath != "" {
		if info, err := os.Stat(overrideSapsignerPath); err == nil && !info.IsDir() {
			return overrideSapsignerPath, nil
		}

		return "", fmt.Errorf("%w: configured signer does not exist: %q", ErrSapsignerNotFound, overrideSapsignerPath)
	}

	for _, candidate := range sapsignerCandidates() {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", ErrSapsignerNotFound
}

// Sign creates the binary SAP signature Apple expects for protected Store
// actions by invoking the bundled "sapsigner.exe" helper on Windows. The
// helper reads the request body from stdin and writes the binary signature to
// stdout (the "-i -" / "-o -" defaults), so the caller only passes the data to
// be signed. This mirrors the macOS CommerceKit signing service.
func Sign(data []byte) ([]byte, error) {
	path, err := resolveSapsigner()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(path)
	// The helper loads its Apple SAP frameworks from a "sap-cache" directory
	// that lives next to the binary, so run it from its own directory.
	cmd.Dir = filepath.Dir(path)
	cmd.Stdin = bytes.NewReader(data)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("sapsigner failed: %w: %s", err, msg)
		}

		return nil, fmt.Errorf("sapsigner failed: %w", err)
	}

	if stdout.Len() == 0 {
		return nil, errors.New("sapsigner returned an empty SAP signature")
	}

	return stdout.Bytes(), nil
}
