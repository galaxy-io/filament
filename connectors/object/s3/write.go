package s3

import (
	"context"
	"fmt"
	"sort"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

// Write encodes a batch and appends it to the resource's multipart stream.
func (s *Sink) Write(ctx context.Context, batch *arrowbatch.Batch) (filament.WriteReceipt, error) {
	if batch.Resource == "" {
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: resource is required")
	}
	session, bucket, key, err := s.beginApply(batch.Resource)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	defer s.applyWG.Done()

	rows := batch.Rows()
	if batch.NumRows() == 0 {
		encodedCRC := uint32(0)
		return filament.WriteReceipt{
			WriteCRC:   batch.IntegrityCRC(),
			EncodedCRC: &encodedCRC,
		}, nil
	}
	scratch := session.takeEncodedBuffer()
	resourceEncoder, err := s.encoderFor(batch.Resource, rows.Schema())
	if err != nil {
		session.releaseEncodedBuffer(scratch)
		return filament.WriteReceipt{}, err
	}
	resourceEncoder.mu.Lock()
	defer resourceEncoder.mu.Unlock()
	encoded, encodedCRC, err := resourceEncoder.encoder.EncodeBatch(scratch, rows)
	if err != nil {
		session.releaseEncodedBuffer(scratch)
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: encode %s: %w", batch.Resource, err)
	}
	defer session.releaseEncodedBuffer(encoded)
	if err := session.Append(ctx, batch.Resource, key, encoded, batch.NumRows(), encodedCRC); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: stream %s: %w", batch.Resource, err)
	}
	resourceEncoder.hasRows = true
	return filament.WriteReceipt{
		URI:        fmt.Sprintf("s3://%s/%s", bucket, key),
		Bytes:      int64(len(encoded)),
		Rows:       batch.NumRows(),
		WriteCRC:   batch.IntegrityCRC(),
		EncodedCRC: &encodedCRC,
	}, nil
}

func (s *Sink) beginApply(resource string) (*multipartSession, string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateOpen || s.session == nil {
		return nil, "", "", fmt.Errorf("s3 sink: write requires an open run")
	}
	key, err := s.layout.object(resource)
	if err != nil {
		return nil, "", "", err
	}
	s.applyWG.Add(1)
	return s.session, s.bucket, key, nil
}

// Apply validates the batch against the run's write policy, then streams it.
func (s *Sink) Apply(ctx context.Context, batch *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	switch opts.Policy.Capability.Mode {
	case filament.WriteAppend, filament.WriteReplace:
		if err := opts.Policy.ValidateBatch(batch.Resource, batch); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("s3 sink: %w", err)
		}
		return s.Write(ctx, batch)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

// finalizeEncoders appends stream trailers before multipart completion. Parquet
// writes its file footer here; gzip writes its trailer; NDJSON emits nothing.
func (s *Sink) finalizeEncoders(ctx context.Context, session *multipartSession) error {
	s.mu.Lock()
	resources := make([]string, 0, len(s.enc))
	encoders := make(map[string]*resourceEncoder, len(s.enc))
	for resource, encoder := range s.enc {
		resources = append(resources, resource)
		encoders[resource] = encoder
	}
	layout := s.layout
	s.mu.Unlock()
	sort.Strings(resources)

	for _, resource := range resources {
		resourceEncoder := encoders[resource]
		resourceEncoder.mu.Lock()
		buffer := session.takeEncodedBuffer()
		encoded, crc, err := resourceEncoder.encoder.Finalize(buffer)
		if err == nil && len(encoded) > 0 && resourceEncoder.hasRows {
			var key string
			if key, err = layout.object(resource); err == nil {
				err = session.Append(ctx, resource, key, encoded, 0, crc)
			}
		}
		if err != nil {
			session.releaseEncodedBuffer(buffer)
			resourceEncoder.mu.Unlock()
			return fmt.Errorf("s3 sink: finalize %s: %w", resource, err)
		}
		session.releaseEncodedBuffer(encoded)
		resourceEncoder.mu.Unlock()
	}
	return nil
}
