package appstore

import (
	"encoding/binary"
	"errors"
	gohttp "net/http"
	"time"

	"github.com/majd/ipatool/v2/pkg/http"
	"github.com/majd/ipatool/v2/pkg/util/machine"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("AppStore (OwnedApps)", func() {
	const (
		testDSID       = "123456789"
		testGUID       = "AABBCCDDEEFF"
		testStoreFront = "143441"
		testToken      = "password-token"
		testSessionID  = uint32(42)
		testRevision   = uint32(99)
	)

	var (
		ctrl            *gomock.Controller
		mockOwnedClient *http.MockClient[[]byte]
		mockMachine     *machine.MockMachine
		as              *appstore
	)

	account := Account{
		DirectoryServicesID: testDSID,
		PasswordToken:       testToken,
		StoreFront:          testStoreFront,
	}

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockOwnedClient = http.NewMockClient[[]byte](ctrl)
		mockMachine = machine.NewMockMachine(ctrl)
		as = &appstore{
			ownedAppsClient: mockOwnedClient,
			machine:         mockMachine,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	expectMac := func() {
		mockMachine.EXPECT().
			MacAddress().
			Return("aa:bb:cc:dd:ee:ff", nil)
	}

	expectLogin := func() *gomock.Call {
		return mockOwnedClient.EXPECT().
			Send(gomock.Any()).
			Do(func(req http.Request) {
				Expect(req.Method).To(Equal(http.MethodPOST))
				Expect(req.URL).To(Equal(PrivatePurchaseDAAPBaseURL + "/login"))
				Expect(req.SignAction).To(BeFalse())
				Expect(req.Payload).To(BeNil())
				Expect(req.ResponseFormat).To(Equal(http.ResponseFormatRaw))
				expectOwnedAppsHeaders(req.Headers, account, testGUID)
			}).
			Return(http.Result[[]byte]{
				StatusCode: gohttp.StatusOK,
				Data:       dmapTag("mlog", dmapUint32("mlid", testSessionID)),
			}, nil)
	}

	expectUpdate := func() *gomock.Call {
		return mockOwnedClient.EXPECT().
			Send(gomock.Any()).
			Do(func(req http.Request) {
				Expect(req.Method).To(Equal(http.MethodPOST))
				Expect(req.URL).To(Equal(PrivatePurchaseDAAPBaseURL + "/update"))
				Expect(req.SignAction).To(BeTrue())
				Expect(req.Headers).To(HaveKeyWithValue("Content-Type", "application/x-www-form-urlencoded"))
				expectOwnedAppsHeaders(req.Headers, account, testGUID)

				payload, ok := req.Payload.(*http.RawPayload)
				Expect(ok).To(BeTrue())
				Expect(string(payload.Content)).To(Equal("session-id=42&revision-number=(null)&query=('com.apple.itunes.extended\\-media\\-kind:131072')"))
			}).
			Return(http.Result[[]byte]{
				StatusCode: gohttp.StatusOK,
				Data:       dmapTag("mupd", dmapUint32("musr", testRevision)),
			}, nil)
	}

	expectItems := func(apps []App) *gomock.Call {
		return mockOwnedClient.EXPECT().
			Send(gomock.Any()).
			Do(func(req http.Request) {
				Expect(req.Method).To(Equal(http.MethodPOST))
				Expect(req.URL).To(Equal(PrivatePurchaseDAAPBaseURL + "/databases/99/items"))
				Expect(req.SignAction).To(BeTrue())
				Expect(req.Headers).To(HaveKeyWithValue("Content-Type", "application/x-dmap-tagged"))
				expectOwnedAppsHeaders(req.Headers, account, testGUID)

				payload, ok := req.Payload.(*http.RawPayload)
				Expect(ok).To(BeTrue())
				sessionID, found, err := firstDMAPUint(payload.Content, "mlid")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(sessionID).To(Equal(uint64(testSessionID)))
				revision, found, err := firstDMAPUint(payload.Content, "musr")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(revision).To(Equal(uint64(testRevision)))
			}).
			Return(http.Result[[]byte]{
				StatusCode: gohttp.StatusOK,
				Data:       ownedAppsDMAPResponse(apps),
			}, nil)
	}

	When("the account has more than one page of apps", func() {
		It("sorts all apps by descending purchase date before returning the requested page", func() {
			expectMac()

			responseOrder := []int{5, 12, 1, 9, 3, 11, 2, 8, 4, 10, 6, 7}
			apps := make([]App, 0, 12)
			for _, index := range responseOrder {
				apps = append(apps, App{
					ID:           int64(1000 + index),
					BundleID:     "com.example.app" + string(rune('a'+index-1)),
					Name:         "DAAP app",
					Version:      "1.0",
					PurchaseDate: time.Unix(1_700_000_000+int64(index*60), 0).UTC(),
				})
			}

			loginCall := expectLogin()
			updateCall := expectUpdate()
			itemsCall := expectItems(apps)
			gomock.InOrder(loginCall, updateCall, itemsCall)

			out, err := as.OwnedApps(OwnedAppsInput{Account: account, Page: 2, Limit: 10})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.TotalCount).To(Equal(12))
			Expect(out.Page).To(Equal(2))
			Expect(out.Count).To(Equal(2))
			Expect(out.Results).To(HaveLen(2))
			Expect(out.Results[0].ID).To(Equal(int64(1002)))
			Expect(out.Results[1].ID).To(Equal(int64(1001)))
		})

		It("returns every app when All is set", func() {
			expectMac()

			apps := []App{
				{ID: 1, BundleID: "a", Name: "A", PurchaseDate: time.Unix(100, 0).UTC()},
				{ID: 2, BundleID: "b", Name: "B", PurchaseDate: time.Unix(300, 0).UTC()},
				{ID: 3, BundleID: "c", Name: "C"},
				{ID: 4, BundleID: "d", Name: "D", PurchaseDate: time.Unix(200, 0).UTC()},
			}

			gomock.InOrder(expectLogin(), expectUpdate(), expectItems(apps))

			out, err := as.OwnedApps(OwnedAppsInput{Account: account, All: true})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.TotalCount).To(Equal(4))
			Expect(out.Count).To(Equal(4))
			ids := []int64{out.Results[0].ID, out.Results[1].ID, out.Results[2].ID, out.Results[3].ID}
			// newest first; apps without a purchase date go last
			Expect(ids).To(Equal([]int64{2, 4, 1, 3}))
		})
	})

	When("the response repeats an app", func() {
		It("deduplicates by app ID", func() {
			expectMac()

			apps := []App{
				{ID: 7, BundleID: "x", Name: "X", PurchaseDate: time.Unix(100, 0).UTC()},
				{ID: 7, BundleID: "x", Name: "X", PurchaseDate: time.Unix(100, 0).UTC()},
			}

			gomock.InOrder(expectLogin(), expectUpdate(), expectItems(apps))

			out, err := as.OwnedApps(OwnedAppsInput{Account: account, All: true})
			Expect(err).ToNot(HaveOccurred())
			Expect(out.TotalCount).To(Equal(1))
		})
	})

	When("the DAAP login answers with an unauthorized status", func() {
		It("returns ErrPasswordTokenExpired", func() {
			expectMac()

			mockOwnedClient.EXPECT().
				Send(gomock.Any()).
				Return(http.Result[[]byte]{
					StatusCode: gohttp.StatusUnauthorized,
				}, nil)

			_, err := as.OwnedApps(OwnedAppsInput{Account: account})
			Expect(err).To(MatchError(ErrPasswordTokenExpired))
		})
	})

	When("the DAAP body carries a 403 status tag", func() {
		It("returns ErrPasswordTokenExpired", func() {
			expectMac()

			mockOwnedClient.EXPECT().
				Send(gomock.Any()).
				Return(http.Result[[]byte]{
					StatusCode: gohttp.StatusOK,
					Data:       dmapTag("mlog", dmapUint32("mstt", 403)),
				}, nil)

			_, err := as.OwnedApps(OwnedAppsInput{Account: account})
			Expect(err).To(MatchError(ErrPasswordTokenExpired))
		})
	})

	When("the login request fails", func() {
		It("returns error", func() {
			expectMac()

			mockOwnedClient.EXPECT().
				Send(gomock.Any()).
				Return(http.Result[[]byte]{}, errors.New("boom"))

			_, err := as.OwnedApps(OwnedAppsInput{Account: account})
			Expect(err).To(HaveOccurred())
		})
	})

	When("the MAC address cannot be resolved", func() {
		It("returns error", func() {
			mockMachine.EXPECT().
				MacAddress().
				Return("", errors.New(""))

			_, err := as.OwnedApps(OwnedAppsInput{Account: account})
			Expect(err).To(HaveOccurred())
		})
	})

	When("input is invalid", func() {
		It("rejects a negative page", func() {
			_, err := as.OwnedApps(OwnedAppsInput{Account: account, Page: -1})
			Expect(err).To(HaveOccurred())
		})

		It("rejects a limit above the maximum", func() {
			_, err := as.OwnedApps(OwnedAppsInput{Account: account, Limit: MaxOwnedAppsLimit + 1})
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("walkDMAP", func() {
		It("rejects truncated payloads", func() {
			data := dmapTag("mlog", dmapUint32("mlid", 1))
			data[7] = 0xff // claim a longer payload than present

			err := walkDMAP(data, 0, func(string, []byte) error { return nil })
			Expect(err).To(HaveOccurred())
		})

		It("rejects non-printable tags", func() {
			data := dmapUint32("ml\x01d", 1)

			err := walkDMAP(data, 0, func(string, []byte) error { return nil })
			Expect(err).To(HaveOccurred())
		})
	})
})

func expectOwnedAppsHeaders(headers map[string]string, account Account, guid string) {
	Expect(headers).To(HaveKeyWithValue("X-Dsid", account.DirectoryServicesID))
	Expect(headers).To(HaveKeyWithValue("iCloud-DSID", account.DirectoryServicesID))
	Expect(headers).To(HaveKeyWithValue("X-Token", account.PasswordToken))
	Expect(headers).To(HaveKeyWithValue("X-Apple-Store-Front", account.StoreFront))
	Expect(headers).To(HaveKeyWithValue("X-Guid", guid))
	Expect(headers).To(HaveKeyWithValue("Client-DAAP-Version", "3.12"))
	Expect(headers).To(HaveKeyWithValue("Client-Cloud-Purchase-Daap-Version", "1.1/Configurator-2.0"))
}

func ownedAppsDMAPResponse(apps []App) []byte {
	listing := make([]byte, 0, len(apps)*128)

	for _, app := range apps {
		item := make([]byte, 0, 128)
		item = append(item, dmapUint64ForTest("aeSI", uint64(app.ID))...)
		item = append(item, dmapString("aeBI", app.BundleID)...)
		item = append(item, dmapString("aeLN", app.Name)...)
		item = append(item, dmapString("aePd", app.Version)...)

		if !app.PurchaseDate.IsZero() {
			item = append(item, dmapUint32("asdp", uint32(app.PurchaseDate.Unix()))...)
		}

		listing = append(listing, dmapTag("mlit", item)...)
	}

	return dmapTag("adbs", dmapTag("mlcl", listing))
}

func dmapUint64ForTest(name string, value uint64) []byte {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, value)

	return dmapTag(name, payload)
}
