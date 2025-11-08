package pkg

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Package represents a subscription package.
type Package struct {
	types.BaseModel

	Name                   string       `gorm:"type:varchar(80);not null;uniqueIndex" json:"name"`
	Description            *string      `gorm:"type:varchar(1000)" json:"description,omitempty"`
	Price                  types.Money  `gorm:"type:numeric(10,2);not null" json:"price"`
	DiscountPercentage     float64      `gorm:"type:numeric(5,2);not null;default:0;column:discount_percentage" json:"discountPercentage"`
	Order                  int          `gorm:"type:int;not null;uniqueIndex" json:"order"`
	SubscriptionPoints     *int         `gorm:"type:int;column:subscription_points" json:"subscriptionPoints,omitempty"`
	SubscriptionPointPrice *types.Money `gorm:"type:numeric(10,2);column:subscription_point_price" json:"subscriptionPointPrice,omitempty"`
	CoursesLimit           *int         `gorm:"type:int;column:courses_limit" json:"coursesLimit,omitempty"`
	CourseLimitInGB        *int         `gorm:"type:int;column:course_limit_in_gb" json:"courseLimitInGB,omitempty"`
	AssistantsLimit        *int         `gorm:"type:int;column:assistants_limit" json:"assistantsLimit,omitempty"`
	WatchLimit             *int         `gorm:"type:int;column:watch_limit" json:"watchLimit,omitempty"`
	WatchInterval          *int         `gorm:"type:int;column:watch_interval" json:"watchInterval,omitempty"`
	Active                 bool         `gorm:"type:boolean;not null;default:true;column:is_active" json:"isActive"`
}

// TableName overrides the default table name.
func (Package) TableName() string { return "subscription_packages" }

// CreateInput carries data for creating a new package.
type CreateInput struct {
	Name                   string
	Description            *string
	Price                  types.Money
	DiscountPercentage     *float64
	Order                  int
	SubscriptionPoints     *int
	SubscriptionPointPrice *types.Money
	CoursesLimit           *int
	CourseLimitInGB        *int
	AssistantsLimit        *int
	WatchLimit             *int
	WatchInterval          *int
	Active                 *bool
}

// UpdateInput captures mutable package fields.
type UpdateInput struct {
	Name                   *string
	Description            *string
	DescriptionProvided    bool
	Price                  *types.Money
	DiscountPercentage     *float64
	Order                  *int
	SubscriptionPoints     *int
	SubscriptionPointPrice *types.Money
	CoursesLimit           *int
	CourseLimitInGB        *int
	AssistantsLimit        *int
	WatchLimit             *int
	WatchInterval          *int
	Active                 *bool
}

// List queries all packages, optionally filtering by active status.
func List(db *gorm.DB, activeOnly bool) ([]Package, error) {
	query := db.Model(&Package{})
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	var packages []Package
	if err := query.Order("\"order\" ASC, created_at DESC").Find(&packages).Error; err != nil {
		return nil, err
	}

	return packages, nil
}

// Get retrieves a package by ID.
func Get(db *gorm.DB, id uuid.UUID) (Package, error) {
	var pkg Package
	if err := db.First(&pkg, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg, ErrPackageNotFound
		}
		return pkg, err
	}
	return pkg, nil
}

// Create inserts a new package.
func Create(db *gorm.DB, input CreateInput) (Package, error) {
	pkg := Package{
		Name:                   strings.TrimSpace(input.Name),
		Description:            trimStringPtr(input.Description),
		Price:                  input.Price,
		DiscountPercentage:     0,
		Order:                  input.Order,
		SubscriptionPoints:     input.SubscriptionPoints,
		SubscriptionPointPrice: input.SubscriptionPointPrice,
		CoursesLimit:           input.CoursesLimit,
		CourseLimitInGB:        input.CourseLimitInGB,
		AssistantsLimit:        input.AssistantsLimit,
		WatchLimit:             input.WatchLimit,
		WatchInterval:          input.WatchInterval,
		Active:                 true,
	}

	if input.DiscountPercentage != nil {
		pkg.DiscountPercentage = *input.DiscountPercentage
	}
	if input.Active != nil {
		pkg.Active = *input.Active
	}

	if err := db.Create(&pkg).Error; err != nil {
		if strings.Contains(err.Error(), "subscription_packages_name_key") {
			return pkg, ErrPackageNameTaken
		}
		if strings.Contains(err.Error(), "subscription_packages_order_key") {
			return pkg, ErrPackageOrderTaken
		}
		return pkg, err
	}

	return pkg, nil
}

// Update modifies an existing package.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (Package, error) {
	pkg, err := Get(db, id)
	if err != nil {
		return pkg, err
	}

	updates := map[string]interface{}{}

	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			return pkg, errors.New("name cannot be empty")
		}
		updates["name"] = trimmed
	}

	if input.DescriptionProvided {
		if input.Description == nil {
			updates["description"] = nil
		} else {
			trimmed := strings.TrimSpace(*input.Description)
			updates["description"] = trimmed
		}
	}

	if input.Price != nil {
		updates["price"] = *input.Price
	}
	if input.DiscountPercentage != nil {
		updates["discount_percentage"] = *input.DiscountPercentage
	}
	if input.Order != nil {
		updates["order"] = *input.Order
	}
	if input.SubscriptionPoints != nil {
		updates["subscription_points"] = *input.SubscriptionPoints
	}
	if input.SubscriptionPointPrice != nil {
		updates["subscription_point_price"] = *input.SubscriptionPointPrice
	}
	if input.CoursesLimit != nil {
		updates["courses_limit"] = *input.CoursesLimit
	}
	if input.CourseLimitInGB != nil {
		updates["course_limit_in_gb"] = *input.CourseLimitInGB
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
	if input.Active != nil {
		updates["is_active"] = *input.Active
	}

	if len(updates) > 0 {
		if err := db.Model(&Package{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			if strings.Contains(err.Error(), "subscription_packages_name_key") {
				return pkg, ErrPackageNameTaken
			}
			if strings.Contains(err.Error(), "subscription_packages_order_key") {
				return pkg, ErrPackageOrderTaken
			}
			return pkg, err
		}
	}

	return Get(db, id)
}

// Delete removes a package.
func Delete(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&Package{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPackageNotFound
	}
	return nil
}

// Helper functions

func trimStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
