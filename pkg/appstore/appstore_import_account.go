package appstore

import (
	"encoding/json"
	"errors"
	"fmt"
)

type ImportAccountInput struct {
	Account Account
}

type ImportAccountOutput struct {
	Account Account
}

// ImportAccount stores an existing account session in the keychain, e.g. one
// exported on another machine using "ipatool auth export". The long-lived
// tokens issued by the App Store can then be reused for purchases and
// downloads without signing in again.
func (t *appstore) ImportAccount(input ImportAccountInput) (ImportAccountOutput, error) {
	if input.Account.Email == "" {
		return ImportAccountOutput{}, errors.New("email is required")
	}

	if input.Account.PasswordToken == "" {
		return ImportAccountOutput{}, errors.New("password token is required")
	}

	if input.Account.DirectoryServicesID == "" {
		return ImportAccountOutput{}, errors.New("directory services identifier is required")
	}

	if input.Account.StoreFront == "" {
		return ImportAccountOutput{}, errors.New("storefront is required")
	}

	data, err := json.Marshal(input.Account)
	if err != nil {
		return ImportAccountOutput{}, fmt.Errorf("failed to marshal json: %w", err)
	}

	err = t.keychain.Set("account", data)
	if err != nil {
		return ImportAccountOutput{}, fmt.Errorf("failed to save account in keychain: %w", err)
	}

	return ImportAccountOutput{
		Account: input.Account,
	}, nil
}
