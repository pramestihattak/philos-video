package upload

import "context"

func (s *Service) GetProgress(ctx context.Context, uploadID string) (received, total int, err error) {
	return s.uploads.GetProgress(ctx, uploadID)
}
