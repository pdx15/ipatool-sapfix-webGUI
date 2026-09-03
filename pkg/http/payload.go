package http

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"

	"howett.net/plist"
)

type Payload interface {
	data() ([]byte, error)
}

type XMLPayload struct {
	Content map[string]interface{}
}

type URLPayload struct {
	Content map[string]interface{}
}

// RawPayload sends pre-encoded bytes as the request body.
type RawPayload struct {
	Content []byte
}

func (p *RawPayload) data() ([]byte, error) {
	return p.Content, nil
}

func (p *XMLPayload) data() ([]byte, error) {
	buffer := new(bytes.Buffer)

	err := plist.NewEncoder(buffer).Encode(p.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to encode plist: %w", err)
	}

	return buffer.Bytes(), nil
}

func (p *URLPayload) data() ([]byte, error) {
	params := url.Values{}

	for key, val := range p.Content {
		switch t := val.(type) {
		case string:
			params.Add(key, val.(string))
		case int:
			params.Add(key, strconv.Itoa(val.(int)))
		default:
			return nil, fmt.Errorf("value type is not supported (%s)", t)
		}
	}

	return []byte(params.Encode()), nil
}
