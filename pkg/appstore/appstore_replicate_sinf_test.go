package appstore

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/majd/ipatool/v2/pkg/http"
	"github.com/majd/ipatool/v2/pkg/keychain"
	"github.com/majd/ipatool/v2/pkg/util/machine"
	"github.com/majd/ipatool/v2/pkg/util/operatingsystem"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"howett.net/plist"
)

var _ = Describe("AppStore (ReplicateSinf)", func() {
	var (
		ctrl               *gomock.Controller
		mockKeychain       *keychain.MockKeychain
		mockDownloadClient *http.MockClient[downloadResult]
		mockPurchaseClient *http.MockClient[purchaseResult]
		mockLoginClient    *http.MockClient[loginResult]
		mockHTTPClient     *http.MockClient[interface{}]
		mockOS             *operatingsystem.MockOperatingSystem
		mockMachine        *machine.MockMachine
		as                 AppStore
		testFile           *os.File
		testZip            *zip.Writer
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockKeychain = keychain.NewMockKeychain(ctrl)
		mockDownloadClient = http.NewMockClient[downloadResult](ctrl)
		mockLoginClient = http.NewMockClient[loginResult](ctrl)
		mockPurchaseClient = http.NewMockClient[purchaseResult](ctrl)
		mockHTTPClient = http.NewMockClient[interface{}](ctrl)
		mockOS = operatingsystem.NewMockOperatingSystem(ctrl)
		mockMachine = machine.NewMockMachine(ctrl)
		as = &appstore{
			keychain:       mockKeychain,
			loginClient:    mockLoginClient,
			purchaseClient: mockPurchaseClient,
			downloadClient: mockDownloadClient,
			httpClient:     mockHTTPClient,
			machine:        mockMachine,
			os:             mockOS,
		}

		var err error
		testFile, err = os.CreateTemp("", "test_file")
		Expect(err).ToNot(HaveOccurred())

		testZip = zip.NewWriter(testFile)
	})

	JustBeforeEach(func() {
		testZip.Close()
	})

	AfterEach(func() {
		err := os.Remove(testFile.Name())
		Expect(err).ToNot(HaveOccurred())

		ctrl.Finish()
	})

	When("app includes codesign manifest", func() {
		BeforeEach(func() {
			mockOS.EXPECT().
				OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(os.OpenFile)

			mockOS.EXPECT().
				Remove(testFile.Name()).
				Return(nil)

			mockOS.EXPECT().
				Rename(fmt.Sprintf("%s.tmp", testFile.Name()), testFile.Name()).
				Return(nil)

			manifest, err := plist.Marshal(packageManifest{
				SinfPaths: []string{
					"SC_Info/TestApp.sinf",
				},
			}, plist.BinaryFormat)
			Expect(err).ToNot(HaveOccurred())

			w, err := testZip.Create("Payload/Test.app/SC_Info/Manifest.plist")
			Expect(err).ToNot(HaveOccurred())

			_, err = w.Write(manifest)
			Expect(err).ToNot(HaveOccurred())

			w, err = testZip.Create("Payload/Test.app/Info.plist")
			Expect(err).ToNot(HaveOccurred())

			info, err := plist.Marshal(map[string]interface{}{
				"CFBundleExecutable": "Test",
			}, plist.BinaryFormat)
			Expect(err).ToNot(HaveOccurred())

			_, err = w.Write(info)
			Expect(err).ToNot(HaveOccurred())

			w, err = testZip.Create("Payload/Test.app/Watch/Test.app/Info.plist")
			Expect(err).ToNot(HaveOccurred())

			watchInfo, err := plist.Marshal(map[string]interface{}{
				"WKWatchKitApp": true,
			}, plist.BinaryFormat)
			Expect(err).ToNot(HaveOccurred())

			_, err = w.Write(watchInfo)
			Expect(err).ToNot(HaveOccurred())
		})

		It("replicates sinf from manifest plist", func() {
			err := as.ReplicateSinf(ReplicateSinfInput{
				PackagePath: testFile.Name(),
				Sinfs: []Sinf{
					{
						ID:   0,
						Data: []byte(""),
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("manifest declares more sinf paths than the store returned (Google-style package)", func() {
		BeforeEach(func() {
			mockOS.EXPECT().
				OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(os.OpenFile)

			// The original is replaced by the patched temp file; keep the temp
			// file around so the test can inspect it.
			mockOS.EXPECT().
				Remove(testFile.Name()).
				Return(nil)

			mockOS.EXPECT().
				Rename(fmt.Sprintf("%s.tmp", testFile.Name()), testFile.Name()).
				Return(nil)

			manifest, err := plist.Marshal(packageManifest{
				SinfPaths: []string{
					"SC_Info/Google.sinf",
					"PlugIns/Widget.appex/SC_Info/Widget.sinf",
					"PlugIns/Share.appex/SC_Info/Share.sinf",
				},
			}, plist.BinaryFormat)
			Expect(err).ToNot(HaveOccurred())

			w, err := testZip.Create("Payload/Google.app/SC_Info/Manifest.plist")
			Expect(err).ToNot(HaveOccurred())

			_, err = w.Write(manifest)
			Expect(err).ToNot(HaveOccurred())

			w, err = testZip.Create("Payload/Google.app/Info.plist")
			Expect(err).ToNot(HaveOccurred())

			info, err := plist.Marshal(map[string]interface{}{
				"CFBundleExecutable": "Google",
			}, plist.BinaryFormat)
			Expect(err).ToNot(HaveOccurred())

			_, err = w.Write(info)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			_ = os.Remove(fmt.Sprintf("%s.tmp", testFile.Name()))
		})

		It("writes the sinfs it has instead of failing", func() {
			err := as.ReplicateSinf(ReplicateSinfInput{
				PackagePath: testFile.Name(),
				Sinfs: []Sinf{
					{ID: 0, Data: []byte("main-sinf")},
					{ID: 2, Data: []byte("share-sinf")},
				},
			})
			Expect(err).ToNot(HaveOccurred())

			patched, err := zip.OpenReader(fmt.Sprintf("%s.tmp", testFile.Name()))
			Expect(err).ToNot(HaveOccurred())
			defer patched.Close()

			contents := map[string]string{}
			for _, f := range patched.File {
				rc, err := f.Open()
				Expect(err).ToNot(HaveOccurred())
				data, err := io.ReadAll(rc)
				Expect(err).ToNot(HaveOccurred())
				rc.Close()
				contents[f.Name] = string(data)
			}

			Expect(contents).To(HaveKeyWithValue("Payload/Google.app/SC_Info/Google.sinf", "main-sinf"))
			Expect(contents).To(HaveKeyWithValue("Payload/Google.app/PlugIns/Share.appex/SC_Info/Share.sinf", "share-sinf"))
			Expect(contents).ToNot(HaveKey("Payload/Google.app/PlugIns/Widget.appex/SC_Info/Widget.sinf"))
		})
	})

	Describe("resolveSinfTargets", func() {
		paths := []string{"SC_Info/A.sinf", "PlugIns/B.appex/SC_Info/B.sinf", "PlugIns/C.appex/SC_Info/C.sinf"}

		It("maps positionally when the counts match", func() {
			targets := resolveSinfTargets([]Sinf{{ID: 7, Data: []byte("a")}, {ID: 8, Data: []byte("b")}, {ID: 9, Data: []byte("c")}}, paths)
			Expect(targets).To(Equal([]sinfTarget{
				{Path: paths[0], Data: []byte("a")},
				{Path: paths[1], Data: []byte("b")},
				{Path: paths[2], Data: []byte("c")},
			}))
		})

		It("replicates a single sinf to every declared path", func() {
			targets := resolveSinfTargets([]Sinf{{ID: 0, Data: []byte("only")}}, paths)
			Expect(targets).To(HaveLen(3))
			for i, target := range targets {
				Expect(target.Path).To(Equal(paths[i]))
				Expect(target.Data).To(Equal([]byte("only")))
			}
		})

		It("uses the sinf ids as indexes when the counts differ", func() {
			targets := resolveSinfTargets([]Sinf{{ID: 2, Data: []byte("c")}, {ID: 0, Data: []byte("a")}}, paths)
			Expect(targets).To(Equal([]sinfTarget{
				{Path: paths[0], Data: []byte("a")},
				{Path: paths[2], Data: []byte("c")},
			}))
		})

		It("falls back to positional mapping when the ids are unusable", func() {
			targets := resolveSinfTargets([]Sinf{{ID: 42, Data: []byte("a")}, {ID: 42, Data: []byte("b")}}, paths)
			Expect(targets).To(Equal([]sinfTarget{
				{Path: paths[0], Data: []byte("a")},
				{Path: paths[1], Data: []byte("b")},
			}))
		})

		It("ignores surplus sinfs when there are fewer paths", func() {
			targets := resolveSinfTargets([]Sinf{{ID: 0, Data: []byte("a")}, {ID: 1, Data: []byte("b")}}, paths[:1])
			Expect(targets).To(Equal([]sinfTarget{{Path: paths[0], Data: []byte("a")}}))
		})

		It("returns nothing when either side is empty", func() {
			Expect(resolveSinfTargets(nil, paths)).To(BeEmpty())
			Expect(resolveSinfTargets([]Sinf{{Data: []byte("a")}}, nil)).To(BeEmpty())
		})
	})

	When("app does not include codesign manifest", func() {
		BeforeEach(func() {
			mockOS.EXPECT().
				OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(os.OpenFile)

			mockOS.EXPECT().
				Remove(testFile.Name()).
				Return(nil)

			mockOS.EXPECT().
				Rename(fmt.Sprintf("%s.tmp", testFile.Name()), testFile.Name()).
				Return(nil)

			w, err := testZip.Create("Payload/Test.app/Info.plist")
			Expect(err).ToNot(HaveOccurred())

			info, err := plist.Marshal(map[string]interface{}{
				"CFBundleExecutable": "Test",
			}, plist.BinaryFormat)
			Expect(err).ToNot(HaveOccurred())

			_, err = w.Write(info)
			Expect(err).ToNot(HaveOccurred())

			w, err = testZip.Create("Payload/Test.app/Watch/Test.app/Info.plist")
			Expect(err).ToNot(HaveOccurred())

			watchInfo, err := plist.Marshal(map[string]interface{}{
				"WKWatchKitApp": true,
			}, plist.BinaryFormat)
			Expect(err).ToNot(HaveOccurred())

			_, err = w.Write(watchInfo)
			Expect(err).ToNot(HaveOccurred())
		})

		It("replicates sinf", func() {
			err := as.ReplicateSinf(ReplicateSinfInput{
				PackagePath: testFile.Name(),
				Sinfs: []Sinf{
					{
						ID:   0,
						Data: []byte(""),
					},
				},
			})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	When("fails to open file", func() {
		BeforeEach(func() {
			mockOS.EXPECT().
				OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, errors.New(""))
		})

		It("returns error", func() {
			err := as.ReplicateSinf(ReplicateSinfInput{
				PackagePath: testFile.Name(),
			})
			Expect(err).To(HaveOccurred())
		})
	})
})
