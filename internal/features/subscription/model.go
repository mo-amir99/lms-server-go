package subscription

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Subscription mirrors the PostgreSQL subscriptions table.
type Subscription struct {
	types.BaseModel

	UserID                 uuid.UUID   `gorm:"type:uuid;not null;column:user_id;index" json:"userId"`
	DisplayName            *string     `gorm:"type:varchar(50);column:display_name" json:"displayName,omitempty"`
	IdentifierName         string      `gorm:"type:varchar(20);not null;uniqueIndex;column:identifier_name" json:"identifierName"`
	SubscriptionPoints     int         `gorm:"type:int;not null;default:0;column:subscription_points" json:"SubscriptionPoints"`
	SubscriptionPointPrice types.Money `gorm:"type:numeric(10,2);not null;default:0;column:subscription_point_price" json:"SubscriptionPointPrice"`
	CourseLimitInGB        float64     `gorm:"type:numeric(10,2);not null;default:25;column:course_limit_in_gb" json:"CourseLimitInGB"`
	CoursesLimit           int         `gorm:"type:int;not null;default:5;column:courses_limit" json:"CoursesLimit"`
	PackageID              *uuid.UUID  `gorm:"type:uuid;column:package_id" json:"packageId,omitempty"`
	AssistantsLimit        int         `gorm:"type:int;not null;default:5;column:assistants_limit" json:"assistantsLimit"`
	WatchLimit             int         `gorm:"type:int;not null;default:2;column:watch_limit" json:"watchLimit"`
	WatchInterval          int         `gorm:"type:int;not null;default:240;column:watch_interval" json:"watchInterval"`
	SubscriptionEnd        time.Time   `gorm:"type:timestamp;not null;default:now();column:subscription_end;index;index:idx_active_end,priority:2" json:"subscriptionEnd"`
	RequireSameDeviceID    bool        `gorm:"type:boolean;not null;default:false;column:is_require_same_device_id" json:"isRequireSameDeviceId"`
	Active                 bool        `gorm:"type:boolean;not null;default:true;column:is_active;index:idx_active_end,priority:1" json:"isActive"`
}

// TableName overrides the default table name.
func (Subscription) TableName() string { return "subscriptions" }

// IsExpired reports whether the subscription has passed its end time.
func (s Subscription) IsExpired(now time.Time) bool { return now.After(s.SubscriptionEnd) }

// CreateInput carries the data needed for a new subscription.
type CreateInput struct {
	UserID                 uuid.UUID
	DisplayName            *string
	IdentifierName         string
	SubscriptionPoints     *int
	SubscriptionPointPrice *types.Money
	CourseLimitInGB        *float64
	CoursesLimit           *int
	AssistantsLimit        *int
	WatchLimit             *int
	WatchInterval          *int
	SubscriptionEnd        *time.Time
	RequireSameDeviceID    *bool
	Active                 *bool
}

// CreateFromPackageInput extends CreateInput with a package reference.
type CreateFromPackageInput struct {
	CreateInput
	PackageID uuid.UUID
}

// UpdateInput captures mutable subscription fields.
type UpdateInput struct {
	UserProvided bool
	UserID       *uuid.UUID

	DisplayNameProvided bool
	DisplayName         *string

	SubscriptionPoints     *int
	SubscriptionPointPrice *types.Money
	CourseLimitInGB        *float64
	CoursesLimit           *int
	AssistantsLimit        *int
	WatchLimit             *int
	WatchInterval          *int
	SubscriptionEnd        *time.Time
	RequireSameDeviceID    *bool
	Active                 *bool
}

