package attachment

import (
	"errors"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

var (
	ErrAttachmentNotFound = errors.New("attachment not found")
	ErrNameRequired       = errors.New("attachment name is required")
	ErrTypeRequired       = errors.New("attachment type is required")
	ErrInvalidType        = errors.New("invalid attachment type")
)

// ValidTypes returns all valid attachment types.
func ValidTypes() []string {
	return []string{
		string(types.AttachmentTypePDF),
		string(types.AttachmentTypeMCQ),
		string(types.AttachmentTypeImage),
		string(types.AttachmentTypeAudio),
		string(types.AttachmentTypeLink),
	}
}
