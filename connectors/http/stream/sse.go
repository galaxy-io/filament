package stream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/obs"
)

// sseReader implements the WHATWG Server-Sent Events parser, simplified:
//
//   - Lines starting with `:` are comments (ignored).
//   - `event: <name>` sets the event type for the in-progress block.
//   - `data: <payload>` appends to the in-progress block (multiple data lines
//     are joined with newlines, per spec).
//   - A blank line dispatches the block as one record. We unmarshal the
//     concatenated data as JSON.
//
// When eventFilter is non-empty, only blocks whose `event:` matches are
// emitted. Empty filter matches every block.
//
// Malformed-block policy mirrors ndjsonReader: maxErrors gates whether a bad
// JSON payload aborts (default strict) or is logged-and-skipped.
type sseReader struct {
	eventFilter string
	logger      *slog.Logger
	maxErrors   int
}

func (r *sseReader) Read(ctx context.Context, body io.Reader, emit EmitFunc) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), ndjsonDefaultMaxLine)
	logger := obs.Logger(r.logger)

	var event string
	var data strings.Builder
	var blockNum int
	var errCount int

	dispatch := func() error {
		// Reset block state in defer so empty/filtered blocks don't leak
		// `event:` into the next block.
		defer func() {
			event = ""
			data.Reset()
		}()
		if data.Len() == 0 {
			return nil
		}
		blockNum++
		if r.eventFilter != "" && event != r.eventFilter {
			return nil
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(data.String()), &record); err != nil {
			errCount++
			logger.Warn("sse malformed data block",
				"block", blockNum,
				"errors_so_far", errCount,
				"max_errors", r.maxErrors,
				"event", event,
				"error", err)
			if r.maxErrors == -1 {
				return nil
			}
			if errCount > r.maxErrors {
				return fmt.Errorf("%w: block %d (event=%q): %v",
					errs.ErrMalformedLine, blockNum, event, err)
			}
			return nil
		}
		return emit(record)
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue // comment
		}
		field, value, ok := splitSSEField(line)
		if !ok {
			continue
		}
		switch field {
		case "event":
			event = value
		case "data":
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(value)
		}
		// id:, retry: ignored — not relevant for record extraction.
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("sse scan: %w", err)
	}
	// Dispatch trailing block (no terminating blank line). Spec-strict
	// readers would discard this, but real-world streams that close
	// abruptly carry a final block worth surfacing.
	return dispatch()
}

// splitSSEField parses `field: value` (with optional space after colon).
func splitSSEField(line string) (field, value string, ok bool) {
	i := strings.Index(line, ":")
	if i < 0 {
		return "", "", false
	}
	field = line[:i]
	value = line[i+1:]
	value = strings.TrimPrefix(value, " ")
	return field, value, true
}
