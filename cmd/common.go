package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/byteness/keyring"
	cookiejar "github.com/juju/persistent-cookiejar"
	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/majd/ipatool/v2/pkg/http"
	"github.com/majd/ipatool/v2/pkg/keychain"
	"github.com/majd/ipatool/v2/pkg/log"
	"github.com/majd/ipatool/v2/pkg/util"
	"github.com/majd/ipatool/v2/pkg/util/machine"
	"github.com/majd/ipatool/v2/pkg/util/operatingsystem"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var dependencies = Dependencies{}
var keychainPassphrase string

type Dependencies struct {
	Logger    log.Logger
	OS        operatingsystem.OperatingSystem
	Machine   machine.Machine
	CookieJar http.CookieJar
	Keychain  keychain.Keychain
	AppStore  appstore.AppStore
}

// newLogger returns a new logger instance.
func newLogger(format OutputFormat, verbose bool) log.Logger {
	var writer io.Writer

	switch format {
	case OutputFormatJSON:
		writer = zerolog.SyncWriter(os.Stdout)
	case OutputFormatText:
		writer = log.NewWriter()
	}

	return log.NewLogger(log.Args{
		Verbose: verbose,
		Writer:  writer,
	},
	)
}

// newCookieJar returns a new cookie jar instance.
func newCookieJar(machine machine.Machine) http.CookieJar {
	return util.Must(cookiejar.New(&cookiejar.Options{
		Filename: filepath.Join(machine.HomeDirectory(), ConfigDirectoryName, CookieJarFileName),
	}))
}

// envSessionKeychain serves the account session from the IPATOOL_SESSION
// environment variable instead of the system keychain. Automation (e.g. a
// GitHub Actions runner) can reuse an exported session without any keychain
// access, which on headless machines may otherwise block waiting for a
// graphical confirmation prompt.
type envSessionKeychain struct {
	data []byte
}

func (k envSessionKeychain) Get(key string) ([]byte, error) {
	if key != "account" {
		return nil, errors.New("account session not found in IPATOOL_SESSION")
	}

	return k.data, nil
}

func (envSessionKeychain) Set(_ string, _ []byte) error {
	return nil
}

func (envSessionKeychain) Remove(_ string) error {
	return nil
}

// keychainPassphraseFile is the name of the file that stores the auto-generated
// keychain passphrase. Keeping it in the ipatool config directory means the
// session token stays decryptable across runs without prompting the user.
const keychainPassphraseFile = "keychain-passphrase"

// resolveKeychainPassphrase returns the passphrase used to encrypt the local
// keychain file. An explicitly provided --keychain-passphrase wins; otherwise a
// random passphrase is generated on first use and persisted in the ipatool
// config directory, so the user is never prompted for a separate local
// password. (The OS keyring backends, when present, are used in preference to
// the file backend anyway.)
func resolveKeychainPassphrase(machine machine.Machine) (string, error) {
	if keychainPassphrase != "" {
		return keychainPassphrase, nil
	}

	dir := filepath.Join(machine.HomeDirectory(), ConfigDirectoryName)
	path := filepath.Join(dir, keychainPassphraseFile)

	if data, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("failed to generate keychain passphrase: %w", err)
	}

	passphrase := hex.EncodeToString(random)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(passphrase+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("failed to save keychain passphrase: %w", err)
	}

	return passphrase, nil
}

// newKeychain returns a new keychain instance.
func newKeychain(machine machine.Machine) keychain.Keychain {
	if session := os.Getenv("IPATOOL_SESSION"); session != "" {
		return envSessionKeychain{data: []byte(session)}
	}

	passphrase, err := resolveKeychainPassphrase(machine)
	if err != nil {
		util.Must("", err)
	}

	ring := util.Must(keyring.Open(keyring.Config{
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,
			keyring.SecretServiceBackend,
			keyring.FileBackend,
		},
		ServiceName:       KeychainServiceName,
		FileDir:           filepath.Join(machine.HomeDirectory(), ConfigDirectoryName),
		FilePasswordFunc: func(s string) (string, error) {
			return passphrase, nil
		},
	}))

	return keychain.New(keychain.Args{Keyring: ring})
}

// initWithCommand initializes the dependencies of the command.
func initWithCommand(cmd *cobra.Command) {
	verbose := cmd.Flag("verbose").Value.String() == "true"
	format := util.Must(OutputFormatFromString(cmd.Flag("format").Value.String()))

	dependencies.Logger = newLogger(format, verbose)
	dependencies.OS = operatingsystem.New()
	dependencies.Machine = machine.New(machine.Args{OS: dependencies.OS})
	dependencies.CookieJar = newCookieJar(dependencies.Machine)
	dependencies.Keychain = newKeychain(dependencies.Machine)
	dependencies.AppStore = appstore.NewAppStore(appstore.Args{
		CookieJar:       dependencies.CookieJar,
		OperatingSystem: dependencies.OS,
		Keychain:        dependencies.Keychain,
		Machine:         dependencies.Machine,
	})

	util.Must("", createConfigDirectory(dependencies.OS, dependencies.Machine))
}

// createConfigDirectory creates the configuration directory for the CLI tool, if needed.
func createConfigDirectory(os operatingsystem.OperatingSystem, machine machine.Machine) error {
	configDirectoryPath := filepath.Join(machine.HomeDirectory(), ConfigDirectoryName)
	_, err := os.Stat(configDirectoryPath)

	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(configDirectoryPath, 0700)
		if err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("could not read metadata: %w", err)
	}

	return nil
}
