// Package s3 implements the ingestion.Sink interface, writing each resource to its own
// NDJSON object in an S3 bucket.
//
// A run lands one object per resource at <prefix>/<run>/<resource>.ndjson, each
// record serialized to its own JSON line — the same dev-readable shape the stdout
// sink emits. Because S3 objects are not appendable, each resource is streamed via
// a single multipart upload: Write buffers records per resource and hands a full
// part to a background uploader, Commit waits for in-flight parts, flushes the
// tails, and completes every upload in parallel; Abort cancels them so a failed
// run leaves no object behind.
//
// Parts upload asynchronously so encoding the next part overlaps the network
// round-trip of the previous one; a sink-wide semaphore caps total in-flight
// parts and full-size buffers are pooled so steady state allocates nothing.
//
// Each Write's WriteCRC is computed over the records it persists, letting the engine
// verify the batch reached the sink intact.
package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/galaxy-io/filament"
)

// s3debug enables per-call S3 API timing to stderr; set S3_SINK_DEBUG=1 to turn on.
// TEMP: instrumentation for diagnosing end-of-run multipart stalls.
var s3debug = os.Getenv("S3_SINK_DEBUG") != ""

func s3debugf(format string, args ...any) {
	if s3debug {
		log.Printf("s3sink "+format, args...)
	}
}

const (
	// minPartSize is S3's floor for every multipart part except the last (5 MiB).
	minPartSize = 5 << 20
	// defaultPartSize trades buffer memory for fewer, fatter requests.
	defaultPartSize = 16 << 20
	// defaultConcurrency caps in-flight part uploads across all resources.
	defaultConcurrency = 8
)

// Sink streams each resource to its own NDJSON object via a per-resource multipart
// upload with asynchronous part uploads.
type Sink struct {
	client   *s3.Client
	bucket   string
	prefix   string
	run      ingestion.RunID
	partSize int

	sem         chan struct{} // sink-wide in-flight part limiter
	bufPool     sync.Pool     // *[]byte part buffers, cap >= partSize
	scratchPool sync.Pool     // *[]byte encode scratch, kept off the upload buffers

	mu      sync.Mutex
	uploads map[string]*upload // keyed by resource
}

// upload is the in-flight multipart upload for one resource: its accumulating part
// buffer, the parts already sent, and the first async upload failure.
type upload struct {
	mu       sync.Mutex
	key      string
	id       string
	buf      []byte
	parts    []types.CompletedPart
	partNum  int32
	inflight sync.WaitGroup
	err      error // first part-upload failure; poisons subsequent Writes and Commit
}

// New returns an unconfigured sink. Open wires it to S3.
func New() *Sink { return &Sink{uploads: map[string]*upload{}} }

var _ ingestion.Sink = (*Sink)(nil)

func (s *Sink) Spec() ingestion.SinkSpec {
	return ingestion.SinkSpec{
		Name:        "s3",
		DisplayName: "Amazon S3 (NDJSON per resource)",
		Version:     "1",
		Config: ingestion.ConfigSchema{Fields: []ingestion.ConfigField{
			{Name: "bucket", Type: ingestion.FieldString, Required: true, Scope: ingestion.ScopeConnection, Help: "Destination S3 bucket."},
			{Name: "prefix", Type: ingestion.FieldString, Scope: ingestion.ScopePipeline, Help: "Key prefix; objects land at <prefix>/<run>/<resource>.ndjson."},
			{Name: "region", Type: ingestion.FieldString, Scope: ingestion.ScopeConnection, Help: "AWS region; defaults to the SDK's resolved region."},
			{Name: "endpoint", Type: ingestion.FieldString, Scope: ingestion.ScopeConnection, Help: "Custom S3 endpoint (e.g. MinIO); defaults to AWS."},
			{Name: "access_key_id", Type: ingestion.FieldSecret, Scope: ingestion.ScopeConnection, Help: "Static access key; omit to use the SDK credential chain."},
			{Name: "secret_access_key", Type: ingestion.FieldSecret, Scope: ingestion.ScopeConnection, Help: "Static secret key; omit to use the SDK credential chain."},
			{Name: "part_size_mib", Type: ingestion.FieldInt, Scope: ingestion.ScopePipeline, Help: "Multipart part size in MiB; min 5, default 16."},
			{Name: "upload_concurrency", Type: ingestion.FieldInt, Scope: ingestion.ScopePipeline, Help: "Max in-flight part uploads across all resources; default 8."},
		}},
		Capabilities: ingestion.SinkCapabilities{WritePolicies: ingestion.WriteCapabilities(
			ingestion.IngestionAppend,
			ingestion.IngestionSnapshotReplace,
		)},
	}
}

