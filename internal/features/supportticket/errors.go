package supportticket

import "errors"

var (
	ErrTicketNotFound    = errors.New("ticket not found")
	ErrSubjectRequired   = errors.New("subject is required")
	ErrMessageRequired   = errors.New("message is required")
	ErrReplyInfoRequired = errors.New("reply information is required")
)
