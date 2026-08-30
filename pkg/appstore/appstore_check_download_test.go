package appstore

import (
	"errors"
	"fmt"

	"github.com/majd/ipatool/v2/pkg/http"
	"github.com/majd/ipatool/v2/pkg/keychain"
	"github.com/majd/ipatool/v2/pkg/util/machine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("AppStore (CheckDownload)", func() {
	var (
		ctrl               *gomock.Controller
		mockDownloadClient *http.MockClient[downloadResult]
		mockPlatformClient *http.MockClient[platformVersionLookupResult]
		mockMachine        *machine.MockMachine
		as                 AppStore
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockDownloadClient = http.NewMockClient[downloadResult](ctrl)
		mockPlatformClient = http.NewMockClient[platformVersionLookupResult](ctrl)
		mockMachine = machine.NewMockMachine(ctrl)
		as = &appstore{
			keychain:       keychain.NewMockKeychain(ctrl),
			downloadClient: mockDownloadClient,
			platformClient: mockPlatformClient,
			machine:        mockMachine,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	When("fails to get MAC address", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("", errors.New(""))
		})

		It("returns error", func() {
			_, err := as.CheckDownload(CheckDownloadInput{})
			Expect(err).To(HaveOccurred())
		})
	})

	When("request fails", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("00:00:00:00:00:00", nil)

			mockDownloadClient.EXPECT().
				Send(gomock.Any()).
				Return(http.Result[downloadResult]{}, errors.New(""))
		})

		It("returns error", func() {
			_, err := as.CheckDownload(CheckDownloadInput{})
			Expect(err).To(HaveOccurred())
		})
	})

	When("license is not found", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("00:00:00:00:00:00", nil)

			mockDownloadClient.EXPECT().
				Send(gomock.Any()).
				Return(http.Result[downloadResult]{
					Data: downloadResult{
						FailureType: FailureTypeLicenseNotFound,
					},
				}, nil)
		})

		It("returns ErrLicenseRequired", func() {
			_, err := as.CheckDownload(CheckDownloadInput{})
			Expect(errors.Is(err, ErrLicenseRequired)).To(BeTrue())
		})
	})

	When("password token is expired", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("00:00:00:00:00:00", nil)

			mockDownloadClient.EXPECT().
				Send(gomock.Any()).
				Return(http.Result[downloadResult]{
					Data: downloadResult{
						FailureType: FailureTypePasswordTokenExpired,
					},
				}, nil)
		})

		It("returns ErrPasswordTokenExpired", func() {
			_, err := as.CheckDownload(CheckDownloadInput{})
			Expect(errors.Is(err, ErrPasswordTokenExpired)).To(BeTrue())
		})
	})

	When("the account holds a license", func() {
		const (
			testPod        = "42"
			testGUID       = "001122334455"
			testVersionID1 = "12345678"
			testVersionID2 = "87654321"
		)

		BeforeEach(func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("00:11:22:33:44:55", nil)

			mockDownloadClient.EXPECT().
				Send(gomock.Any()).
				Do(func(req http.Request) {
					expectedURL := "https://p" + testPod + "-" + PrivateAppStoreAPIDomain + PrivateAppStoreAPIPathDownload + "?guid=" + testGUID
					Expect(req.URL).To(Equal(expectedURL))

					payload, ok := req.Payload.(*http.XMLPayload)
					Expect(ok).To(BeTrue())
					Expect(payload.Content["salableAdamId"]).To(Equal(int64(568903335)))
					Expect(payload.Content).ToNot(HaveKey("externalVersionId"))
				}).
				Return(http.Result[downloadResult]{
					Data: downloadResult{
						Items: []downloadItemResult{
							{
								Metadata: map[string]interface{}{
									"bundleShortVersionString":           "7.8.1",
									"softwareVersionExternalIdentifiers": []interface{}{testVersionID1, testVersionID2},
									"softwareVersionExternalIdentifier":  testVersionID1,
								},
							},
						},
					},
				}, nil)
		})

		It("returns the version metadata without downloading the package", func() {
			out, err := as.CheckDownload(CheckDownloadInput{
				Account: Account{
					Pod: testPod,
				},
				App: App{
					ID: 568903335,
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Version).To(Equal("7.8.1"))
			Expect(out.ExternalVersionIdentifiers).To(Equal([]string{testVersionID1, testVersionID2}))
			Expect(out.LatestExternalVersionID).To(Equal(testVersionID1))
		})
	})

	When("platform is AppleTV", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("00:11:22:33:44:55", nil)

			mockPlatformClient.EXPECT().
				Send(gomock.Any()).
				Return(http.Result[platformVersionLookupResult]{
					StatusCode: 200,
					Data: platformVersionLookupResult{
						Results: map[string]platformVersionLookupItem{
							"42": {
								Offers: []platformVersionLookupOffer{
									{
										Version: platformVersionLookupVersion{
											ExternalID: platformVersionExternalID("123456"),
										},
									},
								},
							},
						},
					},
				}, nil)

			mockDownloadClient.EXPECT().
				Send(gomock.Any()).
				Do(func(req http.Request) {
					payload, ok := req.Payload.(*http.XMLPayload)
					Expect(ok).To(BeTrue())
					Expect(payload.Content["externalVersionId"]).To(Equal("123456"))
				}).
				Return(http.Result[downloadResult]{}, errors.New("request error"))
		})

		It("resolves and sends the tvOS external version id", func() {
			_, err := as.CheckDownload(CheckDownloadInput{
				Account: Account{
					StoreFront: "143441",
				},
				App: App{
					ID: 42,
				},
				Platform: PlatformAppleTV,
			})
			Expect(err).To(MatchError(fmt.Sprintf("failed to send http request: %s", "request error")))
		})
	})
})
