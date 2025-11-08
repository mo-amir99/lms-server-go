package user

import (
	"errors"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already exists")
	ErrInvalidPassword    = errors.New("password must be at least 8 characters")
	ErrUnauthorized       = errors.New("unauthorized to perform this action")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// Re-export types from pkg/types for backward compatibility
const (
	UserTypeReferrer   = types.UserTypeReferrer
	UserTypeStudent    = types.UserTypeStudent
	UserTypeAssistant  = types.UserTypeAssistant
	UserTypeInstructor = types.UserTypeInstructor
	UserTypeTeacher    = types.UserTypeInstructor // legacy alias
	UserTypeAdmin      = types.UserTypeAdmin
	UserTypeOwner      = types.UserTypeAdmin // legacy alias
	UserTypeSuperAdmin = types.UserTypeSuperAdmin
	UserTypeAll        = types.UserTypeAll
)

var UserTypeOrder = []types.UserType{
	types.UserTypeReferrer,
	types.UserTypeStudent,
	types.UserTypeAssistant,
	types.UserTypeInstructor,
	types.UserTypeAdmin,
	types.UserTypeSuperAdmin,
}
