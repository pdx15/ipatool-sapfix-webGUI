package appstore

import (
	"errors"
	gohttp "net/http"
	"net/url"
	"strconv"

	"github.com/majd/ipatool/v2/pkg/http"
	"github.com/majd/ipatool/v2/pkg/util/machine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

const wellKnownRedownloadURL = "https://downloaddispatch.itunes.apple.com/r/redownload"

var _ = Describe("redownload fallback (issues #538/#547)", func() {
	const (
		testGUID      = "001122334455"
		testVersionID = "890676338"
	)

	var (
		ctrl               *gomock.Controller
		mockMachine        *machine.MockMachine
		mockBagClient      *http.MockClient[bagResult]
		mockDownloadClient *http.MockClient[downloadResult]
		mockPlatformClient *http.MockClient[platformVersionLookupResult]
		as                 *appstore
		account            Account
		app                App
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockMachine = machine.NewMockMachine(ctrl)
		mockBagClient = http.NewMockClient[bagResult](ctrl)
		mockDownloadClient = http.NewMockClient[downloadResult](ctrl)
		mockPlatformClient = http.NewMockClient[platformVersionLookupResult](ctrl)
		as = &appstore{
			machine:        mockMachine,
			bagClient:      mockBagClient,
			downloadClient: mockDownloadClient,
			platformClient: mockPlatformClient,
		}
		account = Account{
			DirectoryServicesID: "test-dsid",
			Pod:                 "42",
			StoreFront:          "143441-1,34", // US storefront
		}
		app = App{ID: 6472431552}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var emptyPrimary = http.Result[downloadResult]{StatusCode: gohttp.StatusOK}

	signedItem := func() downloadItemResult {
		return downloadItemResult{
			URL:   "https://cdn.example.com/karing.ipa",
			Sinfs: []Sinf{{ID: 0, Data: []byte("sinf")}},
			Metadata: map[string]interface{}{
				"bundleShortVersionString":          "1.2.24",
				"softwareVersionExternalIdentifier": testVersionID,
			},
		}
	}

	listItem := func() downloadItemResult {
		return downloadItemResult{
			URL: "https://cdn.example.com/karing.ipa",
			Metadata: map[string]interface{}{
				"softwareVersionExternalIdentifier":  testVersionID,
				"softwareVersionExternalIdentifiers": []interface{}{"890000001", testVersionID},
			},
		}
	}

	lookupWith := func(versionID string) platformVersionLookupResult {
		return platformVersionLookupResult{
			Results: map[string]platformVersionLookupItem{
				strconv.FormatInt(app.ID, 10): {
					Offers: []platformVersionLookupOffer{
						{Version: platformVersionLookupVersion{ExternalID: platformVersionExternalID(versionID)}},
					},
				},
			},
		}
	}

	emptyBag := func() http.Result[bagResult] {
		return http.Result[bagResult]{StatusCode: gohttp.StatusOK, Data: bagResult{}}
	}

	bagWithEndpoint := func() http.Result[bagResult] {
		return http.Result[bagResult]{
			StatusCode: gohttp.StatusOK,
			Data: bagResult{
				URLBag: urlBag{
					AuthEndpoint:       "https://buy.itunes.apple.com/WebObjects/MZFinance.woa/wa/authenticate",
					RedownloadEndpoint: wellKnownRedownloadURL,
				},
			},
		}
	}

	// emptyVolumeStoreExpectations models the primary endpoint answering HTTP 200
	// with an empty Items[] twice (first try + one retry), which is when the
	// redownload fallback kicks in.
	emptyVolumeStoreExpectations := func() *gomock.Call {
		first := mockDownloadClient.EXPECT().Send(gomock.Any()).Return(emptyPrimary, nil)
		return mockDownloadClient.EXPECT().Send(gomock.Any()).Return(emptyPrimary, nil).After(first)
	}

	DescribeTable("validates redownload endpoints",
		func(endpoint string, valid bool) {
			_, err := newRedownloadEndpoint(endpoint)
			if valid {
				Expect(err).ToNot(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},
		Entry("valid Apple endpoint", wellKnownRedownloadURL, true),
		Entry("non-HTTPS endpoint", "http://downloaddispatch.itunes.apple.com/r/redownload", false),
		Entry("unexpected host", "https://example.com/r/redownload", false),
		Entry("unexpected path", "https://downloaddispatch.itunes.apple.com/r/other", false),
	)

	It("uses the volumeStore endpoint and does not touch redownload for a normal response", func() {
		mockDownloadClient.EXPECT().
			Send(gomock.Any()).
			Do(func(req http.Request) {
				Expect(req.URL).To(Equal("https://p42-buy.itunes.apple.com/WebObjects/MZFinance.woa/wa/volumeStoreDownloadProduct?guid=" + testGUID))
				Expect(req.Headers).To(HaveKeyWithValue("iCloud-DSID", account.DirectoryServicesID))

				payload, ok := req.Payload.(*http.XMLPayload)
				Expect(ok).To(BeTrue())
				Expect(payload.Content).To(HaveKeyWithValue("externalVersionId", testVersionID))
				Expect(payload.Content).To(HaveKeyWithValue("serialNumber", "0"))
				Expect(payload.Content).ToNot(HaveKey("appExtVrsId"))
			}).
			Return(http.Result[downloadResult]{
				StatusCode: gohttp.StatusOK,
				Data:       downloadResult{Items: []downloadItemResult{signedItem()}},
			}, nil)

		item, err := as.fetchDownloadItem(account, app, testGUID, testVersionID, PlatformIPhone)
		Expect(err).ToNot(HaveOccurred())
		Expect(item.URL).To(Equal("https://cdn.example.com/karing.ipa"))
	})

	It("falls back to the redownload endpoint for an empty volumeStore response", func() {
		gomock.InOrder(
			emptyVolumeStoreExpectations(),
			mockBagClient.EXPECT().Send(gomock.Any()).Return(emptyBag(), nil),
			mockDownloadClient.EXPECT().Send(gomock.Any()).
				Do(func(req http.Request) {
					Expect(req.URL).To(Equal(wellKnownRedownloadURL + "?guid=" + testGUID))

					payload, ok := req.Payload.(*http.XMLPayload)
					Expect(ok).To(BeTrue())
					Expect(payload.Content).To(HaveKeyWithValue("serialNumber", "0"))
					Expect(payload.Content).ToNot(HaveKey("appExtVrsId"))
					Expect(payload.Content).ToNot(HaveKey("externalVersionId"))
				}).
				Return(http.Result[downloadResult]{
					StatusCode: gohttp.StatusOK,
					Data:       downloadResult{Items: []downloadItemResult{signedItem()}},
				}, nil),
		)

		item, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
		Expect(err).ToNot(HaveOccurred())
		Expect(item.URL).To(Equal("https://cdn.example.com/karing.ipa"))
	})

	It("uses the redownload endpoint published in the bag", func() {
		gomock.InOrder(
			emptyVolumeStoreExpectations(),
			mockBagClient.EXPECT().Send(gomock.Any()).
				Do(func(req http.Request) {
					Expect(req.URL).To(Equal("https://init.itunes.apple.com/bag.xml?guid=" + testGUID))
				}).
				Return(bagWithEndpoint(), nil),
			mockDownloadClient.EXPECT().Send(gomock.Any()).
				Do(func(req http.Request) {
					Expect(req.URL).To(Equal(wellKnownRedownloadURL + "?guid=" + testGUID))
				}).
				Return(http.Result[downloadResult]{
					StatusCode: gohttp.StatusOK,
					Data:       downloadResult{Items: []downloadItemResult{signedItem()}},
				}, nil),
		)

		_, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
		Expect(err).ToNot(HaveOccurred())
	})

	It("keeps the fallback available when the bag request fails", func() {
		gomock.InOrder(
			emptyVolumeStoreExpectations(),
			mockBagClient.EXPECT().Send(gomock.Any()).
				Return(http.Result[bagResult]{}, errors.New("bag unavailable")),
			mockDownloadClient.EXPECT().Send(gomock.Any()).
				Do(func(req http.Request) {
					Expect(req.URL).To(Equal(wellKnownRedownloadURL + "?guid=" + testGUID))
				}).
				Return(http.Result[downloadResult]{
					StatusCode: gohttp.StatusOK,
					Data:       downloadResult{Items: []downloadItemResult{signedItem()}},
				}, nil),
		)

		_, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
		Expect(err).ToNot(HaveOccurred())
	})

	It("rejects an unsupported redownload endpoint published in the bag", func() {
		gomock.InOrder(
			emptyVolumeStoreExpectations(),
			mockBagClient.EXPECT().Send(gomock.Any()).
				Return(http.Result[bagResult]{
					StatusCode: gohttp.StatusOK,
					Data: bagResult{URLBag: urlBag{RedownloadEndpoint: "https://example.com/r/redownload"}},
				}, nil),
		)

		_, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported redownload endpoint"))
	})

	It("does not fall back for an App Store error response", func() {
		mockMachine.EXPECT().MacAddress().Return("00:11:22:33:44:55", nil)

		mockDownloadClient.EXPECT().
			Send(gomock.Any()).
			Return(http.Result[downloadResult]{
				StatusCode: gohttp.StatusOK,
				Data: downloadResult{
					FailureType:     FailureTypeLicenseNotFound,
					CustomerMessage: "License not found",
				},
			}, nil)

		_, err := as.ListVersions(ListVersionsInput{Account: account, App: app})
		Expect(errors.Is(err, ErrLicenseRequired)).To(BeTrue())
	})

	Describe("empty redownload HTTP 500 (issue #547)", func() {
		It("retries once with the latest catalog version", func() {
			redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}

			gomock.InOrder(
				emptyVolumeStoreExpectations(),
				mockBagClient.EXPECT().Send(gomock.Any()).Return(bagWithEndpoint(), nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{}, redownloadErr),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Do(func(req http.Request) {
						u, err := url.Parse(req.URL)
						Expect(err).ToNot(HaveOccurred())
						Expect(u.Host).To(Equal("uclient-api.itunes.apple.com"))
						Expect(u.Query().Get("id")).To(Equal(strconv.FormatInt(app.ID, 10)))
						Expect(u.Query().Get("cc")).To(Equal("us"))
						Expect(u.Query().Get("platform")).To(Equal("enterprisestore"))
					}).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK, Data: lookupWith(testVersionID)}, nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Do(func(req http.Request) {
						Expect(req.URL).To(Equal(wellKnownRedownloadURL + "?guid=" + testGUID))
						Expect(req.Method).To(Equal(http.MethodPOST))
						payload := req.Payload.(*http.XMLPayload).Content
						Expect(payload).To(HaveKeyWithValue("salableAdamId", app.ID))
						Expect(payload).To(HaveKeyWithValue("guid", testGUID))
						Expect(payload).To(HaveKeyWithValue("appExtVrsId", testVersionID))
						Expect(payload).ToNot(HaveKey("externalVersionId"))
					}).
					Return(http.Result[downloadResult]{
						StatusCode: gohttp.StatusOK,
						Data:       downloadResult{Items: []downloadItemResult{signedItem()}},
					}, nil),
			)

			item, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
			Expect(err).ToNot(HaveOccurred())
			Expect(item.URL).To(Equal("https://cdn.example.com/karing.ipa"))
		})

		It("applies the same retry to the default platform", func() {
			redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}

			gomock.InOrder(
				emptyVolumeStoreExpectations(),
				mockBagClient.EXPECT().Send(gomock.Any()).Return(emptyBag(), nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{}, redownloadErr),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK, Data: lookupWith(testVersionID)}, nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{
						StatusCode: gohttp.StatusOK,
						Data:       downloadResult{Items: []downloadItemResult{signedItem()}},
					}, nil),
			)

			_, err := as.fetchDownloadItem(account, app, testGUID, "", Platform(""))
			Expect(err).ToNot(HaveOccurred())
		})

		DescribeTable("does not retry unrelated redownload errors",
			func(original error) {
				gomock.InOrder(
					emptyVolumeStoreExpectations(),
					mockBagClient.EXPECT().Send(gomock.Any()).Return(emptyBag(), nil),
					mockDownloadClient.EXPECT().Send(gomock.Any()).
						Return(http.Result[downloadResult]{}, original),
				)

				_, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
				Expect(errors.Is(err, original)).To(BeTrue())
			},
			Entry("authentication", &http.UnexpectedResponseError{StatusCode: gohttp.StatusForbidden}),
			Entry("rate limit", &http.UnexpectedResponseError{StatusCode: gohttp.StatusTooManyRequests}),
			Entry("service unavailable", &http.UnexpectedResponseError{StatusCode: gohttp.StatusServiceUnavailable}),
			Entry("nonempty 500 message", &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError, Snippet: "maintenance"}),
			Entry("network failure", errors.New("connection reset")),
		)

		DescribeTable("does not replace an explicit version or cross platforms",
			func(versionID string, platform Platform) {
				redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}

				gomock.InOrder(
					emptyVolumeStoreExpectations(),
					mockBagClient.EXPECT().Send(gomock.Any()).Return(emptyBag(), nil),
					mockDownloadClient.EXPECT().Send(gomock.Any()).
						Return(http.Result[downloadResult]{}, redownloadErr),
				)

				_, err := as.fetchDownloadItem(account, app, testGUID, versionID, platform)
				Expect(errors.Is(err, redownloadErr)).To(BeTrue())
				Expect(err.Error()).To(ContainSubstring("failed to send redownload request"))
			},
			Entry("explicit version", testVersionID, PlatformIPhone),
			Entry("macOS", "", PlatformMacOS),
			Entry("tvOS", "", PlatformAppleTV),
			Entry("visionOS", "", PlatformVisionOS),
		)

		It("preserves an actionable failure from the pinned retry", func() {
			redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}

			gomock.InOrder(
				emptyVolumeStoreExpectations(),
				mockBagClient.EXPECT().Send(gomock.Any()).Return(emptyBag(), nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{}, redownloadErr),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK, Data: lookupWith(testVersionID)}, nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{
						StatusCode: gohttp.StatusOK,
						Data:       downloadResult{FailureType: FailureTypeLicenseNotFound},
					}, nil),
			)

			_, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
			Expect(errors.Is(err, ErrLicenseRequired)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("both download endpoints failed"))
		})

		It("does not loop when the pinned retry also returns an empty 500", func() {
			redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}
			pinnedErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}

			gomock.InOrder(
				emptyVolumeStoreExpectations(),
				mockBagClient.EXPECT().Send(gomock.Any()).Return(emptyBag(), nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{}, redownloadErr),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK, Data: lookupWith(testVersionID)}, nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{}, pinnedErr),
			)

			_, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
			Expect(errors.Is(err, pinnedErr)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("failed to send version-pinned redownload request"))
		})

		It("preserves the original error when the catalog lookup fails", func() {
			redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}
			lookupErr := errors.New("catalog unavailable")

			gomock.InOrder(
				emptyVolumeStoreExpectations(),
				mockBagClient.EXPECT().Send(gomock.Any()).Return(emptyBag(), nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{}, redownloadErr),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[platformVersionLookupResult]{}, lookupErr),
			)

			_, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
			Expect(errors.Is(err, redownloadErr)).To(BeTrue())
			Expect(errors.Is(err, lookupErr)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("failed to resolve latest version for redownload"))
		})

		It("does not retry without a catalog version", func() {
			redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}

			gomock.InOrder(
				emptyVolumeStoreExpectations(),
				mockBagClient.EXPECT().Send(gomock.Any()).Return(emptyBag(), nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{}, redownloadErr),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK}, nil),
			)

			_, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
			Expect(errors.Is(err, redownloadErr)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("failed to resolve latest version for redownload"))
			Expect(err.Error()).To(ContainSubstring("platform version lookup returned no app"))
		})
	})

	When("ListVersions hits the empty redownload 500 (issue #547)", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().MacAddress().Return("00:11:22:33:44:55", nil)
		})

		It("resolves the latest catalog version in the account storefront and retries with it", func() {
			redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}

			gomock.InOrder(
				emptyVolumeStoreExpectations(),
				mockBagClient.EXPECT().Send(gomock.Any()).Return(bagWithEndpoint(), nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Do(func(req http.Request) {
						Expect(req.URL).To(Equal(wellKnownRedownloadURL + "?guid=" + testGUID))
					}).
					Return(http.Result[downloadResult]{}, redownloadErr),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Do(func(req http.Request) {
						u, err := url.Parse(req.URL)
						Expect(err).ToNot(HaveOccurred())
						Expect(u.Host).To(Equal("uclient-api.itunes.apple.com"))
						Expect(u.Query().Get("id")).To(Equal(strconv.FormatInt(app.ID, 10)))
						Expect(u.Query().Get("cc")).To(Equal("us"))
						Expect(u.Query().Get("platform")).To(Equal("enterprisestore"))
					}).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK, Data: lookupWith(testVersionID)}, nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Do(func(req http.Request) {
						Expect(req.URL).To(Equal(wellKnownRedownloadURL + "?guid=" + testGUID))
						payload := req.Payload.(*http.XMLPayload).Content
						Expect(payload).To(HaveKeyWithValue("appExtVrsId", testVersionID))
						Expect(payload).ToNot(HaveKey("externalVersionId"))
					}).
					Return(http.Result[downloadResult]{
						StatusCode: gohttp.StatusOK,
						Data:       downloadResult{Items: []downloadItemResult{listItem()}},
					}, nil),
			)

			out, err := as.ListVersions(ListVersionsInput{Account: account, App: app})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.ExternalVersionIdentifiers).To(Equal([]string{"890000001", testVersionID}))
			Expect(out.LatestExternalVersionID).To(Equal(testVersionID))
		})

		It("reports both errors when the catalog lookup finds no app", func() {
			redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}

			gomock.InOrder(
				emptyVolumeStoreExpectations(),
				mockBagClient.EXPECT().Send(gomock.Any()).Return(bagWithEndpoint(), nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{}, redownloadErr),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK}, nil),
			)

			_, err := as.ListVersions(ListVersionsInput{Account: account, App: app})
			Expect(errors.Is(err, redownloadErr)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("failed to resolve latest version for redownload"))
			Expect(err.Error()).To(ContainSubstring("platform version lookup returned no app"))
		})
	})

	When("the app is missing from the account storefront catalog (RU account, US app)", func() {
		BeforeEach(func() {
			account.StoreFront = "143469-1,34" // RU storefront
		})

		It("falls back to the US catalog to resolve the version", func() {
			redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}

			gomock.InOrder(
				emptyVolumeStoreExpectations(),
				mockBagClient.EXPECT().Send(gomock.Any()).Return(bagWithEndpoint(), nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{}, redownloadErr),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Do(func(req http.Request) {
						u, err := url.Parse(req.URL)
						Expect(err).ToNot(HaveOccurred())
						Expect(u.Query().Get("cc")).To(Equal("ru"))
					}).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK}, nil),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Do(func(req http.Request) {
						u, err := url.Parse(req.URL)
						Expect(err).ToNot(HaveOccurred())
						Expect(u.Query().Get("cc")).To(Equal("us"))
					}).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK, Data: lookupWith(testVersionID)}, nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{
						StatusCode: gohttp.StatusOK,
						Data:       downloadResult{Items: []downloadItemResult{signedItem()}},
					}, nil),
			)

			_, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
			Expect(err).ToNot(HaveOccurred())
		})

		It("reports the last attempted catalog when the app is nowhere", func() {
			redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}

			gomock.InOrder(
				emptyVolumeStoreExpectations(),
				mockBagClient.EXPECT().Send(gomock.Any()).Return(bagWithEndpoint(), nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{}, redownloadErr),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK}, nil),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK}, nil),
			)

			_, err := as.fetchDownloadItem(account, app, testGUID, "", PlatformIPhone)
			Expect(errors.Is(err, redownloadErr)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("platform version lookup returned no app"))
			Expect(err.Error()).To(ContainSubstring("cc us"))
		})
	})

	When("CheckDownload hits the empty redownload 500 (issue #547)", func() {
		BeforeEach(func() {
			mockMachine.EXPECT().MacAddress().Return("00:11:22:33:44:55", nil)
		})

		It("resolves the latest catalog version and retries with it", func() {
			redownloadErr := &http.UnexpectedResponseError{StatusCode: gohttp.StatusInternalServerError}

			gomock.InOrder(
				emptyVolumeStoreExpectations(),
				mockBagClient.EXPECT().Send(gomock.Any()).Return(bagWithEndpoint(), nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{}, redownloadErr),
				mockPlatformClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[platformVersionLookupResult]{StatusCode: gohttp.StatusOK, Data: lookupWith(testVersionID)}, nil),
				mockDownloadClient.EXPECT().Send(gomock.Any()).
					Return(http.Result[downloadResult]{
						StatusCode: gohttp.StatusOK,
						Data:       downloadResult{Items: []downloadItemResult{signedItem()}},
					}, nil),
			)

			out, err := as.CheckDownload(CheckDownloadInput{Account: account, App: app})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.Version).To(Equal("1.2.24"))
			Expect(out.LatestExternalVersionID).To(Equal(testVersionID))
		})
	})
})
