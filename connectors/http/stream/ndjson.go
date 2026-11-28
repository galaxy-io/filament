package stream

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/obs"
)

const (
	ndjsonDefaultMaxLine = 10 * 1024 * 1024 // 10MB per line
	ndjsonPreviewBytes   = 80
)

// ndjsonReader emits one record per non-empty line.
//
// Malformed lines policy:
//   - maxErrors == 0  (default): strict — first bad line errors with
//     errs.ErrMalformedLine and the line number.
//   - maxErrors >  0           : allow up to N bad lines, log each at WARN;
//     the (N+1)th errors.
//   - maxErrors == -1          : unbounded; every bad line is logged but
//     never errors. Opt-in only, since it allows silent data loss.
type ndjsonReader struct {
	logger    *slog.Logger
	maxLine   int
	maxErrors int
	// emptyBody is the manifest's stream.empty_body: a whole body equal to
	// it, trimmed, is a result with no records.
	emptyBody string
}

func newNDJSON(spec *manifest.StreamSpec, opts Options) (Reader, error) {
	r := &ndjsonReader{logger: opts.Logger, maxLine: opts.NDJSONMaxLine, maxErrors: opts.NDJSONMaxErrors}
	if spec != nil {
		r.emptyBody = spec.EmptyBody
	}
	return r, nil
}

// emptyBodySlack is how much surrounding whitespace an empty-body marker may
// carry, such as a trailing newline, and still match.
const emptyBodySlack = 64

func (r *ndjsonReader) Read(ctx context.Context, body io.Reader, emit EmitFunc) error {
	if r.emptyBody != "" {
		// Peek only as far as a marker could reach, so a large export is
		// never buffered here. A body that ends within the peek and equals
		// the marker once trimmed has no records.
		buffered := bufio.NewReaderSize(body, len(r.emptyBody)+emptyBodySlack)
		head, err := buffered.Peek(len(r.emptyBody) + emptyBodySlack)
		if err == io.EOF && string(bytes.TrimSpace(head)) == r.emptyBody {
			return nil
		}
		body = buffered
	}
	maxLine := r.maxLine
	if maxLine <= 0 {
		maxLine = ndjsonDefaultMaxLine
	}
	logger := obs.Logger(r.logger)

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), maxLine)

	var lineNum int
	var errors int

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			errors++
			logger.Warn("ndjson malformed line",
				"line", lineNum,
				"errors_so_far", errors,
				"max_errors", r.maxErrors,
				"error", err,
				"preview", preview(line, ndjsonPreviewBytes))
			if r.maxErrors == -1 {
				continue
			}
			if errors > r.maxErrors {
				return fmt.Errorf("%w: line %d: %v (preview=%q)",
					errs.ErrMalformedLine, lineNum, err, preview(line, ndjsonPreviewBytes))
			}
			continue
		}
		if err := emit(record); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("ndjson scan: %w", err)
	}
	return nil
}

func preview(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
