package comment

import "context"

func (s *Service) DeleteComment(ctx context.Context, commentID, userID string) error {
	return s.comments.Delete(ctx, commentID, userID)
}
