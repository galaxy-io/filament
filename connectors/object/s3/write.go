package s3

import (
	"context"
	"fmt"
	"sort"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	object "github.com/galaxy-io/filament/connectors/object/internal"
)

// Write encodes a batch and appends it to the resource's current object, then
// rolls to a new object if this one reached the size target.
func (s *Sink) Write(ctx context.Context, batch *arrowbatch.Batch) (filament.WriteReceipt, error) {
	if batch.Resource == "" {
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: resource is required")
	}
	session, bucket, layout, err := s.beginApply()
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
	resourceEncoder, err := s.encoderFor(batch.Resource, rows.Schema())
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	resourceEncoder.mu.Lock()
	defer resourceEncoder.mu.Unlock()
	key, err := layout.Object(batch.Resource, resourceEncoder.file)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: %w", err)
	}
	scratch := session.takeEncodedBuffer()
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
	resourceEncoder.fileBytes += int64(len(encoded))
	receipt := filament.WriteReceipt{
		URI:        fmt.Sprintf("s3://%s/%s", bucket, key),
		Bytes:      int64(len(encoded)),
		Rows:       batch.NumRows(),
		WriteCRC:   batch.IntegrityCRC(),
		EncodedCRC: &encodedCRC,
	}
	if resourceEncoder.fileBytes >= s.maxFileSize {
		if err := s.rollFile(ctx, session, layout, batch.Resource, resourceEncoder); err != nil {
			return filament.WriteReceipt{}, err
		}
	}
	return receipt, nil
}

// rollFile closes the resource's current object and opens the next numbered
// one. The caller holds resourceEncoder.mu. The closed object gets its format
// trailer (gzip trailer, JSON array close, Parquet footer) so it is a complete
// file on its own, and its tail upload starts now; it is completed with every
// other object at Commit. A new encoder starts the next object's stream.
func (s *Sink) rollFile(ctx context.Context, session *multipartSession, layout object.Layout, resource string, resourceEncoder *resourceEncoder) error {
	key, err := layout.Object(resource, resourceEncoder.file)
	if err != nil {
		return fmt.Errorf("s3 sink: roll %s: %w", resource, err)
	}
	if err := s.appendTrailer(ctx, session, resource, key, resourceEncoder); err != nil {
		return fmt.Errorf("s3 sink: roll %s: %w", resource, err)
	}
	if err := session.Seal(ctx, key); err != nil {
		return fmt.Errorf("s3 sink: roll %s: %w", resource, err)
	}
	next, err := object.NewEncoder(s.format, s.compression, resourceEncoder.schema)
	if err != nil {
		return fmt.Errorf("s3 sink: roll %s: %w", resource, err)
	}
	resourceEncoder.encoder = next
	resourceEncoder.file++
	resourceEncoder.fileBytes = 0
	resourceEncoder.hasRows = false
	return nil
}

// appendTrailer finalizes the encoder's stream into key. The caller holds
// resourceEncoder.mu.
func (s *Sink) appendTrailer(ctx context.Context, session *multipartSession, resource, key string, resourceEncoder *resourceEncoder) error {
	buffer := session.takeEncodedBuffer()
	encoded, crc, err := resourceEncoder.encoder.Finalize(buffer)
	if err != nil {
		session.releaseEncodedBuffer(buffer)
		return err
	}
	defer session.releaseEncodedBuffer(encoded)
	if len(encoded) == 0 {
		return nil
	}
	return session.Append(ctx, resource, key, encoded, 0, crc)
}

func (s *Sink) beginApply() (*multipartSession, string, object.Layout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateOpen || s.session == nil {
		return nil, "", object.Layout{}, fmt.Errorf("s3 sink: write requires an open run")
	}
	s.applyWG.Add(1)
	return s.session, s.bucket, s.layout, nil
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

// finalizeEncoders appends stream trailers to each resource's open object
// before multipart completion. Parquet writes its file footer here; gzip
// writes its trailer; NDJSON emits nothing. An object opened by a roll that
// received no rows is skipped, so a roll never leaves an empty trailing file.
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
		if err := s.finalizeEncoder(ctx, session, layout, resource, encoders[resource]); err != nil {
			return fmt.Errorf("s3 sink: finalize %s: %w", resource, err)
		}
	}
	return nil
}

func (s *Sink) finalizeEncoder(ctx context.Context, session *multipartSession, layout object.Layout, resource string, resourceEncoder *resourceEncoder) error {
	resourceEncoder.mu.Lock()
	defer resourceEncoder.mu.Unlock()
	if !resourceEncoder.hasRows {
		// Still finalize so the encoder releases its writer state.
		buffer := session.takeEncodedBuffer()
		encoded, _, err := resourceEncoder.encoder.Finalize(buffer)
		if err != nil {
			session.releaseEncodedBuffer(buffer)
			return err
		}
		session.releaseEncodedBuffer(encoded)
		return nil
	}
	key, err := layout.Object(resource, resourceEncoder.file)
	if err != nil {
		return err
	}
	return s.appendTrailer(ctx, session, resource, key, resourceEncoder)
}
