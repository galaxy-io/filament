package stream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/galaxy-io/filament/connectors/http/errs"
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
}

func (r *ndjsonReader) Read(ctx context.Context, body io.Reader, emit EmitFunc) error {
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
