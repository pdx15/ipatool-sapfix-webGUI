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
								Sinfs: []Sinf{{ID: 0, Data: []byte("sinf")}},
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

	When("the primary endpoint returns an empty Items[] (e.g. Google Chrome)", func() {
		const testGUID = "001122334455"

		BeforeEach(func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("00:11:22:33:44:55", nil)

			// Primary endpoint: empty response twice (first try + one retry).
			mockDownloadClient.EXPECT().
				Send(gomock.Any()).
				Do(func(req http.Request) {
					Expect(req.URL).To(ContainSubstring(PrivateAppStoreAPIPathDownload))
				}).
				Return(http.Result[downloadResult]{Data: downloadResult{}}, nil).
				Times(2)

			// Redownload fallback answers with the real metadata.
			mockDownloadClient.EXPECT().
				Send(gomock.Any()).
				Do(func(req http.Request) {
					Expect(req.URL).To(Equal("https://downloaddispatch.itunes.apple.com/r/redownload?guid=" + testGUID))

					payload, ok := req.Payload.(*http.XMLPayload)
					Expect(ok).To(BeTrue())
					Expect(payload.Content["salableAdamId"]).To(Equal(int64(535886823)))
				}).
				Return(http.Result[downloadResult]{
					Data: downloadResult{
						Items: []downloadItemResult{
							{
								URL:   "https://cdn/chrome.ipa",
								Sinfs: []Sinf{{ID: 0, Data: []byte("sinf")}},
								Metadata: map[string]interface{}{
									"bundleShortVersionString":          "140.0",
									"softwareVersionExternalIdentifier": "876543210",
								},
							},
						},
					},
				}, nil)
		})

		It("falls back to the redownload endpoint like Download does", func() {
			out, err := as.CheckDownload(CheckDownloadInput{
				App: App{ID: 535886823},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Version).To(Equal("140.0"))
			Expect(out.LatestExternalVersionID).To(Equal("876543210"))
		})
	})

	When("the descriptor has no sinfs (unsigned package)", func() {
		itemWithout := downloadItemResult{URL: "https://cdn/without.ipa", Metadata: map[string]interface{}{"bundleShortVersionString": "1.0"}}
		itemWith := downloadItemResult{URL: "https://cdn/with.ipa", Sinfs: []Sinf{{ID: 0, Data: []byte("sinf")}}, Metadata: map[string]interface{}{"bundleShortVersionString": "2.0"}}
		reply := func(item downloadItemResult) http.Result[downloadResult] {
			return http.Result[downloadResult]{Data: downloadResult{Items: []downloadItemResult{item}}}
		}

		BeforeEach(func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("00:11:22:33:44:55", nil)
		})

		It("re-asks the primary endpoint and uses the signed descriptor", func() {
			first := mockDownloadClient.EXPECT().Send(gomock.Any()).Return(reply(itemWithout), nil)
			mockDownloadClient.EXPECT().Send(gomock.Any()).Return(reply(itemWith), nil).After(first)

			out, err := as.CheckDownload(CheckDownloadInput{App: App{ID: 284815942}})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Version).To(Equal("2.0"))
		})

		It("falls back to the redownload endpoint for a signed descriptor", func() {
			first := mockDownloadClient.EXPECT().Send(gomock.Any()).Return(reply(itemWithout), nil)
			second := mockDownloadClient.EXPECT().Send(gomock.Any()).Return(reply(itemWithout), nil).After(first)
			mockDownloadClient.EXPECT().Send(gomock.Any()).
				Do(func(req http.Request) {
					Expect(req.URL).To(HavePrefix("https://downloaddispatch.itunes.apple.com/r/redownload"))
				}).
				Return(reply(itemWith), nil).
				After(second)

			out, err := as.CheckDownload(CheckDownloadInput{App: App{ID: 284815942}})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Version).To(Equal("2.0"))
		})

		It("keeps the unsigned descriptor when no endpoint returns sinfs", func() {
			mockDownloadClient.EXPECT().
				Send(gomock.Any()).
				Return(reply(itemWithout), nil).
				Times(3)

			out, err := as.CheckDownload(CheckDownloadInput{App: App{ID: 284815942}})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Version).To(Equal("1.0"))
		})
	})

	When("both endpoints return an empty Items[]", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("00:11:22:33:44:55", nil)

			mockDownloadClient.EXPECT().
				Send(gomock.Any()).
				Return(http.Result[downloadResult]{Data: downloadResult{}}, nil).
				Times(3)
		})

		It("reports that both endpoints failed", func() {
			_, err := as.CheckDownload(CheckDownloadInput{App: App{ID: 1}})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("both download endpoints failed"))
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
