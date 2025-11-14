package lesson

import "errors"

var (
	ErrLessonNotFound     = errors.New("lesson not found")
	ErrNameRequired       = errors.New("lesson name is required")
	ErrNameLength         = errors.New("lesson name must be between 3 and 80 characters")
	ErrVideoIDRequired    = errors.New("video ID is required")
	ErrCourseNotFound     = errors.New("course not found")
	ErrDescriptionTooLong = errors.New("lesson description cannot exceed 1000 characters")
	ErrOrderInvalid       = errors.New("lesson order cannot be negative")
	ErrDurationInvalid    = errors.New("lesson duration cannot be negative")
	ErrVideoMismatch      = errors.New("video not found for this lesson")
	ErrWatchLimitReached  = errors.New("watch limit reached for this lesson")
	ErrJobIDRequired      = errors.New("job id is required")
)
