package user

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// User represents a system user.
type User struct {
	types.BaseModel

	SubscriptionID *uuid.UUID     `gorm:"type:uuid;column:subscription_id;index:idx_usertype_subscription,priority:2;index:idx_subscription_active,priority:1" json:"subscriptionId,omitempty"`
	FullName       string         `gorm:"type:varchar(30);not null;column:full_name" json:"fullName"`
	Email          string         `gorm:"type:varchar(255);not null;uniqueIndex" json:"email"`
	Phone          *string        `gorm:"type:varchar(20)" json:"phone,omitempty"`
	Password       string         `gorm:"type:varchar(255);not null" json:"-"`
	UserType       types.UserType `gorm:"type:varchar(20);not null;default:'student';column:user_type;index;index:idx_usertype_subscription,priority:1;index:idx_usertype_active,priority:1" json:"userType"`
	RefreshToken   *string        `gorm:"type:text;column:refresh_token" json:"-"`
	DeviceID       *string        `gorm:"type:varchar(255);column:device_id" json:"-"`
	Active         bool           `gorm:"type:boolean;not null;default:true;column:is_active;index;index:idx_usertype_active,priority:2;index:idx_subscription_active,priority:2" json:"isActive"`
	EmailVerified  bool           `gorm:"type:boolean;not null;default:false;column:email_verified" json:"emailVerified"`

	// Relations
	Subscription *subscription.Subscription `gorm:"foreignKey:SubscriptionID" json:"subscription,omitempty"`
}

// TableName overrides the default table name.
func (User) TableName() string { return "users" }

// ListFilters defines user query filters.
type ListFilters struct {
	Keyword         string
	SubscriptionID  *uuid.UUID
	UserType        []string
	UserTypes       []types.UserType
	ExcludeID       *uuid.UUID
	ExcludeUserType types.UserType
}

// CreateInput carries data for creating a new user.
type CreateInput struct {
	SubscriptionID *uuid.UUID
	FullName       string
	Email          string
	Phone          *string
	Password       string
	UserType       types.UserType
	Active         *bool
}

// UpdateInput captures mutable user fields.
type UpdateInput struct {
	SubscriptionID         *uuid.UUID
	SubscriptionIDProvided bool
	FullName               *string
	Email                  *string
	Phone                  *string
	PhoneProvided          bool
	Password               *string
	UserType               *types.UserType
	Active                 *bool
}

// List queries users with filters and pagination.
func List(db *gorm.DB, filters ListFilters, params pagination.Params) ([]User, int64, error) {
	query := db.Model(&User{})

	if filters.Keyword != "" {
		keyword := "%" + strings.ToLower(filters.Keyword) + "%"
		query = query.Where("LOWER(full_name) LIKE ? OR LOWER(email) LIKE ? OR phone LIKE ?",
			keyword, keyword, keyword)
	}

	if filters.SubscriptionID != nil {
		query = query.Where("subscription_id = ?", *filters.SubscriptionID)
	}

	if len(filters.UserType) > 0 {
		query = query.Where("user_type IN ?", filters.UserType)
	}

	if len(filters.UserTypes) > 0 {
		query = query.Where("user_type IN ?", filters.UserTypes)
	}

	if filters.ExcludeID != nil {
		query = query.Where("id != ?", *filters.ExcludeID)
	}

	if filters.ExcludeUserType != "" {
		query = query.Where("user_type != ?", filters.ExcludeUserType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []User
	if err := query.Order("created_at DESC").Offset(params.Skip).Limit(params.Limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// Get retrieves a user by ID.
func Get(db *gorm.DB, id uuid.UUID) (User, error) {
	var user User
	if err := db.First(&user, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return user, ErrUserNotFound
		}
		return user, err
	}
	return user, nil
}

// GetByEmail retrieves a user by email with subscription preloaded.
func GetByEmail(db *gorm.DB, email string) (User, error) {
	var user User
	if err := db.Preload("Subscription").First(&user, "LOWER(email) = ?", strings.ToLower(strings.TrimSpace(email))).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return user, ErrUserNotFound
		}
		return user, err
	}
	return user, nil
}

// Create inserts a new user with hashed password.
func Create(db *gorm.DB, input CreateInput) (User, error) {
	if len(input.Password) < 8 {
		return User{}, ErrInvalidPassword
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), 10)
	if err != nil {
		return User{}, err
	}

	user := User{
		SubscriptionID: input.SubscriptionID,
		FullName:       strings.TrimSpace(input.FullName),
		Email:          strings.ToLower(strings.TrimSpace(input.Email)),
		Phone:          trimStringPtr(input.Phone),
		Password:       string(hashedPassword),
		UserType:       input.UserType,
		Active:         true,
	}

	if input.Active != nil {
		user.Active = *input.Active
	}

	if err := db.Create(&user).Error; err != nil {
		if strings.Contains(err.Error(), "users_email_key") {
			return user, ErrEmailTaken
		}
		return user, err
	}

	return user, nil
}

// Update modifies an existing user.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (User, error) {
	user, err := Get(db, id)
	if err != nil {
		return user, err
	}

	updates := map[string]interface{}{}

	if input.SubscriptionIDProvided {
		updates["subscription_id"] = input.SubscriptionID
	}

	if input.FullName != nil {
		trimmed := strings.TrimSpace(*input.FullName)
		if trimmed == "" {
			return user, errors.New("fullName cannot be empty")
		}
		updates["full_name"] = trimmed
	}

	if input.Email != nil {
		trimmed := strings.ToLower(strings.TrimSpace(*input.Email))
		if trimmed == "" {
			return user, errors.New("email cannot be empty")
		}
		updates["email"] = trimmed
	}

	if input.PhoneProvided {
		if input.Phone == nil {
			updates["phone"] = nil
		} else {
			trimmed := strings.TrimSpace(*input.Phone)
			updates["phone"] = trimmed
		}
	}

	if input.Password != nil {
		if len(*input.Password) < 8 {
			return user, ErrInvalidPassword
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*input.Password), 10)
		if err != nil {
			return user, err
		}
		updates["password"] = string(hashedPassword)
	}

	if input.UserType != nil {
		updates["user_type"] = *input.UserType
	}

	if input.Active != nil {
		updates["is_active"] = *input.Active
	}

	if len(updates) > 0 {
		if err := db.Model(&User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			if strings.Contains(err.Error(), "users_email_key") {
				return user, ErrEmailTaken
			}
			return user, err
		}
	}

	return Get(db, id)
}

// Delete removes a user.
func Delete(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&User{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// ComparePassword checks if the provided password matches the user's hashed password.
func (u *User) ComparePassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
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

// UserTypeIndex returns the position of a userType in the hierarchy.
func UserTypeIndex(userType types.UserType) int {
	for i, t := range UserTypeOrder {
		if t == userType {
			return i
		}
	}
	return -1
}

// CanManageUserType checks if requester can manage a target user type.
func CanManageUserType(requesterType, targetType types.UserType) bool {
	requesterIdx := UserTypeIndex(requesterType)
	targetIdx := UserTypeIndex(targetType)

	if requesterIdx == -1 || targetIdx == -1 {
		return false
	}

	// Can only manage users with lower user type
	return targetIdx < requesterIdx
}
