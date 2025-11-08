package course

import "errors"

var (
	ErrCourseNotFound = errors.New("course not found")
	ErrNameRequired   = errors.New("course name is required")
	ErrOrderTaken     = errors.New("course order already exists for this subscription")
)
