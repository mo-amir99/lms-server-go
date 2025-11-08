package forum

import "errors"

var (
	ErrForumNotFound  = errors.New("forum not found")
	ErrTitleRequired  = errors.New("forum title is required")
	ErrTitleExists    = errors.New("a forum with this title already exists")
	ErrForbidden      = errors.New("access to this forum is forbidden")
	ErrAssistantsOnly = errors.New("only assistants can post in this forum")
)
