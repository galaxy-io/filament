package s3

import (
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
