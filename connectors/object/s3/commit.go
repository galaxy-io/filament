package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

const manifestContentType = "application/json"

type successManifest struct {
	Version   int                `json:"version"`
	Run       string             `json:"run"`
	Resources []manifestResource `json:"resources"`
}

type manifestResource struct {
	Name   string `json:"name"`
	Key    string `json:"key"`
	URI    string `json:"uri"`
	Rows   int64  `json:"rows"`
	Bytes  int64  `json:"bytes"`
	CRC32C string `json:"crc32c"`
}

// Commit completes every resource object and publishes _SUCCESS.json last.
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
	manifest := successManifest{Version: 1, Run: string(layout.run), Resources: make([]manifestResource, len(results))}
	for i, result := range results {
		manifest.Resources[i] = manifestResource{
			Name: result.resource, Key: result.key, URI: fmt.Sprintf("s3://%s/%s", bucket, result.key),
			Rows: result.rows, Bytes: result.bytes, CRC32C: fmt.Sprintf("%08x", result.crc32c),
		}
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("s3 sink: encode success manifest: %w", err)
	}
	if err := session.PutObject(ctx, layout.success(), objectMetadata{contentType: manifestContentType}, bytes.NewReader(body), int64(len(body))); err != nil {
		return fmt.Errorf("s3 sink: publish success marker: %w", err)
	}

	s.mu.Lock()
	s.state = stateCommitted
	s.mu.Unlock()
	session.Cancel()
	return nil
}

func (s *Sink) beginCommit() (*multipartSession, string, keyLayout, error) {
	s.mu.Lock()
	if s.state != stateOpen || s.session == nil {
		s.mu.Unlock()
		return nil, "", keyLayout{}, fmt.Errorf("s3 sink: commit requires an open run")
	}
	s.state = stateCommitting
	session, bucket, layout := s.session, s.bucket, s.layout
	s.mu.Unlock()

	s.applyWG.Wait()
	return session, bucket, layout, nil
}

// Abort cancels in-flight requests and abandons every incomplete multipart upload.
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
