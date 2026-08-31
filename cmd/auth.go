package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with the App Store",
	}

	cmd.AddCommand(loginCmd())
	cmd.AddCommand(infoCmd())
	cmd.AddCommand(exportCmd())
	cmd.AddCommand(importCmd())
	cmd.AddCommand(revokeCmd())

	return cmd
}

func loginCmd() *cobra.Command {
	promptForAuthCode := func() (string, error) {
		authCode, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read string: %w", err)
		}

		authCode = strings.Trim(authCode, "\n")
		authCode = strings.Trim(authCode, "\r")

		return authCode, nil
	}

	var email, password, authCode, sessionOutput string
	var mzfinance bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to the App Store",
		RunE: func(cmd *cobra.Command, args []string) error {
			interactive := cmd.Context().Value(interactiveKey).(bool)

			if password == "" && !interactive {
				return errors.New("password is required when not running in interactive mode; use the \"--password\" flag")
			}

			if password == "" && interactive {
				dependencies.Logger.Log().Msg("enter password:")

				bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
				if err != nil {
					return fmt.Errorf("failed to read password: %w", err)
				}
				password = string(bytes)
			}

			var lastErr error

			// nolint:wrapcheck
			return retry.Do(func() error {
				if errors.Is(lastErr, appstore.ErrAuthCodeRequired) && interactive {
					dependencies.Logger.Log().Msg("enter 2FA code:")

					var err error
					authCode, err = promptForAuthCode()
					if err != nil {
						return fmt.Errorf("failed to read auth code: %w", err)
					}
				}

				dependencies.Logger.Verbose().
					Str("email", email).
					Bool("authCodeProvided", authCode != "").
					Bool("mzfinance", mzfinance).
					Msg("logging in")

				var (
					output appstore.LoginOutput
					err    error
				)
				if mzfinance {
					// Diagnostic login used by the macOS-only "Войти в Apple ID
					// ТЕСТ" button: GSA (public anisette) first, then the stable
					// legacy MZFinance authenticate flow — bypassing the glitchy
					// native/fast path, without changing the standard login.
					output, err = dependencies.AppStore.LoginMZFinance(appstore.LoginInput{
						Email:    email,
						Password: password,
						AuthCode: authCode,
					})
				} else {
					bag, bagErr := dependencies.AppStore.Bag(appstore.BagInput{})
					if bagErr != nil {
						return fmt.Errorf("failed to get bag: %w", bagErr)
					}

					output, err = dependencies.AppStore.Login(appstore.LoginInput{
						Email:    email,
						Password: password,
						AuthCode: authCode,
						Endpoint: bag.AuthEndpoint,
					})
				}
				if err != nil {
					if errors.Is(err, appstore.ErrAuthCodeRequired) && !interactive {
						dependencies.Logger.Log().Msg("2FA code is required; run the command again and supply a code using the `--auth-code` flag")

						return nil
					}

					return err
				}

				dependencies.Logger.Log().
					Str("name", output.Account.Name).
					Str("email", output.Account.Email).
					Bool("success", true).
					Send()
				if sessionOutput != "" {
					err := writeSessionFile(output.Account, sessionOutput)
					if err != nil {
						return err
					}
				}

				return nil
			},
				retry.LastErrorOnly(true),
				retry.DelayType(retry.FixedDelay),
				retry.Delay(time.Millisecond),
				retry.Attempts(2),
				retry.RetryIf(func(err error) bool {
					lastErr = err

					return errors.Is(err, appstore.ErrAuthCodeRequired)
				}),
			)
		},
	}

	cmd.Flags().StringVarP(&email, "email", "e", "", "email address for the Apple ID (required)")
	cmd.Flags().StringVarP(&password, "password", "p", "", "password for the Apple ID (required)")
	cmd.Flags().StringVar(&authCode, "auth-code", "", "2FA code for the Apple ID")
	cmd.Flags().StringVar(&sessionOutput, "session-output", "", "path to save the account session to after a successful login")
	cmd.Flags().BoolVar(&mzfinance, "mzfinance", false, "use the stable legacy MZFinance login flow (GSA -> MZFinance) instead of the default native/fast path")

	_ = cmd.MarkFlagRequired("email")

	return cmd
}

// nolint:wrapcheck
func infoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show current account info",
		RunE: func(cmd *cobra.Command, args []string) error {
			output, err := dependencies.AppStore.AccountInfo()
			if err != nil {
				return err
			}

			dependencies.Logger.Log().
				Str("name", output.Account.Name).
				Str("email", output.Account.Email).
				Bool("success", true).
				Send()

			return nil
		},
	}
}

// writeSessionFile stores the account session without the password so it can
// be imported on another machine or passed to automation.
func writeSessionFile(account appstore.Account, path string) error {
	account.Password = ""

	data, err := json.Marshal(account)
	if err != nil {
		return fmt.Errorf("failed to marshal account session: %w", err)
	}

	err = os.WriteFile(path, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write account session: %w", err)
	}

	return nil
}

// nolint:wrapcheck
func exportCmd() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the active account session for reuse on another machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			infoOutput, err := dependencies.AppStore.AccountInfo()
			if err != nil {
				return err
			}

			// Downloads and purchases only need the tokens issued by the App
			// Store, so the password is intentionally excluded from the export.
			account := infoOutput.Account
			account.Password = ""

			data, err := json.Marshal(account)
			if err != nil {
				return fmt.Errorf("failed to marshal account session: %w", err)
			}

			if outputPath == "" || outputPath == "-" {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				if err != nil {
					return fmt.Errorf("failed to write account session: %w", err)
				}

				return nil
			}

			err = writeSessionFile(infoOutput.Account, outputPath)
			if err != nil {
				return err
			}

			dependencies.Logger.Log().
				Str("output", outputPath).
				Bool("success", true).
				Send()

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "-", "destination path of the exported account session (defaults to stdout)")

	return cmd
}

// nolint:wrapcheck
func importCmd() *cobra.Command {
	var inputPath string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import an account session exported by \"ipatool auth export\"",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				data []byte
				err  error
			)

			if inputPath == "" || inputPath == "-" {
				data, err = io.ReadAll(cmd.InOrStdin())
			} else {
				data, err = os.ReadFile(inputPath)
			}

			if err != nil {
				return fmt.Errorf("failed to read account session: %w", err)
			}

			var account appstore.Account

			err = json.Unmarshal(data, &account)
			if err != nil {
				return fmt.Errorf("failed to parse account session: %w", err)
			}

			output, err := dependencies.AppStore.ImportAccount(appstore.ImportAccountInput{
				Account: account,
			})
			if err != nil {
				return err
			}

			dependencies.Logger.Log().
				Str("email", output.Account.Email).
				Bool("success", true).
				Send()

			return nil
		},
	}

	cmd.Flags().StringVarP(&inputPath, "input", "i", "-", "path to the account session to import (defaults to stdin)")

	return cmd
}

// nolint:wrapcheck
func revokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke",
		Short: "Revoke your App Store credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := dependencies.AppStore.Revoke()
			if err != nil {
				return err
			}

			dependencies.Logger.Log().Bool("success", true).Send()

			return nil
		},
	}
}
