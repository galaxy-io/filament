package app

import "context"

// ValidateConfiguration validates the selected target's structured document.
func (s *Service) ValidateConfiguration(ctx context.Context) error {
	document, err := s.target.Configuration(ctx)
	if err != nil {
		return err
	}
	catalog, err := s.target.Catalog(ctx)
	if err != nil {
		return err
	}
	return ValidateDocument(document, catalog)
}
