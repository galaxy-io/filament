package s3

import (
	"fmt"

	object "github.com/galaxy-io/filament/connectors/object/internal"
)

func (s *Sink) beginCommit() (*multipartSession, string, object.Layout, error) {
	s.mu.Lock()
	if s.state != stateOpen || s.session == nil {
		s.mu.Unlock()
		return nil, "", object.Layout{}, fmt.Errorf("s3 sink: commit requires an open run")
	}
	s.state = stateCommitting
	session, bucket, layout := s.session, s.bucket, s.layout
	s.mu.Unlock()

	s.applyWG.Wait()
	return session, bucket, layout, nil
}
