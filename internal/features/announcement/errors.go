package announcement

import "errors"

var (
	ErrAnnouncementNotFound = errors.New("announcement not found")
	ErrTitleRequired        = errors.New("announcement title is required")
)
