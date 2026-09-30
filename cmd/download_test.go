package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/majd/ipatool/v2/pkg/log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Download command", func() {
	Describe("app resolution", func() {
		var store *fakeDownloadAppStore

		BeforeEach(func() {
			store = &fakeDownloadAppStore{account: appstore.Account{StoreFront: "143441"}}
			previousDependencies := dependencies
			DeferCleanup(func() { dependencies = previousDependencies })
			dependencies.AppStore = store
			dependencies.Logger = log.NewLogger(log.Args{})
		})

		execute := func(args ...string) error {
			cmd := downloadCmd()
			cmd.SetArgs(args)
			cmd.SetContext(context.WithValue(context.Background(), interactiveKey, false))

			return cmd.Execute()
		}

		It("uses the app ID when the bundle is absent from the catalog", func() {
			store.lookupError = fmt.Errorf("lookup: %w", appstore.ErrAppNotFound)
			Expect(execute("-i", "42", "-b", "com.example.delisted", "--platform", "appletv")).To(Succeed())
			Expect(store.lookupInputs).To(Equal([]appstore.LookupInput{{
				Account:  appstore.Account{StoreFront: "143441"},
				BundleID: "com.example.delisted",
				Platform: appstore.PlatformAppleTV,
			}}))
			Expect(store.downloadInputs).To(HaveLen(1))
			Expect(store.downloadInputs[0].App).To(Equal(appstore.App{ID: 42, BundleID: "com.example.delisted"}))
			Expect(store.downloadInputs[0].Platform).To(Equal(appstore.PlatformAppleTV))
		})

		It("preserves bundle identifier precedence when lookup succeeds", func() {
			store.lookupOutput.App = appstore.App{ID: 43, BundleID: "com.example.listed"}
			Expect(execute("-i", "42", "-b", "com.example.listed")).To(Succeed())
			Expect(store.downloadInputs).To(HaveLen(1))
			Expect(store.downloadInputs[0].App).To(Equal(store.lookupOutput.App))
		})

		It("resolves a bundle identifier without an app ID", func() {
			store.lookupOutput.App = appstore.App{ID: 43, BundleID: "com.example.listed"}
			Expect(execute("-b", "com.example.listed")).To(Succeed())
			Expect(store.downloadInputs).To(HaveLen(1))
			Expect(store.downloadInputs[0].App).To(Equal(store.lookupOutput.App))
		})

		It("does not look up the bundle for an explicit app ID and version", func() {
			Expect(execute("-i", "42", "--platform", "appletv", "--external-version-id", "123456")).To(Succeed())
			Expect(store.lookupInputs).To(BeEmpty())
			Expect(store.downloadInputs).To(HaveLen(1))
			Expect(store.downloadInputs[0].App).To(Equal(appstore.App{ID: 42}))
			Expect(store.downloadInputs[0].ExternalVersionID).To(Equal("123456"))
		})

		DescribeTable("preserves lookup errors", func(args []string, lookupError error) {
			store.lookupError = lookupError
			Expect(execute(args...)).To(MatchError(lookupError))
			Expect(store.downloadInputs).To(BeEmpty())
		},
			Entry("missing bundle without an app ID", []string{"-b", "com.example.delisted"}, appstore.ErrAppNotFound),
			Entry("request failure with an app ID", []string{"-i", "42", "-b", "com.example.app"}, errors.New("request failed")),
		)
	})
})

type fakeDownloadAppStore struct {
	appstore.AppStore
	account        appstore.Account
	downloadInputs []appstore.DownloadInput
	lookupInputs   []appstore.LookupInput
	lookupOutput   appstore.LookupOutput
	lookupError    error
}

func (s *fakeDownloadAppStore) AccountInfo() (appstore.AccountInfoOutput, error) {
	return appstore.AccountInfoOutput{Account: s.account}, nil
}
func (s *fakeDownloadAppStore) Lookup(input appstore.LookupInput) (appstore.LookupOutput, error) {
	s.lookupInputs = append(s.lookupInputs, input)

	return s.lookupOutput, s.lookupError
}
func (s *fakeDownloadAppStore) Download(input appstore.DownloadInput) (appstore.DownloadOutput, error) {
	s.downloadInputs = append(s.downloadInputs, input)

	return appstore.DownloadOutput{DestinationPath: "/tmp/delisted-test.ipa"}, nil
}
func (*fakeDownloadAppStore) ReplicateSinf(appstore.ReplicateSinfInput) error {
	return nil
}