// List queries subscriptions with optional keyword filtering.
func List(db *gorm.DB, params pagination.Params, keyword string) ([]Subscription, int64, error) {
	query := db.Model(&Subscription{})
	if trimmed := strings.TrimSpace(strings.ToLower(keyword)); trimmed != "" {
		like := "%" + trimmed + "%"
		query = query.Where("LOWER(display_name) LIKE ? OR LOWER(identifier_name) LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []Subscription
	if err := query.Order("created_at DESC").Offset(params.Skip).Limit(params.Limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Get retrieves a subscription by ID.
func Get(db *gorm.DB, id uuid.UUID) (Subscription, error) {
	return fetchSubscription(db, id)
}

// Create inserts a new subscription and links it to a user.
func Create(db *gorm.DB, input CreateInput) (Subscription, error) {
	sub := newSubscriptionFromInput(input)

	err := db.Transaction(func(tx *gorm.DB) error {
		user, err := fetchUser(tx, input.UserID)
		if err != nil {
			return err
		}
		if user.SubscriptionID != nil {
			return ErrUserHasSubscription
		}

		exists, err := subscriptionExists(tx, input.UserID, sub.IdentifierName, uuid.Nil)
		if err != nil {
			return err
		}
		if exists {
			return ErrSubscriptionTaken
		}

		if err := tx.Create(&sub).Error; err != nil {
			return err
		}

		return setUserSubscription(tx, input.UserID, &sub.ID)
	})

	return sub, err
}

// CreateFromPackage seeds a subscription using package defaults.
func CreateFromPackage(db *gorm.DB, input CreateFromPackageInput) (Subscription, error) {
	sub := newSubscriptionFromInput(input.CreateInput)

	err := db.Transaction(func(tx *gorm.DB) error {
		user, err := fetchUser(tx, input.UserID)
		if err != nil {
			return err
		}
		if user.SubscriptionID != nil {
			return ErrUserHasSubscription
		}

		exists, err := subscriptionExists(tx, input.UserID, sub.IdentifierName, uuid.Nil)
		if err != nil {
			return err
		}
		if exists {
			return ErrSubscriptionTaken
		}

		pkg, err := fetchPackage(tx, input.PackageID)
		if err != nil {
			return err
		}

		applyPackage(&sub, pkg)
		sub.PackageID = &pkg.ID

		if err := tx.Create(&sub).Error; err != nil {
			return err
		}

		return setUserSubscription(tx, input.UserID, &sub.ID)
	})

	return sub, err
}

// Update modifies a subscription and optionally reassigns its user.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (Subscription, error) {
	var updated Subscription

	err := db.Transaction(func(tx *gorm.DB) error {
		current, err := fetchSubscription(tx, id)
		if err != nil {
			return err
		}

		updates := map[string]interface{}{}
		userChange := false
		var newUserID uuid.UUID

		if input.UserProvided {
			if input.UserID == nil {
				return ErrUserNotFound
			}
			if current.UserID != *input.UserID {
				user, err := fetchUser(tx, *input.UserID)
				if err != nil {
					return err
				}
				if user.SubscriptionID != nil {
					return ErrUserHasSubscription
				}

				exists, err := subscriptionExists(tx, *input.UserID, current.IdentifierName, current.ID)
				if err != nil {
					return err
				}
				if exists {
					return ErrSubscriptionTaken
				}

				newUserID = *input.UserID
				userChange = true
				updates["user_id"] = newUserID
			}
		}

		if input.DisplayNameProvided {
			if input.DisplayName == nil {
				updates["display_name"] = nil
			} else {
				updates["display_name"] = *input.DisplayName
			}
		}

		if input.SubscriptionPoints != nil {
			updates["subscription_points"] = *input.SubscriptionPoints
		}
		if input.SubscriptionPointPrice != nil {
			updates["subscription_point_price"] = *input.SubscriptionPointPrice
		}
		if input.CourseLimitInGB != nil {
			updates["course_limit_in_gb"] = *input.CourseLimitInGB
		}
		if input.CoursesLimit != nil {
			updates["courses_limit"] = *input.CoursesLimit
		}
		if input.AssistantsLimit != nil {
			updates["assistants_limit"] = *input.AssistantsLimit
		}
		if input.WatchLimit != nil {
			updates["watch_limit"] = *input.WatchLimit
		}
		if input.WatchInterval != nil {
			updates["watch_interval"] = *input.WatchInterval
		}
		if input.SubscriptionEnd != nil {
			updates["subscription_end"] = input.SubscriptionEnd.UTC()
		}
		if input.RequireSameDeviceID != nil {
			updates["is_require_same_device_id"] = *input.RequireSameDeviceID
		}
		if input.Active != nil {
			updates["is_active"] = *input.Active
		}

		if len(updates) > 0 {
			if err := updateSubscription(tx, current.ID, updates); err != nil {
				return err
			}
		}

		if userChange {
			if err := setUserSubscription(tx, current.UserID, nil); err != nil {
				return err
			}
			if err := setUserSubscription(tx, newUserID, &current.ID); err != nil {
				return err
			}
		}

		refreshed, err := fetchSubscription(tx, current.ID)
		if err != nil {
			return err
		}
		updated = refreshed
		return nil
	})

	return updated, err
}

// Delete removes a subscription and clears the user's association.
func Delete(db *gorm.DB, id uuid.UUID) error {
	return db.Transaction(func(tx *gorm.DB) error {
		sub, err := fetchSubscription(tx, id)
		if err != nil {
			return err
		}

		if err := setUserSubscription(tx, sub.UserID, nil); err != nil {
			return err
		}

		if err := tx.Delete(&Subscription{}, "id = ?", id).Error; err != nil {
			return err
		}

		return nil
	})
}

// Helpers --------------------------------------------------------------------

func newSubscriptionFromInput(input CreateInput) Subscription {
	now := time.Now().UTC()

	sub := Subscription{
		UserID:                 input.UserID,
		DisplayName:            input.DisplayName,
		IdentifierName:         input.IdentifierName,
		SubscriptionPoints:     defaultSubscriptionPoints,
		SubscriptionPointPrice: defaultSubscriptionPointPrice,
		CourseLimitInGB:        defaultCourseLimitInGB,
		CoursesLimit:           defaultCoursesLimit,
		AssistantsLimit:        defaultAssistantsLimit,
		WatchLimit:             defaultWatchLimit,
		WatchInterval:          defaultWatchInterval,
		SubscriptionEnd:        now,
		RequireSameDeviceID:    false,
		Active:                 true,
	}

	if input.SubscriptionPoints != nil {
		sub.SubscriptionPoints = *input.SubscriptionPoints
	}
	if input.SubscriptionPointPrice != nil {
		sub.SubscriptionPointPrice = *input.SubscriptionPointPrice
	}
	if input.CourseLimitInGB != nil {
		sub.CourseLimitInGB = *input.CourseLimitInGB
	}
	if input.CoursesLimit != nil {
		sub.CoursesLimit = *input.CoursesLimit
	}
	if input.AssistantsLimit != nil {
		sub.AssistantsLimit = *input.AssistantsLimit
	}
	if input.WatchLimit != nil {
		sub.WatchLimit = *input.WatchLimit
	}
	if input.WatchInterval != nil {
		sub.WatchInterval = *input.WatchInterval
	}
	if input.SubscriptionEnd != nil {
		sub.SubscriptionEnd = input.SubscriptionEnd.UTC()
	}
	if input.RequireSameDeviceID != nil {
		sub.RequireSameDeviceID = *input.RequireSameDeviceID
	}
	if input.Active != nil {
		sub.Active = *input.Active
	}

	return sub
}

func applyPackage(sub *Subscription, pkg subscriptionPackageRow) {
	if pkg.SubscriptionPointPrice != nil {
		sub.SubscriptionPointPrice = *pkg.SubscriptionPointPrice
	}
	if pkg.CourseLimitInGB != nil {
		sub.CourseLimitInGB = *pkg.CourseLimitInGB
	}
	if pkg.CoursesLimit != nil {
		sub.CoursesLimit = *pkg.CoursesLimit
	}
	if pkg.AssistantsLimit != nil {
		sub.AssistantsLimit = *pkg.AssistantsLimit
	}
	if pkg.WatchLimit != nil {
		sub.WatchLimit = *pkg.WatchLimit
	}
	if pkg.WatchInterval != nil {
		sub.WatchInterval = *pkg.WatchInterval
	}
}

func fetchSubscription(db *gorm.DB, id uuid.UUID) (Subscription, error) {
	var sub Subscription
	if err := db.First(&sub, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sub, ErrSubscriptionNotFound
		}
		return sub, err
	}
	return sub, nil
}

func fetchUser(db *gorm.DB, id uuid.UUID) (userRow, error) {
	var user userRow
	if err := db.Model(&userRow{}).Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user, ErrUserNotFound
		}
		return user, err
	}
	return user, nil
}

