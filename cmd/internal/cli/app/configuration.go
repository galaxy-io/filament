package app

import "context"

// ValidateConfiguration validates the selected target's structured document.
func (s *Service) ValidateConfiguration(ctx context.Context) error {
	if _, ok := s.target.(RawConfigurationTarget); !ok {
		return ErrRawConfigurationUnsupported
	}
	document, err := s.Configuration(ctx)
	if err != nil {
		return err
	}
	return s.target.ValidateConfiguration(ctx, document)
}
