package appstore

import (
	gohttp "net/http"

	"github.com/majd/ipatool/v2/pkg/http"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("AppStore (Update Product fallback)", func() {
	const (
		testGUID      = "001122334455"
		testVersionID = "818970197"
	)

	var (
		ctrl          *gomock.Controller
		mockDownload  *http.MockClient[downloadResult]
		mockBagClient *http.MockClient[bagResult]
		store         *appstore
		acc           Account
		app           App
	)

	validUpdateItem := downloadItemResult{
		URL:   "https://cdn/pinned.ipa",
		Sinfs: []Sinf{{ID: 0, Data: []byte("sinf")}},
		Metadata: map[string]interface{}{
			"itemId":                            int64(568903335),
			"softwareVersionExternalIdentifier": testVersionID,
			"softwareVersionBundleId":           "com.example.app",
		},
	}

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockDownload = http.NewMockClient[downloadResult](ctrl)
		mockBagClient = http.NewMockClient[bagResult](ctrl)
		store = &appstore{
			downloadClient: mockDownload,
			bagClient:      mockBagClient,
		}
		acc = Account{StoreFront: "143441", DirectoryServicesID: "1234"}
		app = App{ID: 568903335, BundleID: "com.example.app"}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	expectEmptyPrimary := func(previous *gomock.Call) *gomock.Call {
		call := mockDownload.EXPECT().Send(gomock.Any()).Return(http.Result[downloadResult]{
			StatusCode: gohttp.StatusOK, Data: downloadResult{},
		}, nil)

		if previous != nil {
			call = call.After(previous)
		}

		return call
	}

	expectBag := func(previous *gomock.Call, updateEndpoint string) *gomock.Call {
		call := mockBagClient.EXPECT().Send(gomock.Any()).Return(http.Result[bagResult]{
			StatusCode: gohttp.StatusOK,
			Data:       bagResult{URLBag: urlBag{UpdateEndpoint: updateEndpoint}},
		}, nil)

		return call.After(previous)
	}

	expectUpdateRequest := func(previous *gomock.Call, item downloadItemResult, itemErr error) *gomock.Call {
		return mockDownload.EXPECT().Send(gomock.Any()).Do(func(req http.Request) {
			Expect(req.URL).To(Equal("https://downloaddispatch.itunes.apple.com/up/updateProduct?guid=" + testGUID))

			payload, ok := req.Payload.(*http.XMLPayload)
			Expect(ok).To(BeTrue())
			Expect(payload.Content).To(HaveKeyWithValue("salableAdamId", int64(568903335)))
			Expect(payload.Content).To(HaveKeyWithValue("appExtVrsId", testVersionID))
		}).Return(http.Result[downloadResult]{
			StatusCode: gohttp.StatusOK, Data: downloadResult{Items: []downloadItemResult{item}},
		}, itemErr).After(previous)
	}

	// The fallback only kicks in after the primary endpoint came back empty
	// twice (first try + retry).
	expectEmptyPrimaries := func() *gomock.Call {
		first := expectEmptyPrimary(nil)

		return expectEmptyPrimary(first)
	}

	It("serves a pinned tvOS version via updateProduct when redownload returns an empty HTTP 500", func() {
		previous := expectEmptyPrimaries()
		previous = mockDownload.EXPECT().Send(gomock.Any()).Return(http.Result[downloadResult]{}, &http.UnexpectedResponseError{
			StatusCode: gohttp.StatusInternalServerError,
		}).After(previous)
		previous = expectBag(previous, "https://downloaddispatch.itunes.apple.com/up/updateProduct")
		expectUpdateRequest(previous, validUpdateItem, nil)

		item, err := store.fetchDownloadItem(acc, app, testGUID, testVersionID, PlatformAppleTV)
		Expect(err).ToNot(HaveOccurred())
		Expect(item.URL).To(Equal(validUpdateItem.URL))
	})

	It("serves a pinned version when redownload reports it as no longer available", func() {
		previous := expectEmptyPrimaries()
		previous = mockDownload.EXPECT().Send(gomock.Any()).Return(http.Result[downloadResult]{
			StatusCode:    gohttp.StatusOK,
			Data:          downloadResult{CustomerMessage: "No Longer Available"},
		}, nil).After(previous)
		previous = expectBag(previous, "https://downloaddispatch.itunes.apple.com/up/updateProduct")
		expectUpdateRequest(previous, validUpdateItem, nil)

		item, err := store.fetchDownloadItem(acc, app, testGUID, testVersionID, PlatformAppleTV)
		Expect(err).ToNot(HaveOccurred())
		Expect(item.URL).To(Equal(validUpdateItem.URL))
	})

	It("preserves an unpinned tvOS availability response without updating", func() {
		previous := expectEmptyPrimaries()
		mockDownload.EXPECT().Send(gomock.Any()).Return(http.Result[downloadResult]{
			StatusCode:    gohttp.StatusOK,
			Data:          downloadResult{CustomerMessage: "No Longer Available"},
		}, nil).After(previous)

		_, err := store.fetchDownloadItem(acc, app, testGUID, "", PlatformAppleTV)
		Expect(err).To(MatchError(ContainSubstring("No Longer Available")))
	})

	It("rejects an update response that does not match the requested version", func() {
		mismatched := validUpdateItem
		mismatched.Metadata = map[string]interface{}{
			"itemId":                            int64(568903335),
			"softwareVersionExternalIdentifier": "999999999",
			"softwareVersionBundleId":           "com.example.app",
		}

		previous := expectEmptyPrimaries()
		previous = mockDownload.EXPECT().Send(gomock.Any()).Return(http.Result[downloadResult]{}, &http.UnexpectedResponseError{
			StatusCode: gohttp.StatusInternalServerError,
		}).After(previous)
		previous = expectBag(previous, "https://downloaddispatch.itunes.apple.com/up/updateProduct")
		expectUpdateRequest(previous, mismatched, nil)

		_, err := store.fetchDownloadItem(acc, app, testGUID, testVersionID, PlatformAppleTV)
		Expect(err).To(MatchError("update response does not match the requested app or version"))
	})

	It("rejects an update response with a wrong bundle identifier", func() {
		mismatched := validUpdateItem
		mismatched.Metadata = map[string]interface{}{
			"itemId":                            int64(568903335),
			"softwareVersionExternalIdentifier": testVersionID,
			"softwareVersionBundleId":           "com.example.other",
		}

		previous := expectEmptyPrimaries()
		previous = mockDownload.EXPECT().Send(gomock.Any()).Return(http.Result[downloadResult]{}, &http.UnexpectedResponseError{
			StatusCode: gohttp.StatusInternalServerError,
		}).After(previous)
		previous = expectBag(previous, "https://downloaddispatch.itunes.apple.com/up/updateProduct")
		expectUpdateRequest(previous, mismatched, nil)

		_, err := store.fetchDownloadItem(acc, app, testGUID, testVersionID, PlatformAppleTV)
		Expect(err).To(MatchError("update response does not match the requested bundle identifier"))
	})

	It("rejects an update endpoint outside the download dispatch domain", func() {
		previous := expectEmptyPrimaries()
		previous = mockDownload.EXPECT().Send(gomock.Any()).Return(http.Result[downloadResult]{}, &http.UnexpectedResponseError{
			StatusCode: gohttp.StatusInternalServerError,
		}).After(previous)
		previous = expectBag(previous, "https://evil.example.com/up/updateProduct")

		_, err := store.fetchDownloadItem(acc, app, testGUID, testVersionID, PlatformAppleTV)
		Expect(err).To(MatchError("invalid download endpoint in bag"))
	})

	It("propagates license errors from the update endpoint", func() {
		previous := expectEmptyPrimaries()
		previous = mockDownload.EXPECT().Send(gomock.Any()).Return(http.Result[downloadResult]{}, &http.UnexpectedResponseError{
			StatusCode: gohttp.StatusInternalServerError,
		}).After(previous)
		previous = expectBag(previous, "https://downloaddispatch.itunes.apple.com/up/updateProduct")
		mockDownload.EXPECT().Send(gomock.Any()).Return(http.Result[downloadResult]{
			StatusCode: gohttp.StatusOK,
			Data:       downloadResult{FailureType: FailureTypeLicenseNotFound},
		}, nil).After(previous)

		_, err := store.fetchDownloadItem(acc, app, testGUID, testVersionID, PlatformAppleTV)
		Expect(err).To(MatchError(ErrLicenseRequired))
	})

	It("keeps the redownload error when the bag has no update endpoint", func() {
		previous := expectEmptyPrimaries()
		previous = mockDownload.EXPECT().Send(gomock.Any()).Return(http.Result[downloadResult]{}, &http.UnexpectedResponseError{
			StatusCode: gohttp.StatusInternalServerError,
		}).After(previous)
		mockBagClient.EXPECT().Send(gomock.Any()).Return(http.Result[bagResult]{
			StatusCode: gohttp.StatusOK, Data: bagResult{URLBag: urlBag{}},
		}, nil).After(previous)

		_, err := store.fetchDownloadItem(acc, app, testGUID, testVersionID, PlatformAppleTV)
		Expect(err).To(MatchError(ContainSubstring("failed to send redownload request")))
	})
})
