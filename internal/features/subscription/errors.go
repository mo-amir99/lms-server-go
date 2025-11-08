package subscription

import (
	"errors"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

var (
	ErrUserNotFound         = errors.New("user not found to associate with subscription")
	ErrUserHasSubscription  = errors.New("user already has an active subscription")
	ErrSubscriptionTaken    = errors.New("user already has a subscription or identifier is taken")
	ErrPackageNotFound      = errors.New("subscription package not found")
	ErrSubscriptionNotFound = errors.New("subscription not found")
)

var (
	defaultSubscriptionPoints     = 0
	defaultSubscriptionPointPrice = types.NewMoney(0)
	defaultCourseLimitInGB        = 25
	defaultCoursesLimit           = 5
	defaultAssistantsLimit        = 5
	defaultWatchLimit             = 2
	defaultWatchInterval          = 240
)
