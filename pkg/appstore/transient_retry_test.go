package appstore

import (
	"errors"
	gohttp "net/http"
	"time"

	"github.com/majd/ipatool/v2/pkg/http"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("transient retry client", func() {
	var (
		ctrl        *gomock.Controller
		inner       *http.MockClient[downloadResult]
		client      http.Client[downloadResult]
		request     http.Request
		successResp http.Result[downloadResult]
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		inner = http.NewMockClient[downloadResult](ctrl)
		client = transientRetryClient[downloadResult]{
			inner: inner,
			sleep: func(time.Duration) {},
		}
		request = http.Request{URL: "https://example.com"}
		successResp = http.Result[downloadResult]{
			StatusCode: gohttp.StatusOK,
			Data:       downloadResult{Items: []downloadItemResult{{URL: "https://cdn.example.com/app.ipa"}}},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	It("retries a 504 and returns the successful response", func() {
		gatewayTimeout := &http.UnexpectedResponseError{StatusCode: gohttp.StatusGatewayTimeout}

		gomock.InOrder(
			inner.EXPECT().Send(gomock.Any()).
				Return(http.Result[downloadResult]{}, gatewayTimeout),
			inner.EXPECT().Send(gomock.Any()).
				Return(successResp, nil),
		)

		res, err := client.Send(request)
		Expect(err).ToNot(HaveOccurred())
		Expect(res).To(Equal(successResp))
	})

	It("retries a 503 and a 429 in sequence", func() {
		gomock.InOrder(
			inner.EXPECT().Send(gomock.Any()).
				Return(http.Result[downloadResult]{}, &http.UnexpectedResponseError{StatusCode: gohttp.StatusServiceUnavailable}),
			inner.EXPECT().Send(gomock.Any()).
				Return(http.Result[downloadResult]{}, &http.UnexpectedResponseError{StatusCode: gohttp.StatusTooManyRequests}),
			inner.EXPECT().Send(gomock.Any()).
				Return(successResp, nil),
		)

		_, err := client.Send(request)
		Expect(err).ToNot(HaveOccurred())
	})

	It("gives up after three attempts on a persistent 504", func() {
		gatewayTimeout := &http.UnexpectedResponseError{StatusCode: gohttp.StatusGatewayTimeout}

		inner.EXPECT().Send(gomock.Any()).
			Return(http.Result[downloadResult]{}, gatewayTimeout).
			Times(3)

		_, err := client.Send(request)
		Expect(errors.Is(err, gatewayTimeout)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("after 3 attempts"))
		Expect(err.Error()).To(ContainSubstring("504, 504, 504"))
	})

	DescribeTable("does not retry non-transient responses",
		func(status int) {
			original := &http.UnexpectedResponseError{StatusCode: status}

			inner.EXPECT().Send(gomock.Any()).
				Return(http.Result[downloadResult]{}, original)

			_, err := client.Send(request)
			Expect(errors.Is(err, original)).To(BeTrue())
		},
		Entry("forbidden", gohttp.StatusForbidden),
		Entry("server error 500 (redownload signal)", gohttp.StatusInternalServerError),
		Entry("bad request", gohttp.StatusBadRequest),
	)

	It("passes non-response errors through without retrying", func() {
		original := errors.New("connection reset")

		inner.EXPECT().Send(gomock.Any()).
			Return(http.Result[downloadResult]{}, original)

		_, err := client.Send(request)
		Expect(errors.Is(err, original)).To(BeTrue())
	})

	It("passes Do and NewRequest through to the inner client", func() {
		var called bool

		inner.EXPECT().
			Do(gomock.Any()).
			DoAndReturn(func(req *gohttp.Request) (*gohttp.Response, error) {
				called = true
				return &gohttp.Response{StatusCode: gohttp.StatusOK}, nil
			})

		_, err := client.Do(&gohttp.Request{})
		Expect(err).ToNot(HaveOccurred())
		Expect(called).To(BeTrue())
	})
})
