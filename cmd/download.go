package cmd

import (
	"errors"
	"os"
	"time"

	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

// nolint:wrapcheck
func downloadCmd() *cobra.Command {
	var (
		acquireLicense    bool
		outputPath        string
		appID             int64
		bundleID          string
		externalVersionID string
		platformValue     string
	)

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download (encrypted) iOS and tvOS app packages from the App Store",
		RunE: func(cmd *cobra.Command, args []string) error {
			if appID == 0 && bundleID == "" {
				return errors.New("either the app ID or the bundle identifier must be specified")
			}

			infoResult, err := dependencies.AppStore.AccountInfo()
			if err != nil {
				return err
			}

			acc := infoResult.Account

			platform, err := appstore.ParsePlatform(platformValue)
			if err != nil {
				return err
			}

			app := appstore.App{ID: appID}
			if bundleID != "" {
				lookupResult, err := dependencies.AppStore.Lookup(appstore.LookupInput{
					Account:  acc,
					BundleID: bundleID,
					Platform: platform,
				})
				if err != nil {
					return err
				}

				app = lookupResult.App
			}

			interactive, _ := cmd.Context().Value(interactiveKey).(bool)
			var progress *progressbar.ProgressBar
			if interactive {
				progress = progressbar.NewOptions64(1,
					progressbar.OptionSetDescription("downloading"),
					progressbar.OptionSetWriter(os.Stdout),
					progressbar.OptionShowBytes(true),
					progressbar.OptionSetWidth(20),
					progressbar.OptionFullWidth(),
					progressbar.OptionThrottle(65*time.Millisecond),
					progressbar.OptionShowCount(),
					progressbar.OptionClearOnFinish(),
					progressbar.OptionSpinnerType(14),
					progressbar.OptionSetRenderBlankState(true),
					progressbar.OptionSetElapsedTime(false),
					progressbar.OptionSetPredictTime(false),
				)
			}

			_, out, purchased, err := downloadWithRetry(downloadTaskInput{
				Account:           acc,
				App:               app,
				OutputPath:        outputPath,
				Progress:          progress,
				ExternalVersionID: externalVersionID,
				Platform:          platform,
				AcquireLicense:    acquireLicense,
			})
			if err != nil {
				return err
			}

			err = dependencies.AppStore.ReplicateSinf(appstore.ReplicateSinfInput{Sinfs: out.Sinfs, PackagePath: out.DestinationPath})
			if err != nil {
				return err
			}

			dependencies.Logger.Log().
				Str("output", out.DestinationPath).
				Bool("purchased", purchased).
				Bool("success", true).
				Send()

			return nil
		},
	}

	cmd.Flags().Int64VarP(&appID, "app-id", "i", 0, "ID of the target iOS app (required)")
	cmd.Flags().StringVarP(&bundleID, "bundle-identifier", "b", "", "The bundle identifier of the target iOS app (overrides the app ID)")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "The destination path of the downloaded app package")
	cmd.Flags().StringVar(&externalVersionID, "external-version-id", "", "External version identifier of the target iOS app (defaults to latest version when not specified)")
	cmd.Flags().StringVar(&platformValue, "platform", "", "Platform to download for: iphone, ipad, or appletv")
	cmd.Flags().BoolVar(&acquireLicense, "purchase", false, "Obtain a license for the app if needed")

	return cmd
}
