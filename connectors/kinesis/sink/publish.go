package sink

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/aws/smithy-go"
)

const (
	maxRequestRecords = 500
	maxRequestBytes   = 10 << 20
	maxBackoff        = 2 * time.Second
)

// publish sends requests in input order, one at a time.
func (s *Sink) publish(ctx context.Context, stream string, entries []types.PutRecordsRequestEntry) error {
	for len(entries) > 0 {
		n := requestLen(entries)
		if err := s.put(ctx, stream, entries[:n]); err != nil {
			return err
		}
		entries = entries[n:]
	}
	return nil
}

// requestLen returns how many leading entries fit one request.
func requestLen(entries []types.PutRecordsRequestEntry) int {
	var size int
	for i, e := range entries {
		size += len(e.Data) + len(*e.PartitionKey)
		if i == maxRequestRecords || (i > 0 && size > maxRequestBytes) {
			return i
		}
	}
	return len(entries)
}

// put resends only the records Kinesis rejected until the context ends. A
// resent record lands after later records already accepted on its shard.
func (s *Sink) put(ctx context.Context, stream string, pending []types.PutRecordsRequestEntry) error {
	backoff := retry.NewExponentialJitterBackoff(maxBackoff)
	for attempt := 0; ; attempt++ {
		out, err := s.client.PutRecords(ctx, &kinesis.PutRecordsInput{StreamName: aws.String(stream), Records: pending})
		if err != nil {
			// A request without a service response may still take effect.
			var rejected smithy.APIError
			if !errors.As(err, &rejected) {
				s.uncertain = err
			}
			return fmt.Errorf("kinesis sink: put records to %q: %w", stream, err)
		}
		if len(out.Records) != len(pending) {
			return fmt.Errorf("kinesis sink: %d results for %d records", len(out.Records), len(pending))
		}
		failed, code := pending[:0], ""
		for i, r := range out.Records {
			if r.ErrorCode != nil {
				failed, code = append(failed, pending[i]), *r.ErrorCode
			}
		}
		if len(failed) == 0 {
			return nil
		}
		pending = failed
		delay, err := backoff.BackoffDelay(attempt, nil)
		if err != nil {
			return err
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return fmt.Errorf("kinesis sink: %d records rejected by %q (%s): %w", len(pending), stream, code, ctx.Err())
		}
	}
}
