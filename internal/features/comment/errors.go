package comment

import "errors"

var (
	ErrCommentNotFound = errors.New("comment not found")
	ErrContentRequired = errors.New("comment content is required")
	ErrUnauthorized    = errors.New("not authorized to perform this action")
)