func fetchPackage(db *gorm.DB, id uuid.UUID) (subscriptionPackageRow, error) {
	var pkg subscriptionPackageRow
	if err := db.Model(&subscriptionPackageRow{}).Where("id = ?", id).First(&pkg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkg, ErrPackageNotFound
		}
		return pkg, err
	}
	return pkg, nil
}

func setUserSubscription(db *gorm.DB, userID uuid.UUID, subscriptionID *uuid.UUID) error {
	return db.Model(&userRow{}).Where("id = ?", userID).Update("subscription_id", subscriptionID).Error
}

func subscriptionExists(db *gorm.DB, userID uuid.UUID, identifier string, ignoreID uuid.UUID) (bool, error) {
	query := db.Model(&Subscription{}).Where("user_id = ? OR identifier_name = ?", userID, identifier)
	if ignoreID != uuid.Nil {
		query = query.Where("id <> ?", ignoreID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func updateSubscription(db *gorm.DB, id uuid.UUID, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	return db.Model(&Subscription{}).Where("id = ?", id).Updates(updates).Error
}

// Database helpers -----------------------------------------------------------

type userRow struct {
	ID             uuid.UUID  `gorm:"column:id"`
	SubscriptionID *uuid.UUID `gorm:"column:subscription_id"`
}

func (userRow) TableName() string { return "users" }

type subscriptionPackageRow struct {
	ID                     uuid.UUID    `gorm:"column:id"`
	SubscriptionPointPrice *types.Money `gorm:"column:subscription_point_price"`
	CourseLimitInGB        *float64     `gorm:"column:course_limit_in_gb"`
	CoursesLimit           *int         `gorm:"column:courses_limit"`
	AssistantsLimit        *int         `gorm:"column:assistants_limit"`
	WatchLimit             *int         `gorm:"column:watch_limit"`
	WatchInterval          *int         `gorm:"column:watch_interval"`
}

func (subscriptionPackageRow) TableName() string { return "subscription_packages" }
