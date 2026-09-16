package gcs

import (
	"context"
	"fmt"

	object "github.com/galaxy-io/filament/connectors/object/internal"
)

func (s *Sink) beginCommit() (*uploadSession, string, object.Layout, error) {
	s.mu.Lock()
	if s.state != stateOpen || s.session == nil {
		s.mu.Unlock()
		return nil, "", object.Layout{}, fmt.Errorf("gcs sink: commit requires an open run")
	}
	s.state = stateCommitting
	session, bucket, layout := s.session, s.bucket, s.layout
	s.mu.Unlock()
	s.applyWG.Wait()
	return session, bucket, layout, nil
}

// Commit finalizes all resource objects and publishes _SUCCESS.json last.
func (s *Sink) Commit(ctx context.Context) error {
	session, bucket, layout, err := s.beginCommit()
	if err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, session.Cancel)
	defer stop()
	if err := s.finalizeEncoders(ctx, session); err != nil {
		return err
	}
	results, err := session.Complete(ctx)
	if err != nil {
		return err
	}
	body, err := object.EncodeManifest(layout.Run(), "gs", bucket, results)
	if err != nil {
		return fmt.Errorf("gcs sink: encode success manifest: %w", err)
	}
	if err := session.PutObject(ctx, layout.Success(), objectMetadata{contentType: object.ManifestContentType}, body); err != nil {
		return fmt.Errorf("gcs sink: publish success marker: %w", err)
	}
	s.mu.Lock()
	s.state = stateCommitted
	s.mu.Unlock()
	session.Cancel()
	_ = session.closeStore()
	return nil
}

// Abort cancels session-owned work. Unfinalized GCS objects remain invisible.
func (s *Sink) Abort(ctx context.Context) error {
	s.mu.Lock()
	if s.state == stateAborted || s.state == stateCommitted {
		s.mu.Unlock()
		return nil
	}
	s.state = stateAborted
	session := s.session
	s.mu.Unlock()
	if session == nil {
		return nil
	}
	session.Cancel()
	s.applyWG.Wait()
	return session.Abort(ctx)
}
