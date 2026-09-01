package app

import "context"

// ValidateConfiguration validates the selected target's structured document.
func (s *Service) ValidateConfiguration(ctx context.Context) error {
	document, err := s.Configuration(ctx)
	if err != nil {
		return err
	}
	return s.target.ValidateConfiguration(ctx, document)
}