func (s *Sink) Name() string { return "s3" }

// Open reads the sink config, builds an S3 client, and resets per-run state.
func (s *Sink) Open(ctx context.Context, run ingestion.RunSpec) error {
	cfg := ingestion.NewConfig(run.Sink.Config)
	s.bucket = cfg.String("bucket")
	if s.bucket == "" {
		return fmt.Errorf("s3 sink: bucket is required")
	}
	// Trim surrounding slashes so a configured prefix like "ingestion/runs/" does not
	// produce a "//" segment in the key (S3 treats it as a real empty path segment,
	// landing the object somewhere other than where it's looked for).
	s.prefix = strings.Trim(cfg.String("prefix"), "/")
	s.run = run.Run
	s.uploads = map[string]*upload{}

	s.partSize = cfg.Int("part_size_mib") << 20
	if s.partSize < minPartSize {
		s.partSize = defaultPartSize
	}
	conc := cfg.Int("upload_concurrency")
	if conc < 1 {
		conc = defaultConcurrency
	}
	s.sem = make(chan struct{}, conc)
	s.bufPool = sync.Pool{New: func() any {
		b := make([]byte, 0, s.partSize+s.partSize/8) // slack so a part can overshoot without growing
		return &b
	}}
	s.scratchPool = sync.Pool{New: func() any {
		b := make([]byte, 0, 64<<10) // grows to the largest batch encoded; reused thereafter
		return &b
	}}

	var loadOpts []func(*awscfg.LoadOptions) error
	if region := cfg.String("region"); region != "" {
		loadOpts = append(loadOpts, awscfg.WithRegion(region))
	}
	if id, secret := cfg.Secret("access_key_id"), cfg.Secret("secret_access_key"); id != "" && secret != "" {
		loadOpts = append(loadOpts, awscfg.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(id, secret, ""),
		))
	}
	awsCfg, err := awscfg.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return fmt.Errorf("s3 sink: load aws config: %w", err)
	}

	endpoint := cfg.String("endpoint")
	s.client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true // MinIO and most non-AWS gateways need path-style addressing.
		}
	})
	return nil
}

// Write appends each record as one NDJSON line to its resource's part buffer. A
// full buffer is handed to a background goroutine to upload, so Write never waits
// on the network. Safe for concurrent calls across resources and across parts of
// one resource.
func (s *Sink) Write(ctx context.Context, b ingestion.Batch) (ingestion.WriteReceipt, error) {
	if s.client == nil {
		return ingestion.WriteReceipt{}, fmt.Errorf("s3 sink: write before open")
	}
	u, err := s.uploadFor(ctx, b.Resource)
	if err != nil {
		return ingestion.WriteReceipt{}, err
	}

	// Encode outside u.mu: JSON serialization is pure CPU and would otherwise
	// serialize every writer targeting the same resource. The lock window below is
	// just a memcpy plus the threshold check.
	scratch := (*s.scratchPool.Get().(*[]byte))[:0]
	for i := range b.Records {
		line, err := encodeRecord(b.Records[i])
		if err != nil {
			s.scratchPool.Put(&scratch)
			return ingestion.WriteReceipt{}, fmt.Errorf("s3 sink: encode %s: %w", b.Resource, err)
		}
		scratch = append(scratch, line...)
		scratch = append(scratch, '\n')
	}
	nbytes := int64(len(scratch))

	u.mu.Lock()
	if u.err != nil {
		err := u.err
		u.mu.Unlock()
		s.scratchPool.Put(&scratch)
		return ingestion.WriteReceipt{}, fmt.Errorf("s3 sink: upload part %s: %w", b.Resource, err)
	}
	u.buf = append(u.buf, scratch...)
	var body []byte
	var num int32
	if len(u.buf) >= s.partSize {
		body, num = s.swapBuffer(u)
	}
	u.mu.Unlock()
	s.scratchPool.Put(&scratch)

	// Acquire the in-flight slot outside u.mu (taking it under the lock would
	// deadlock against the upload goroutines, which need u.mu to record their part).
	// Blocking here is the point: it backpressures the pipeline when the cap is hit.
	if body != nil {
		s.startPartUpload(ctx, u, body, num)
	}

	crc, _ := ingestion.CRC32C(b.Records)
	return ingestion.WriteReceipt{
		URI:      fmt.Sprintf("s3://%s/%s", s.bucket, u.key),
		Bytes:    nbytes,
		Rows:     len(b.Records),
		WriteCRC: crc,
	}, nil
}

