package cmd

import (
	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/spf13/cobra"
)

// nolint:wrapcheck
func purchaseCmd() *cobra.Command {
	var bundleID string

	cmd := &cobra.Command{
		Use:   "purchase",
		Short: "Obtain a license for the app from the App Store",
		RunE: func(cmd *cobra.Command, args []string) error {
			infoResult, err := dependencies.AppStore.AccountInfo()
			if err != nil {
				return err
			}

			acc := infoResult.Account

			lookupResult, err := dependencies.AppStore.Lookup(appstore.LookupInput{
				Account:  acc,
				BundleID: bundleID,
			})
			if err != nil {
				return err
			}

			_, alreadyOwned, err := purchaseWithRetry(acc, lookupResult.App)
			if err != nil {
				return err
			}

			dependencies.Logger.Log().
				Bool("alreadyOwned", alreadyOwned).
				Bool("success", true).
				Send()

			return nil
		},
	}

	cmd.Flags().StringVarP(&bundleID, "bundle-identifier", "b", "", "Bundle identifier of the target iOS app (required)")
	_ = cmd.MarkFlagRequired("bundle-identifier")

	return cmd
}
