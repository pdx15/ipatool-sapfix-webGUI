package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/schollz/progressbar/v3"
)

// refreshAccount re-authenticates a stored account session whose password
// token has expired. This only works when the password from the original
// login is still stored in the keychain. Imported session files intentionally
// do not contain the password, so users must log in again in that case.
//
//nolint:wrapcheck
func refreshAccount(acc appstore.Account) (appstore.Account, error) {
	if acc.Password == "" {
		return acc, errors.New("password token is expired and no password is stored; log in again or import a fresh account session")
	}

	bagOutput, err := dependencies.AppStore.Bag(appstore.BagInput{})
	if err != nil {
		return acc, fmt.Errorf("failed to get bag: %w", err)
	}

	loginResult, err := dependencies.AppStore.Login(appstore.LoginInput{
		Email:    acc.Email,
		Password: acc.Password,
		Endpoint: bagOutput.AuthEndpoint,
	})
	if err != nil {
		return acc, err
	}

	return loginResult.Account, nil
}

// purchaseWithRetry acquires a free app license. When the App Store reports
// that the cached password token is expired, it logs in again once with the
// stored password and retries the purchase. It returns the account that should
// be used by subsequent requests and whether the license was already owned.
//
//nolint:wrapcheck
func purchaseWithRetry(acc appstore.Account, app appstore.App) (appstore.Account, bool, error) {
	refreshed := false

	for attempt := 0; attempt < 2; attempt++ {
		err := dependencies.AppStore.Purchase(appstore.PurchaseInput{Account: acc, App: app})
		if errors.Is(err, appstore.ErrLicenseAlreadyExists) {
			return acc, true, nil
		}
		if err == nil {
			return acc, false, nil
		}

		if errors.Is(err, appstore.ErrPasswordTokenExpired) && !refreshed {
			if acc.Password == "" {
				return acc, false, errors.New("password token is expired and no password is stored; log in again or import a fresh account session")
			}

			refreshedAcc, refreshErr := refreshAccount(acc)
			if refreshErr != nil {
				return acc, false, refreshErr
			}

			acc = refreshedAcc
			refreshed = true
			continue
		}

		return acc, false, err
	}

	return acc, false, errors.New("failed to purchase after re-authentication")
}

// checkDownloadWithRetry performs the direct-download probe and refreshes the
// stored session once if Apple reports an expired password token.
//
//nolint:wrapcheck
func checkDownloadWithRetry(acc appstore.Account, app appstore.App, platform appstore.Platform) (appstore.Account, appstore.CheckDownloadOutput, error) {
	refreshed := false

	for attempt := 0; attempt < 3; attempt++ {
		out, err := dependencies.AppStore.CheckDownload(appstore.CheckDownloadInput{
			Account:  acc,
			App:      app,
			Platform: platform,
		})
		if err == nil {
			return acc, out, nil
		}

		if errors.Is(err, appstore.ErrPasswordTokenExpired) && !refreshed {
			if acc.Password == "" {
				return acc, appstore.CheckDownloadOutput{}, errors.New("password token is expired and no password is stored; log in again or import a fresh account session")
			}

			refreshedAcc, refreshErr := refreshAccount(acc)
			if refreshErr != nil {
				return acc, appstore.CheckDownloadOutput{}, refreshErr
			}

			acc = refreshedAcc
			refreshed = true
			continue
		}

		return acc, appstore.CheckDownloadOutput{}, err
	}

	return acc, appstore.CheckDownloadOutput{}, errors.New("too many download check attempts")
}

// downloadTaskInput carries the parameters used by downloadWithRetry.
type downloadTaskInput struct {
	Account           appstore.Account
	App               appstore.App
	OutputPath        string
	Progress          *progressbar.ProgressBar
	ExternalVersionID string
	Platform          appstore.Platform
	ProgressWriter    io.Writer
	OnTotalBytes      func(total int64)
	AcquireLicense    bool
}

// downloadWithRetry downloads an app package with automatic session refresh.
// It also acquires the license on demand when Apple returns
// ErrLicenseRequired and AcquireLicense is set, so the common case of
// "never downloaded with this Apple ID + expired token" is handled in one
// retry sequence instead of surfacing a bare "license is required" error.
//
//nolint:wrapcheck
func downloadWithRetry(input downloadTaskInput) (appstore.Account, appstore.DownloadOutput, bool, error) {
	acc := input.Account
	purchased := false
	refreshed := false

	for attempt := 0; attempt < 4; attempt++ {
		out, err := dependencies.AppStore.Download(appstore.DownloadInput{
			Account:           acc,
			App:               input.App,
			OutputPath:        input.OutputPath,
			Progress:          input.Progress,
			ExternalVersionID: input.ExternalVersionID,
			Platform:          input.Platform,
			ProgressWriter:    input.ProgressWriter,
			OnTotalBytes:      input.OnTotalBytes,
		})
		if err == nil {
			return acc, out, purchased, nil
		}

		if errors.Is(err, appstore.ErrPasswordTokenExpired) {
			if acc.Password == "" {
				return acc, appstore.DownloadOutput{}, purchased, errors.New("password token is expired and no password is stored; log in again or import a fresh account session")
			}
			if refreshed {
				return acc, appstore.DownloadOutput{}, purchased, err
			}

			refreshedAcc, refreshErr := refreshAccount(acc)
			if refreshErr != nil {
				return acc, appstore.DownloadOutput{}, purchased, refreshErr
			}

			acc = refreshedAcc
			refreshed = true
			continue
		}

		if errors.Is(err, appstore.ErrLicenseRequired) && input.AcquireLicense {
			if purchased {
				return acc, appstore.DownloadOutput{}, purchased, err
			}

			purchasedAcc, _, purchaseErr := purchaseWithRetry(acc, input.App)
			if purchaseErr != nil {
				return acc, appstore.DownloadOutput{}, purchased, purchaseErr
			}

			acc = purchasedAcc
			purchased = true
			continue
		}

		return acc, appstore.DownloadOutput{}, purchased, err
	}

	return acc, appstore.DownloadOutput{}, purchased, errors.New("too many download attempts")
}