func (s *Sink) Apply(ctx context.Context, b ingestion.Batch, opts ingestion.ApplyOptions) (ingestion.WriteReceipt, error) {
	switch opts.Policy.Capability.Mode {
	case ingestion.WriteAppend, ingestion.WriteReplace:
		if err := opts.Policy.ValidateRecords(b.Resource, b.Records); err != nil {
			return ingestion.WriteReceipt{}, fmt.Errorf("s3 sink: %w", err)
		}
		return s.Write(ctx, b)
	default:
		return ingestion.WriteReceipt{}, fmt.Errorf("s3 sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

// swapBuffer hands off the full part buffer and assigns its part number, leaving a
// fresh pooled buffer in its place. Caller holds u.mu.
func (s *Sink) swapBuffer(u *upload) (body []byte, num int32) {
	u.partNum++
	num = u.partNum
	body = u.buf
	u.buf = (*s.bufPool.Get().(*[]byte))[:0]
	return body, num
}

// startPartUpload uploads a handed-off part buffer in the background. The semaphore
// is acquired here, off u.mu, so a saturated cap backpressures the calling Write
// rather than spawning unbounded goroutines and buffers.
func (s *Sink) startPartUpload(ctx context.Context, u *upload, body []byte, num int32) {
	s.sem <- struct{}{}
	u.inflight.Go(func() {
		defer func() { <-s.sem }()

		t0 := time.Now()
		out, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(s.bucket),
			Key:        aws.String(u.key),
			UploadId:   aws.String(u.id),
			PartNumber: aws.Int32(num),
			Body:       bytes.NewReader(body),
		})
		s3debugf("UploadPart key=%s part=%d bytes=%d dur=%s err=%v", u.key, num, len(body), time.Since(t0), err)

		u.mu.Lock()
		if err != nil {
			if u.err == nil {
				u.err = err
			}
		} else {
			u.parts = append(u.parts, types.CompletedPart{ETag: out.ETag, PartNumber: aws.Int32(num)})
		}
		u.mu.Unlock()
		s.bufPool.Put(&body)
	})
}

// uploadFor returns the resource's upload, starting its multipart upload on first use.
func (s *Sink) uploadFor(ctx context.Context, resource string) (*upload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.uploads[resource]; ok {
		return u, nil
	}
	key := s.objectKey(resource)
	t0 := time.Now()
	out, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String("application/x-ndjson"),
	})
	s3debugf("CreateMultipartUpload key=%s dur=%s err=%v", key, time.Since(t0), err)
	if err != nil {
		return nil, fmt.Errorf("s3 sink: create upload %s: %w", resource, err)
	}
	u := &upload{key: key, id: aws.ToString(out.UploadId), buf: (*s.bufPool.Get().(*[]byte))[:0]}
	s.uploads[resource] = u
	return u, nil
}

// objectKey builds <prefix>/<run>/<resource>.ndjson, trimming an empty prefix.
func (s *Sink) objectKey(resource string) string {
	if s.prefix == "" {
		return fmt.Sprintf("%s/%s.ndjson", s.run, resource)
	}
	return fmt.Sprintf("%s/%s/%s.ndjson", s.prefix, s.run, resource)
}

