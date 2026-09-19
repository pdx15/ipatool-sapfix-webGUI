package appstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/majd/ipatool/v2/pkg/http"
	"github.com/majd/ipatool/v2/pkg/keychain"
	"github.com/majd/ipatool/v2/pkg/util/machine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("AppStore (Login)", func() {
	const (
		testPassword  = "test-password"
		testEmail     = "test-email"
		testFirstName = "test-first-name"
		testLastName  = "test-last-name"
		testPod       = "42"
	)

	var (
		ctrl         *gomock.Controller
		as           AppStore
		mockKeychain *keychain.MockKeychain
		mockClient   *http.MockClient[loginResult]
		mockMachine  *machine.MockMachine
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockKeychain = keychain.NewMockKeychain(ctrl)
		mockClient = http.NewMockClient[loginResult](ctrl)
		mockMachine = machine.NewMockMachine(ctrl)
		as = &appstore{
			keychain:    mockKeychain,
			loginClient: mockClient,
			machine:     mockMachine,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	DescribeTable("rejects malformed 2FA codes before preparing authentication", func(code string) {
		_, err := as.Login(LoginInput{AuthCode: code})

		Expect(err).To(MatchError("2FA code must contain exactly six digits"))
	},
		Entry("whitespace only", " \t\r\n"),
		Entry("too short", "12345"),
		Entry("too long", "1234567"),
		Entry("letters", "12345a"),
		Entry("non-ASCII digits", "１２３４５６"),
		Entry("other escape sequences", "\x1b[31m123456"),
		Entry("unmatched paste marker", "\x1b[200~123456"),
		Entry("empty paste", "\x1b[200~\x1b[201~"),
		Entry("embedded paste markers", "123\x1b[200~456\x1b[201~"),
	)
	When("fails to read Machine's MAC address", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("", errors.New(""))
		})

		It("returns error", func() {
			_, err := as.Login(LoginInput{
				Password: testPassword,
			})
			Expect(err).To(HaveOccurred())
		})
	})

	When("successfully reads machine's MAC address", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("00:00:00:00:00:00", nil)
		})

		DescribeTable("normalizes 2FA codes without changing the password", func(code, suffix string) {
			const password = " \tpäss word\n"
			mockClient.EXPECT().Send(gomock.Any()).DoAndReturn(func(req http.Request) (http.Result[loginResult], error) {
				Expect(req.Payload.(*http.XMLPayload).Content).To(HaveKeyWithValue("password", password+suffix))

				return http.Result[loginResult]{}, errors.New("test complete")
			})

			_, err := as.Login(LoginInput{Password: password, AuthCode: code})

			Expect(err).To(MatchError(ContainSubstring("test complete")))
		},
			Entry("no code on initial login", "", ""),
			Entry("plain code with leading zero", "012345", "012345"),
			Entry("spaces", "123 456", "123456"),
			Entry("Unicode whitespace", "\t123\u00a0456\r\n", "123456"),
			Entry("bracketed paste", "\x1b[200~123456\x1b[201~", "123456"),
			Entry("bracketed paste with whitespace", " \x1b[200~123 456\n\x1b[201~\r\n", "123456"),
		)

		When("client returns error", func() {
			BeforeEach(func() {
				mockClient.EXPECT().
					Send(gomock.Any()).
					Do(func(req http.Request) {
						Expect(req.SignAction).To(BeTrue())
					}).
					Return(http.Result[loginResult]{}, errors.New(""))
			})

			It("returns wrapped error", func() {
				_, err := as.Login(LoginInput{
					Password: testPassword,
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("normalizes the native authentication endpoint", func() {
			BeforeEach(func() {
				mockClient.EXPECT().
					Send(gomock.Any()).
					Do(func(req http.Request) {
						Expect(req.URL).To(Equal("https://auth.itunes.apple.com/auth/v1/native/fast/"))
					}).
					Return(http.Result[loginResult]{}, errors.New("stop"))
			})

			It("appends the trailing slash", func() {
				_, err := as.Login(LoginInput{
					Password: testPassword,
					Endpoint: "https://auth.itunes.apple.com/auth/v1/native/fast",
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("native authentication returns an empty response", func() {
			const podURL = "https://p7-buy.itunes.apple.com/WebObjects/MZFinance.woa/wa/authenticate?Pod=7&PRH=7"

			BeforeEach(func() {
				native := mockClient.EXPECT().
					Send(gomock.Any()).
					Do(func(req http.Request) {
						Expect(req.URL).To(Equal("https://auth.itunes.apple.com/auth/v1/native/fast/"))
					}).
					Return(http.Result[loginResult]{}, &http.UnexpectedResponseError{StatusCode: 204})
				legacy := mockClient.EXPECT().
					Send(gomock.Any()).
					Do(func(req http.Request) {
						Expect(req.URL).To(Equal(legacyAuthenticateEndpoint))
						payload := req.Payload.(*http.XMLPayload)
						Expect(payload.Content).To(HaveKeyWithValue("attempt", "1"))
					}).
					Return(http.Result[loginResult]{
						StatusCode: 302,
						Headers:    map[string]string{"Location": podURL},
					}, nil)
				pod := mockClient.EXPECT().
					Send(gomock.Any()).
					Do(func(req http.Request) {
						Expect(req.URL).To(Equal(podURL))
						payload := req.Payload.(*http.XMLPayload)
						Expect(payload.Content).To(HaveKeyWithValue("attempt", "1"))
					}).
					Return(http.Result[loginResult]{}, errors.New("stop after pod redirect"))
				gomock.InOrder(native, legacy, pod)
			})

			It("falls back to legacy authentication and reposts the plist to the assigned pod", func() {
				_, err := as.Login(LoginInput{
					Password: testPassword,
					Endpoint: "https://auth.itunes.apple.com/auth/v1/native/fast",
				})
				Expect(err).To(MatchError("request failed: stop after pod redirect"))
			})
		})
		When("store API returns invalid credentials", func() {
			BeforeEach(func() {
				mockClient.EXPECT().
					Send(gomock.Any()).
					Return(http.Result[loginResult]{
						Data: loginResult{
							FailureType: FailureTypeInvalidCredentials,
						},
					}, nil)
			})

			It("returns an error without retrying", func() {
				_, err := as.Login(LoginInput{
					Password: testPassword,
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("store API returns error", func() {
			BeforeEach(func() {
				mockClient.EXPECT().
					Send(gomock.Any()).
					Return(http.Result[loginResult]{
						Data: loginResult{
							FailureType: "random-error",
						},
					}, nil)
			})

			It("returns error", func() {
				_, err := as.Login(LoginInput{
					Password: testPassword,
				})
				Expect(err).To(HaveOccurred())
			})
		})

		When("store API indicates account is disabled", func() {
			BeforeEach(func() {
				mockClient.EXPECT().
					Send(gomock.Any()).
					Return(http.Result[loginResult]{
						Data: loginResult{
							CustomerMessage: CustomerMessageAccountDisabled,
						},
					}, nil)
			})

			It("returns account disabled error", func() {
				_, err := as.Login(LoginInput{
					Password: testPassword,
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("account is disabled"))
			})
		})

		When("store API requires 2FA code", func() {
			BeforeEach(func() {
				mockClient.EXPECT().
					Send(gomock.Any()).
					Return(http.Result[loginResult]{
						StatusCode: 200,
						Data: loginResult{
							FailureType:     "",
							CustomerMessage: CustomerMessageBadLogin,
						},
					}, nil)
			})

			It("returns ErrAuthCodeRequired error", func() {
				_, err := as.Login(LoginInput{
					Password: testPassword,
				})
				Expect(err).To(Equal(ErrAuthCodeRequired))
			})

			It("reports an incomplete verification when a code was already supplied", func() {
				_, err := as.Login(LoginInput{Password: testPassword, AuthCode: "123456"})

				Expect(err).To(MatchError("apple did not complete verification; try a fresh 2FA code"))
				Expect(errors.Is(err, ErrAuthCodeRequired)).To(BeFalse())
			})
		})

		When("store API redirects", func() {
			const (
				testRedirectLocation = "https://test-redirect-url.com"
			)

			BeforeEach(func() {
				firstCall := mockClient.EXPECT().
					Send(gomock.Any()).
					Do(func(req http.Request) {
						Expect(req.Payload).To(BeAssignableToTypeOf(&http.XMLPayload{}))
						x := req.Payload.(*http.XMLPayload)
						Expect(x.Content).To(HaveKeyWithValue("attempt", "1"))
					}).
					Return(http.Result[loginResult]{
						StatusCode: 302,
						Headers:    map[string]string{"Location": testRedirectLocation},
					}, nil)
				secondCall := mockClient.EXPECT().
					Send(gomock.Any()).
					Do(func(req http.Request) {
						Expect(req.URL).To(Equal(testRedirectLocation))
						Expect(req.Payload).To(BeAssignableToTypeOf(&http.XMLPayload{}))
						x := req.Payload.(*http.XMLPayload)
						Expect(x.Content).To(HaveKeyWithValue("attempt", "1"))
					}).
					Return(http.Result[loginResult]{}, errors.New("test complete"))
				gomock.InOrder(firstCall, secondCall)
			})

			It("follows the redirect while preserving the original request body", func() {
				_, err := as.Login(LoginInput{
					Password: testPassword,
				})
				Expect(err).To(MatchError("sign-in at Store pod request failed: test complete"))
			})
		})

		When("store API returns valid response", func() {
			const (
				testPasswordToken       = "test-password-token"
				testDirectoryServicesID = "directory-services-id"
				testStoreFront          = "test-storefront"
			)

			BeforeEach(func() {
				mockClient.EXPECT().
					Send(gomock.Any()).
					Return(http.Result[loginResult]{
						StatusCode: 200,
						Headers: map[string]string{
							HTTPHeaderStoreFront: testStoreFront,
							HTTPHeaderPod:        testPod,
						},
						Data: loginResult{
							PasswordToken:       testPasswordToken,
							DirectoryServicesID: testDirectoryServicesID,
							Account: loginAccountResult{
								Email: testEmail,
								Address: loginAddressResult{
									FirstName: testFirstName,
									LastName:  testLastName,
								},
							},
						},
					}, nil)
			})

			When("successfully saves account in keychain", func() {
				BeforeEach(func() {
					mockKeychain.EXPECT().
						Set("account", gomock.Any()).
						Do(func(key string, data []byte) {
							want := Account{
								Name:                fmt.Sprintf("%s %s", testFirstName, testLastName),
								Email:               testEmail,
								PasswordToken:       testPasswordToken,
								Password:            testPassword,
								DirectoryServicesID: testDirectoryServicesID,
								StoreFront:          testStoreFront,
								Pod:                 testPod,
							}

							var got Account
							Expect(json.Unmarshal(data, &got)).To(Succeed())
							Expect(got).To(Equal(want))
						}).
						Return(nil)
				})

				It("returns nil", func() {
					out, err := as.Login(LoginInput{
						Password: testPassword,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(out.Account.Email).To(Equal(testEmail))
					Expect(out.Account.Name).To(Equal(strings.Join([]string{testFirstName, testLastName}, " ")))
				})
			})
		})
	})
})
