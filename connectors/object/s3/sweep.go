package s3

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// defaultStaleUploadAge is how old an incomplete upload must be before the
	// sweep aborts it. A live write never holds an upload this long unless a
	// single bounded run lasts longer, so it is set well past typical runs.
	defaultStaleUploadAge = 24 * time.Hour
	minStaleUploadAge     = time.Hour
	// sweepInterval spaces sweeps of one bucket prefix within a process, since
	// a continuous run opens a sink at every boundary.
	sweepInterval = time.Hour
	sweepTimeout  = 30 * time.Second
)

// pendingUpload is an incomplete multipart upload reported by the store.
type pendingUpload struct {
	key, uploadID string
	initiated     time.Time
}

// objectName matches the objects this sink writes, <write id>.<file number>
// followed by the format extension, so the sweep leaves other writers'
// uploads under the same prefix alone. A write ID is a run or boundary UUID,
// or a local CLI run's cli-<base 36 time>.
var objectName = regexp.MustCompile(`^(?:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|cli-[0-9a-z]+)\.\d{4,}(?:\.[a-z0-9]+)+$`)

type sweepScope struct{ endpoint, bucket, prefix string }

// sweeps records when each bucket prefix was last swept in this process.
var sweeps = struct {
	sync.Mutex
	last map[sweepScope]time.Time
}{last: make(map[sweepScope]time.Time)}

// sweepClock is replaced in tests.
var sweepClock = time.Now

// sweepStaleUploads aborts incomplete multipart uploads of this sink's objects
// under the configured prefix that started before the stale age. Without a
// prefix the sink would reach every pipeline writing to the bucket, so it does
// not sweep at all. A worker that
// dies without running Abort leaves its uploads open, and S3 keeps and bills
// them until they are removed. The sweep runs at most once per sweepInterval
// for a bucket prefix in this process. It returns the uploads it aborted.
func sweepStaleUploads(ctx context.Context, store multipartStore, cfg sinkConfig) (int, error) {
	if cfg.staleUploadAge <= 0 || cfg.prefix == "" {
		return 0, nil
	}
	now := sweepClock()
	scope := sweepScope{endpoint: cfg.endpoint, bucket: cfg.bucket, prefix: cfg.prefix}
	sweeps.Lock()
	if last, ok := sweeps.last[scope]; ok && now.Sub(last) < sweepInterval {
		sweeps.Unlock()
		return 0, nil
	}
	// Claimed before listing, so a failing sweep is not retried every boundary.
	sweeps.last[scope] = now
	sweeps.Unlock()

	ctx, cancel := context.WithTimeout(ctx, sweepTimeout)
	defer cancel()
	prefix := cfg.prefix + "/"
	// The whole bucket is listed and filtered here: MinIO matches a listing
	// prefix only against complete keys, so a prefix finds nothing there.
	// Incomplete uploads are few, so the listing stays small.
	uploads, err := store.ListMultipartUploads(ctx, cfg.bucket, "")
	if err != nil {
		return 0, fmt.Errorf("list incomplete multipart uploads: %w", err)
	}
	cutoff := now.Add(-cfg.staleUploadAge)
	aborted := 0
	var errs []error
	for _, upload := range uploads {
		if !strings.HasPrefix(upload.key, prefix) || !upload.initiated.Before(cutoff) || !objectName.MatchString(path.Base(upload.key)) {
			continue
		}
		if err := store.AbortMultipart(ctx, cfg.bucket, upload.key, upload.uploadID); err != nil {
			errs = append(errs, fmt.Errorf("abort %s: %w", upload.key, err))
			continue
		}
		aborted++
	}
	return aborted, errors.Join(errs...)
}

// sweepOnOpen runs the sweep for a newly opened sink. It is best effort: a
// failure, such as a missing s3:ListBucketMultipartUploads permission, is
// logged and never fails the run.
func sweepOnOpen(ctx context.Context, store multipartStore, cfg sinkConfig, run string) {
	aborted, err := sweepStaleUploads(ctx, store, cfg)
	if err != nil {
		slog.WarnContext(ctx, "S3 stale multipart upload sweep failed", "component", "s3", "event.name", "s3.sweep.failed",
			"run_id", run, "bucket", cfg.bucket, "prefix", cfg.prefix, "aborted", aborted, "error", err)
		return
	}
	if aborted > 0 {
		slog.InfoContext(ctx, "Aborted stale S3 multipart uploads", "component", "s3", "event.name", "s3.sweep.aborted",
			"run_id", run, "bucket", cfg.bucket, "prefix", cfg.prefix, "aborted", aborted, "older_than", cfg.staleUploadAge.String())
	}
}
