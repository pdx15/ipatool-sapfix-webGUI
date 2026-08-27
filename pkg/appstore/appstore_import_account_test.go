package appstore

import (
	"encoding/json"
	"errors"

	"github.com/majd/ipatool/v2/pkg/keychain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("AppStore (ImportAccount)", func() {
	const (
		testEmail               = "test-email"
		testPasswordToken       = "test-password-token"
		testDirectoryServicesID = "test-directory-services-id"
		testStoreFront          = "test-storefront"
	)

	var (
		ctrl         *gomock.Controller
		as           AppStore
		mockKeychain *keychain.MockKeychain
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockKeychain = keychain.NewMockKeychain(ctrl)
		as = &appstore{
			keychain: mockKeychain,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	newValidAccount := func() Account {
		return Account{
			Email:               testEmail,
			PasswordToken:       testPasswordToken,
			DirectoryServicesID: testDirectoryServicesID,
			StoreFront:          testStoreFront,
		}
	}

	When("account is missing the email address", func() {
		It("returns error", func() {
			account := newValidAccount()
			account.Email = ""

			_, err := as.ImportAccount(ImportAccountInput{
				Account: account,
			})
			Expect(err).To(MatchError("email is required"))
		})
	})

	When("account is missing the password token", func() {
		It("returns error", func() {
			account := newValidAccount()
			account.PasswordToken = ""

			_, err := as.ImportAccount(ImportAccountInput{
				Account: account,
			})
			Expect(err).To(MatchError("password token is required"))
		})
	})

	When("account is missing the directory services identifier", func() {
		It("returns error", func() {
			account := newValidAccount()
			account.DirectoryServicesID = ""

			_, err := as.ImportAccount(ImportAccountInput{
				Account: account,
			})
			Expect(err).To(MatchError("directory services identifier is required"))
		})
	})

	When("account is missing the storefront", func() {
		It("returns error", func() {
			account := newValidAccount()
			account.StoreFront = ""

			_, err := as.ImportAccount(ImportAccountInput{
				Account: account,
			})
			Expect(err).To(MatchError("storefront is required"))
		})
	})

	When("keychain returns error", func() {
		It("returns wrapped error", func() {
			mockKeychain.EXPECT().
				Set("account", gomock.Any()).
				Return(errors.New(""))

			_, err := as.ImportAccount(ImportAccountInput{
				Account: newValidAccount(),
			})
			Expect(err).To(HaveOccurred())
		})
	})

	When("keychain stores the account", func() {
		It("stores the marshalled account and returns it", func() {
			account := newValidAccount()

			var stored []byte

			mockKeychain.EXPECT().
				Set("account", gomock.Any()).
				Do(func(_ string, data []byte) {
					stored = data
				}).
				Return(nil)

			out, err := as.ImportAccount(ImportAccountInput{
				Account: account,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Account).To(Equal(account))

			var restored Account

			err = json.Unmarshal(stored, &restored)
			Expect(err).ToNot(HaveOccurred())
			Expect(restored).To(Equal(account))
		})
	})
})
