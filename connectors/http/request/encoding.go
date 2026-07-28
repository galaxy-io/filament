// Package request builds outgoing HTTP requests from a v1 manifest.Resource.
//
// The Builder renders the resource against a template.Scope, encodes the body
// per the resource's encoding choice, applies arbitrary connection + resource
// headers and query params, and finally invokes an auth.Authenticator.
//
// A Limiter (static or dynamic) gates outgoing requests and observes
// rate-limit response headers.
package request

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
)

// Encoder serializes a Body template into an io.Reader and reports the
// Content-Type that should accompany it. Encoding "none" returns (nil, "").
type Encoder interface {
	Encode(template any) (body io.Reader, contentType string, err error)
}

// EncoderFor selects an Encoder by name. Empty defaults to "json" when there
// is a non-nil template, "none" otherwise.
func EncoderFor(name string, template any) (Encoder, error) {
	if name == "" {
		if template == nil {
			return noneEncoder{}, nil
		}
		name = "json"
	}
	switch name {
	case "none":
		return noneEncoder{}, nil
	case "json":
		return jsonEncoder{}, nil
	case "form":
		return formEncoder{}, nil
	case "multipart":
		return multipartEncoder{}, nil
	case "raw":
		return rawEncoder{}, nil
	}
	return nil, fmt.Errorf("encoding: unknown kind %q", name)
}

type noneEncoder struct{}

func (noneEncoder) Encode(_ any) (io.Reader, string, error) { return nil, "", nil }

type jsonEncoder struct{}

func (jsonEncoder) Encode(t any) (io.Reader, string, error) {
	if t == nil {
		return nil, "application/json", nil
	}
	data, err := json.Marshal(t)
	if err != nil {
		return nil, "", fmt.Errorf("json encode: %w", err)
	}
	return bytes.NewReader(data), "application/json", nil
}

// formEncoder takes a map[string]any of scalar values and emits
// application/x-www-form-urlencoded. Non-scalar values (maps, slices) are
// rejected so the caller never produces form bodies like "[1 2 3]" or
// "map[k:v]" that servers silently misparse.
type formEncoder struct{}

func (formEncoder) Encode(t any) (io.Reader, string, error) {
	m, ok := t.(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("form encode: template must be a map, got %T", t)
	}
	form := url.Values{}
	for k, v := range m {
		s, err := scalarToString(v)
		if err != nil {
			return nil, "", fmt.Errorf("form encode: field %q: %w", k, err)
		}
		form.Set(k, s)
	}
	return strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", nil
}

// scalarToString stringifies JSON scalar values losslessly. Numbers use
// strconv (not %v) so floats don't pick up scientific notation. Returns an
// error for non-scalar types so encoders surface bad inputs explicitly.
func scalarToString(v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int:
		return strconv.Itoa(x), nil
	case int32:
		return strconv.FormatInt(int64(x), 10), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	}
	return "", fmt.Errorf("non-scalar value of type %T (form/multipart fields must be string|number|bool|null)", v)
}

// multipartEncoder takes a map[string]any. Scalar values become form fields;
// map values shaped {"filename": ..., "content": ..., "content_type": ...}
// become file parts. When supplied, content_type sets the part's
// Content-Type header so binary uploads tag correctly.
type multipartEncoder struct{}

func (multipartEncoder) Encode(t any) (io.Reader, string, error) {
	m, ok := t.(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("multipart encode: template must be a map, got %T", t)
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range m {
		switch x := v.(type) {
		case map[string]any:
			filename, _ := x["filename"].(string)
			content, _ := x["content"].(string)
			contentType, _ := x["content_type"].(string)
			if err := writeFilePart(w, k, filename, contentType, content); err != nil {
				return nil, "", fmt.Errorf("multipart file %q: %w", k, err)
			}
		default:
			s, err := scalarToString(v)
			if err != nil {
				return nil, "", fmt.Errorf("multipart field %q: %w", k, err)
			}
			if err := w.WriteField(k, s); err != nil {
				return nil, "", fmt.Errorf("multipart write field %q: %w", k, err)
			}
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("multipart close: %w", err)
	}
	return &buf, w.FormDataContentType(), nil
}

// writeFilePart writes one file form-part. When contentType is empty, falls
// back to multipart.Writer.CreateFormFile (which uses application/octet-stream).
func writeFilePart(w *multipart.Writer, field, filename, contentType, content string) error {
	if contentType == "" {
		fw, err := w.CreateFormFile(field, filename)
		if err != nil {
			return err
		}
		_, err = fw.Write([]byte(content))
		return err
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name=%q; filename=%q`, field, filename))
	h.Set("Content-Type", contentType)
	fw, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = fw.Write([]byte(content))
	return err
}

// rawEncoder writes the template as a literal body. Two shapes:
//
//	"hello world"                             — body, default octet-stream
//	{ "body": "...", "content_type": "..." }  — body with explicit Content-Type
//
// The map form is the only way to send non-octet-stream raw payloads (XML,
// HTML, custom MIME types) without going through json/form encoders.
type rawEncoder struct{}

func (rawEncoder) Encode(t any) (io.Reader, string, error) {
	switch x := t.(type) {
	case string:
		return strings.NewReader(x), "application/octet-stream", nil
	case map[string]any:
		body, ok := x["body"].(string)
		if !ok {
			return nil, "", fmt.Errorf("raw encode: map form requires string \"body\" (got %T)", x["body"])
		}
		ct, _ := x["content_type"].(string)
		if ct == "" {
			ct = "application/octet-stream"
		}
		return strings.NewReader(body), ct, nil
	}
	return nil, "", fmt.Errorf("raw encode: template must be a string or {body, content_type} map, got %T", t)
}
