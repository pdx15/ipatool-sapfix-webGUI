package http

type ResponseFormat string

const (
	ResponseFormatJSON ResponseFormat = "json"
	ResponseFormatXML  ResponseFormat = "xml"
	// ResponseFormatRaw returns the response body untouched. The client's
	// result type must be []byte.
	ResponseFormatRaw ResponseFormat = "raw"
)

const (
	DefaultUserAgent = "Configurator/2.17 (Macintosh; OS X 15.2; 24C5089c) AppleWebKit/0620.1.16.11.6"

	HeaderAppleActionSignature = "X-Apple-ActionSignature"
)
