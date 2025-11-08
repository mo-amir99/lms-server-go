package thread

import "errors"

var (
	ErrThreadNotFound   = errors.New("thread not found")
	ErrTitleRequired    = errors.New("thread title is required")
	ErrContentRequired  = errors.New("thread content is required")
	ErrUserNameRequired = errors.New("author name is required")
	ErrUnauthorized     = errors.New("unauthorized to modify this thread")
	ErrReplyNotFound    = errors.New("reply not found")
)
