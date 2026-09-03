package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/spf13/cobra"
)

// nolint:wrapcheck
func listPurchasesCmd() *cobra.Command {
	var (
		page       int
		maxResults int
		all        bool
	)

	cmd := &cobra.Command{
		Use:   "list-purchases",
		Short: "List apps owned by the authenticated App Store account",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if all {
				return nil
			}
			if page < 1 {
				return errors.New("page must be greater than 0")
			}
			if maxResults < 1 {
				return errors.New("max results must be greater than 0")
			}
			if maxResults > appstore.MaxOwnedAppsLimit {
				return fmt.Errorf("max results must not exceed %d", appstore.MaxOwnedAppsLimit)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var lastErr error

			return retry.Do(func() error {
				infoResult, err := dependencies.AppStore.AccountInfo()
				if err != nil {
					return err
				}

				acc := infoResult.Account
				if errors.Is(lastErr, appstore.ErrPasswordTokenExpired) {
					acc, err = refreshAccount(acc)
					if err != nil {
						return err
					}
				}

				output, err := dependencies.AppStore.OwnedApps(appstore.OwnedAppsInput{
					Account: acc,
					Page:    page,
					Limit:   maxResults,
					All:     all,
				})
				if err != nil {
					return err
				}

				dependencies.Logger.Log().
					Int("count", output.Count).
					Int("totalCount", output.TotalCount).
					Int("page", output.Page).
					Array("apps", appstore.Apps(output.Results)).
					Send()

				return nil
			},
				retry.LastErrorOnly(true),
				retry.DelayType(retry.FixedDelay),
				retry.Delay(time.Millisecond),
				retry.Attempts(2),
				retry.RetryIf(func(err error) bool {
					lastErr = err

					return errors.Is(err, appstore.ErrPasswordTokenExpired)
				}),
			)
		},
	}

	cmd.Flags().IntVarP(&maxResults, "max-results", "l", appstore.DefaultOwnedAppsLimit, "maximum number of apps to return per page")
	cmd.Flags().IntVarP(&page, "page", "p", 1, "page of owned apps to return")
	cmd.Flags().BoolVar(&all, "all", false, "return every owned app in one response (ignores --page and --max-results)")

	return cmd
}
