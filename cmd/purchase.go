package cmd

import (
	"errors"

	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/spf13/cobra"
)

// nolint:wrapcheck
func purchaseCmd() *cobra.Command {
	var (
		appID    int64
		bundleID string
	)

	cmd := &cobra.Command{
		Use:   "purchase",
		Short: "Obtain a license for the app from the App Store",
		RunE: func(cmd *cobra.Command, args []string) error {
			if appID == 0 && bundleID == "" {
				return errors.New("either the app ID or the bundle identifier must be specified")
			}

			infoResult, err := dependencies.AppStore.AccountInfo()
			if err != nil {
				return err
			}

			acc := infoResult.Account

			// The app is always resolved through the iTunes lookup API, even
			// when only the numeric ID is given: Purchase needs the price to
			// refuse paid apps, and the bundle identifier is useful in the log.
			lookupInput := appstore.LookupInput{Account: acc}
			if bundleID != "" {
				lookupInput.BundleID = bundleID
			} else {
				lookupInput.AppID = appID
			}

			lookupResult, err := dependencies.AppStore.Lookup(lookupInput)
			if err != nil {
				return err
			}

			_, alreadyOwned, err := purchaseWithRetry(acc, lookupResult.App)
			if err != nil {
				return err
			}

			dependencies.Logger.Log().
				Int64("appId", lookupResult.App.ID).
				Str("bundleId", lookupResult.App.BundleID).
				Str("name", lookupResult.App.Name).
				Bool("alreadyOwned", alreadyOwned).
				Bool("success", true).
				Send()

			return nil
		},
	}

	cmd.Flags().Int64VarP(&appID, "app-id", "i", 0, "ID of the target iOS app")
	cmd.Flags().StringVarP(&bundleID, "bundle-identifier", "b", "", "Bundle identifier of the target iOS app (overrides the app ID)")

	return cmd
}
