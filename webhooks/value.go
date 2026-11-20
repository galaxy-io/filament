package webhooks

import (
	"fmt"
	"net/http"
	"net/textproto"

	"github.com/tidwall/gjson"
)

// ValueSource names where a value is in a request: a header or a gjson path
// into the JSON body. Exactly one is set.
type ValueSource struct {
	Header string `json:"header,omitempty"`
	Body   string `json:"body,omitempty"`
}

func (s ValueSource) validate(what string) error {
	if (s.Header == "") == (s.Body == "") {
		return fmt.Errorf("%s needs exactly one of header or body", what)
	}
	return nil
}

// canonical returns the source with its header name canonicalized.
func (s ValueSource) canonical() ValueSource {
	if s.Header != "" {
		s.Header = textproto.CanonicalMIMEHeaderKey(s.Header)
	}
	return s
}

// lookup returns the single value the source names. A repeated header, a
// body that is not JSON and a body value that is not a string or number all
// count as missing.
func (s ValueSource) lookup(header http.Header, body []byte) (string, bool) {
	switch {
	case s.Header != "":
		values := header.Values(s.Header)
		if len(values) != 1 {
			return "", false
		}
		return values[0], true
	case s.Body != "":
		if !gjson.ValidBytes(body) {
			return "", false
		}
		result := gjson.GetBytes(body, s.Body)
		if result.Type != gjson.String && result.Type != gjson.Number {
			return "", false
		}
		return result.String(), true
	}
	return "", false
}

// deliveryID reads a provider delivery ID and checks it is a safe token.
func deliveryID(source ValueSource, header http.Header, body []byte) (string, error) {
	id, ok := source.lookup(header, body)
	if !ok || !validToken(id, maxDeliveryID) {
		return "", ErrUnauthorized
	}
	return id, nil
}
