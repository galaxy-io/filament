package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/obs"
)

// chunkedArrayReader incrementally decodes a JSON array of objects without
// buffering the entire response. Useful for APIs that return very large
// response bodies as `[ {...}, {...}, ... ]`, or wrapped variants like
// `{"data":{"items":[...]}}` (set arrayPath to navigate to the array).
type chunkedArrayReader struct {
	arrayPath       string // dot-path to wrapping array; "" means root array
	maxElementBytes int    // 0 means no per-element cap
	logger          *slog.Logger
}

func (r *chunkedArrayReader) Read(ctx context.Context, body io.Reader, emit EmitFunc) error {
	dec := json.NewDecoder(body)
	logger := obs.Logger(r.logger)

	if r.arrayPath != "" {
		if err := descendToArray(dec, r.arrayPath); err != nil {
			return err
		}
	} else {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("chunked_array: read open token: %w", err)
		}
		if d, ok := tok.(json.Delim); !ok || d != '[' {
			return fmt.Errorf("chunked_array: expected '[' at root, got %v", tok)
		}
	}

	for dec.More() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return fmt.Errorf("chunked_array: decode element: %w", err)
		}
		if r.maxElementBytes > 0 && len(raw) > r.maxElementBytes {
			logger.Warn("chunked_array skip oversize element",
				"size", len(raw), "max", r.maxElementBytes)
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(raw, &record); err != nil {
			return fmt.Errorf("chunked_array: unmarshal element: %w", err)
		}
		if err := emit(record); err != nil {
			return err
		}
	}

	// Validate the array's closing `]` so truncated bodies
	// (`{"data":{"items":[{"id":1}` with no closing bracket) surface as
	// ErrTruncatedStream rather than a silent success — dec.More() alone
	// returns false on EOF without complaint.
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("%w: chunked_array (path=%q): %v",
			errs.ErrTruncatedStream, r.arrayPath, err)
	}
	if d, ok := tok.(json.Delim); !ok || d != ']' {
		return fmt.Errorf("%w: chunked_array (path=%q): expected ']', got %v",
			errs.ErrTruncatedStream, r.arrayPath, tok)
	}
	// Trailing JSON tail (e.g. sibling `meta` after our nested array) is
	// allowed and not consumed — caller's body reader closes when done.
	return nil
}

// descendToArray walks the json.Decoder token stream into an object tree until
// it consumes the opening `[` of the array at the configured path.
func descendToArray(dec *json.Decoder, path string) error {
	// paths.Split honors backslash-escaped dots so an array_path like
	// `data.user\.events` resolves to two segments, not three.
	parts := paths.Split(path)
	for depth, target := range parts {
		// Expect '{' at this level.
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("chunked_array: descend %q: %w", path, err)
		}
		if d, ok := tok.(json.Delim); !ok || d != '{' {
			return fmt.Errorf("chunked_array: expected '{' at depth %d, got %v", depth, tok)
		}
		// Scan keys until we hit `target`.
		found := false
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return fmt.Errorf("chunked_array: scan key at depth %d: %w", depth, err)
			}
			key, ok := keyTok.(string)
			if !ok {
				return fmt.Errorf("chunked_array: non-string key at depth %d: %v", depth, keyTok)
			}
			if key == target {
				found = true
				if depth == len(parts)-1 {
					// Final segment — expect `[`.
					arrTok, err := dec.Token()
					if err != nil {
						return fmt.Errorf("chunked_array: read array open: %w", err)
					}
					if d, ok := arrTok.(json.Delim); !ok || d != '[' {
						return fmt.Errorf("chunked_array: expected '[' at %s, got %v", path, arrTok)
					}
					return nil
				}
				// Not final — fall into the nested object.
				break
			}
			// Skip non-target value.
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return fmt.Errorf("chunked_array: skip value at depth %d: %w", depth, err)
			}
		}
		if !found {
			return fmt.Errorf("chunked_array: path %q: key %q not found at depth %d", path, target, depth)
		}
	}
	return nil
}
