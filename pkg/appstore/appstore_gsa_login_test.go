package appstore

import (
	"context"
	"errors"

	"github.com/majd/ipatool/v2/pkg/anisette"
	"github.com/majd/ipatool/v2/pkg/gsa"
	"github.com/majd/ipatool/v2/pkg/http"
	"github.com/majd/ipatool/v2/pkg/keychain"
	"github.com/majd/ipatool/v2/pkg/util/machine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

type fakeGSAClient struct {
	loginResult gsa.Account
	loginErr    error
	authResult  gsa.Account
	authErr     error

	gotEmail    string
	gotPassword string
	gotAuthCode string
	gotAnisette anisette.Data
	gotGUID     string
}

func (f *fakeGSAClient) Login(email, password string, a anisette.Data, authCode string) (gsa.Account, error) {
	f.gotEmail = email
	f.gotPassword = password
	f.gotAuthCode = authCode
	f.gotAnisette = a

	if f.loginErr != nil {
		return gsa.Account{}, f.loginErr
	}

	return f.loginResult, nil
}

func (f *fakeGSAClient) ItunesAuthenticate(acc gsa.Account, a anisette.Data, guid string) (gsa.Account, error) {
	f.gotGUID = guid

	if f.authErr != nil {
		return gsa.Account{}, f.authErr
	}

	return f.authResult, nil
}

type fakeAnisetteProvider struct {
	data anisette.Data
	err  error
}

func (p fakeAnisetteProvider) Fetch(context.Context) (anisette.Data, error) {
	return p.data, p.err
}

var _ = Describe("AppStore (GSA Login)", func() {
	const (
		testPassword = "test-password"
		testEmail    = "test-email"
	)

	var (
		ctrl          *gomock.Controller
		as            AppStore
		mockKeychain  *keychain.MockKeychain
		mockCookieJar *http.MockCookieJar
		mockMachine   *machine.MockMachine
		mockClient    *http.MockClient[loginResult]
		fakeGSA       *fakeGSAClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockKeychain = keychain.NewMockKeychain(ctrl)
		mockCookieJar = http.NewMockCookieJar(ctrl)
		mockMachine = machine.NewMockMachine(ctrl)
		mockClient = http.NewMockClient[loginResult](ctrl)
		fakeGSA = &fakeGSAClient{}

		as = &appstore{
			keychain:    mockKeychain,
			cookieJar:   mockCookieJar,
			loginClient: mockClient,
			machine:     mockMachine,
			gsa:         fakeGSA,
			anisette:    fakeAnisetteProvider{data: anisette.Data{OTP: "otp", MachineID: "machine-id"}},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	When("GSA login succeeds", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().MacAddress().Return("00:00:00:00:00:00", nil)
			mockCookieJar.EXPECT().Save().Return(nil)
			mockKeychain.EXPECT().Set("account", gomock.Any()).Return(nil)

			fakeGSA.loginResult = gsa.Account{
				Email:               testEmail,
				Name:                "Alice Smith",
				DirectoryServicesID: "12345",
				AdsID:               "12345",
				GsIDMSToken:         "idms-token",
				PETToken:            "pet-token",
			}
			fakeGSA.authResult = gsa.Account{
				Email:               testEmail,
				Name:                "Alice Smith",
				DirectoryServicesID: "12345",
				PasswordToken:       "store-token",
				StoreFront:          "143441-1,32",
				Pod:                 "51",
			}
		})

		It("exchanges the PET for a store password token and persists the session", func() {
			out, err := as.Login(LoginInput{
				Email:    testEmail,
				Password: testPassword,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Account.Email).To(Equal(testEmail))
			Expect(out.Account.PasswordToken).To(Equal("store-token"))
			Expect(out.Account.DirectoryServicesID).To(Equal("12345"))
			Expect(out.Account.StoreFront).To(Equal("143441-1,32"))
			Expect(out.Account.Pod).To(Equal("51"))
			Expect(fakeGSA.gotGUID).To(Equal("000000000000"))
			Expect(fakeGSA.gotEmail).To(Equal(testEmail))
			Expect(fakeGSA.gotPassword).To(Equal(testPassword))
			Expect(fakeGSA.gotAnisette.OTP).To(Equal("otp"))
		})
	})

	When("GSA requires an auth code", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().MacAddress().Return("00:00:00:00:00:00", nil)
			fakeGSA.loginErr = gsa.ErrAuthCodeRequired
		})

		It("returns ErrAuthCodeRequired without falling back", func() {
			_, err := as.Login(LoginInput{
				Email:    testEmail,
				Password: testPassword,
			})
			Expect(err).To(Equal(ErrAuthCodeRequired))
		})
	})

	When("GSA rejects the credentials", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().MacAddress().Return("00:00:00:00:00:00", nil)
			fakeGSA.loginErr = gsa.ErrBadCredentials
		})

		It("surfaces the credentials error without falling back", func() {
			_, err := as.Login(LoginInput{
				Email:    testEmail,
				Password: testPassword,
			})
			Expect(errors.Is(err, gsa.ErrBadCredentials)).To(BeTrue())
		})
	})

	When("anisette data is unavailable", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().MacAddress().Return("00:00:00:00:00:00", nil)
			as = &appstore{
				keychain:    mockKeychain,
				cookieJar:   mockCookieJar,
				loginClient: mockClient,
				machine:     mockMachine,
				gsa:         fakeGSA,
				anisette:    fakeAnisetteProvider{err: errors.New("no anisette")},
			}

			mockClient.EXPECT().
				Send(gomock.Any()).
				Return(http.Result[loginResult]{}, errors.New("legacy failed"))
		})

		It("falls back to the legacy login flow", func() {
			_, err := as.Login(LoginInput{
				Email:    testEmail,
				Password: testPassword,
			})
			Expect(err).To(MatchError("request failed: legacy failed"))
		})
	})

	When("the GSA client is not configured", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().MacAddress().Return("00:00:00:00:00:00", nil)
			as = &appstore{
				keychain:    mockKeychain,
				loginClient: mockClient,
				machine:     mockMachine,
			}

			mockClient.EXPECT().
				Send(gomock.Any()).
				Return(http.Result[loginResult]{}, errors.New("legacy failed"))
		})

		It("uses the legacy login flow", func() {
			_, err := as.Login(LoginInput{
				Email:    testEmail,
				Password: testPassword,
			})
			Expect(err).To(MatchError("request failed: legacy failed"))
		})
	})
})