// Commit drains each resource's in-flight parts, flushes its tail, and completes
// its multipart upload — resources in parallel — making every object durable. A
// resource that produced no records is completed as an empty object via a single
// PutObject (S3 forbids a zero-part multipart upload).
func (s *Sink) Commit(ctx context.Context) error {
	uploads := s.snapshot()

	errs := make([]error, len(uploads))
	var wg sync.WaitGroup
	for i, u := range uploads {
		wg.Go(func() {
			u.inflight.Wait()
			// Gate completion through the same cap: without it, one run with many
			// resources fires an unbounded burst of tail/Complete requests at S3,
			// ignoring upload_concurrency. Acquire after the Wait so no slot is held idle.
			s.sem <- struct{}{}
			defer func() { <-s.sem }()
			u.mu.Lock()
			err := u.complete(ctx, s.client, s.bucket)
			u.mu.Unlock()
			if err != nil {
				errs[i] = fmt.Errorf("s3 sink: commit %s: %w", u.key, err)
			}
		})
	}
	wg.Wait()
	// Keys are sorted, so the reported (first) error is deterministic.
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// snapshot returns the current uploads sorted by key for deterministic iteration.
func (s *Sink) snapshot() []*upload {
	s.mu.Lock()
	uploads := make([]*upload, 0, len(s.uploads))
	for _, u := range s.uploads {
		uploads = append(uploads, u)
	}
	s.mu.Unlock()
	sort.Slice(uploads, func(i, j int) bool { return uploads[i].key < uploads[j].key })
	return uploads
}

// complete uploads the tail part and finishes the multipart upload. Caller holds
// u.mu and has waited out in-flight parts.
func (u *upload) complete(ctx context.Context, client *s3.Client, bucket string) error {
	if u.err != nil {
		return u.err
	}
	if len(u.buf) > 0 {
		u.partNum++
		t0 := time.Now()
		out, err := client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(bucket),
			Key:        aws.String(u.key),
			UploadId:   aws.String(u.id),
			PartNumber: aws.Int32(u.partNum),
			Body:       bytes.NewReader(u.buf),
		})
		s3debugf("UploadPart(tail) key=%s part=%d bytes=%d dur=%s err=%v", u.key, u.partNum, len(u.buf), time.Since(t0), err)
		if err != nil {
			return err
		}
		u.parts = append(u.parts, types.CompletedPart{ETag: out.ETag, PartNumber: aws.Int32(u.partNum)})
		u.buf = u.buf[:0]
	}
	if len(u.parts) == 0 {
		// No part was ever uploaded: abandon the empty multipart upload and put a
		// zero-byte object so the resource's file still exists.
		_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket: aws.String(bucket), Key: aws.String(u.key), UploadId: aws.String(u.id),
		})
		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(u.key),
			ContentType: aws.String("application/x-ndjson"),
			Body:        bytes.NewReader(nil),
		})
		return err
	}
	// Async uploads complete out of order; S3 requires ascending part numbers.
	sort.Slice(u.parts, func(i, j int) bool {
		return aws.ToInt32(u.parts[i].PartNumber) < aws.ToInt32(u.parts[j].PartNumber)
	})
	t0 := time.Now()
	_, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(u.key),
		UploadId:        aws.String(u.id),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: u.parts},
	})
	s3debugf("CompleteMultipartUpload key=%s parts=%d dur=%s err=%v", u.key, len(u.parts), time.Since(t0), err)
	return err
}

// Abort cancels every in-flight multipart upload so a failed run leaves no partial
// object. Cleanup runs on a cancel-free context so it completes even when the
// triggering failure cancelled ctx.
func (s *Sink) Abort(ctx context.Context) error {
	ctx = context.WithoutCancel(ctx)
	s.mu.Lock()
	uploads := make([]*upload, 0, len(s.uploads))
	for _, u := range s.uploads {
		uploads = append(uploads, u)
	}
	s.uploads = map[string]*upload{}
	s.mu.Unlock()

	var firstErr error
	for _, u := range uploads {
		if s.client == nil {
			break
		}
		u.inflight.Wait() // don't abort under an in-flight part; S3 rejects parts of an aborted upload
		_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket: aws.String(s.bucket), Key: aws.String(u.key), UploadId: aws.String(u.id),
		})
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("s3 sink: abort %s: %w", u.key, err)
		}
	}
	return firstErr
}

type recordLine struct {
	ID   string          `json:"id"`
	Op   int             `json:"op"`
	Data json.RawMessage `json:"data"`
}

func encodeRecord(r ingestion.Record) ([]byte, error) {
	data := json.RawMessage(r.Data)
	if !json.Valid(r.Data) {
		s, err := json.Marshal(string(r.Data))
		if err != nil {
			return nil, err
		}
		data = s
	}
	return json.Marshal(recordLine{ID: r.ID, Op: int(r.Op), Data: data})
}
