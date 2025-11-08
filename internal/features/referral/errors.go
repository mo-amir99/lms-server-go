package referral

import "errors"

var (
	ErrReferralNotFound     = errors.New("referral not found")
	ErrReferralExists       = errors.New("referral already exists for this user")
	ErrReferrerRequired     = errors.New("referrer is required")
	ErrReferrerNotFound     = errors.New("referrer user not found")
	ErrInvalidReferrerType  = errors.New("selected user is not a referrer")
	ErrReferredUserNotFound = errors.New("referred user not found")
	ErrUnauthorized         = errors.New("unauthorized to create referral for another referrer")
)
