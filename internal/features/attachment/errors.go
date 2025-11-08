package attachment

import "errors"

var (
	ErrAttachmentNotFound = errors.New("attachment not found")
	ErrNameRequired       = errors.New("attachment name is required")
	ErrTypeRequired       = errors.New("attachment type is required")
	ErrInvalidType        = errors.New("invalid attachment type")
)

// AttachmentType constants
const (
	TypePDF   = "pdf"
	TypeMCQ   = "mcq"
	TypeImage = "image"
	TypeAudio = "audio"
	TypeLink  = "link"
)

// ValidTypes returns all valid attachment types.
func ValidTypes() []string {
	return []string{TypePDF, TypeMCQ, TypeImage, TypeAudio, TypeLink}
}
