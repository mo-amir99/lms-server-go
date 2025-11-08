package referral

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Referral represents an affiliate/referral link.
type Referral struct {
	types.BaseModel

	ReferrerID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"referrerId"`
	ReferredUserID *uuid.UUID `gorm:"type:uuid" json:"referredUserId,omitempty"`
	ExpiresAt      time.Time  `gorm:"not null" json:"expiresAt"`

	// Associations - using inline structs to avoid circular dependencies
	Referrer *struct {
		ID       uuid.UUID      `json:"id"`
		FullName string         `json:"fullName" gorm:"column:full_name"`
		Email    string         `json:"email"`
		UserType types.UserType `json:"userType" gorm:"column:user_type"`
	} `gorm:"foreignKey:ReferrerID;references:ID;-:migration" json:"referrer,omitempty"`

	ReferredUser *struct {
		ID             uuid.UUID  `json:"id"`
		FullName       string     `json:"fullName" gorm:"column:full_name"`
		Email          string     `json:"email"`
		SubscriptionID *uuid.UUID `json:"subscriptionId" gorm:"column:subscription_id"`
	} `gorm:"foreignKey:ReferredUserID;references:ID;-:migration" json:"referredUser,omitempty"`
}

// TableName overrides the default table name.
func (Referral) TableName() string {
	return "referrals"
}

// GetAll retrieves all referrals with user info, optionally filtered by referrer.
func GetAll(db *gorm.DB, referrerID *uuid.UUID) ([]Referral, error) {
	var referrals []Referral
	query := db.Model(&Referral{})

	if referrerID != nil {
		query = query.Where("referrer_id = ?", *referrerID)
	}

	if err := query.
		Preload("Referrer", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, full_name, email, user_type")
		}).
		Preload("ReferredUser", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, full_name, email, subscription_id")
		}).
		Find(&referrals).Error; err != nil {
		return nil, err
	}

	return referrals, nil
}

// Get retrieves a single referral by ID with user info.
func Get(db *gorm.DB, id uuid.UUID) (*Referral, error) {
	var referral Referral
	if err := db.
		Preload("Referrer", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, full_name, email, user_type")
		}).
		Preload("ReferredUser", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, full_name, email, subscription_id")
		}).
		First(&referral, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrReferralNotFound
		}
		return nil, err
	}
	return &referral, nil
}

// CreateInput defines the payload to create a referral.
type CreateInput struct {
	ReferrerID     uuid.UUID
	ReferredUserID *uuid.UUID
	ExpiresAt      *time.Time
}

// Create inserts a new referral after checking for duplicates.
func Create(db *gorm.DB, input CreateInput) (*Referral, error) {
	// Check for duplicate referral if referredUserID is provided
	if input.ReferredUserID != nil {
		var existing Referral
		err := db.Where("referrer_id = ? AND referred_user_id = ?", input.ReferrerID, *input.ReferredUserID).
			First(&existing).Error
		if err == nil {
			return nil, ErrReferralExists
		}
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}

	expiresAt := time.Now().AddDate(1, 0, 0) // Default: 1 year from now
	if input.ExpiresAt != nil {
		expiresAt = *input.ExpiresAt
	}

	referral := Referral{
		ReferrerID:     input.ReferrerID,
		ReferredUserID: input.ReferredUserID,
		ExpiresAt:      expiresAt,
	}

	if err := db.Create(&referral).Error; err != nil {
		return nil, err
	}

	// Reload with user info
	return Get(db, referral.ID)
}

// UpdateInput defines the payload to update a referral.
type UpdateInput struct {
	ReferredUserIDProvided bool
	ReferredUserID         *uuid.UUID
	ExpiresAt              *time.Time
}

// Update modifies an existing referral and returns it with user info.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (*Referral, error) {
	var referral Referral
	if err := db.First(&referral, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrReferralNotFound
		}
		return nil, err
	}

	updates := map[string]interface{}{}

	if input.ReferredUserIDProvided {
		updates["referred_user_id"] = input.ReferredUserID
	}

	if input.ExpiresAt != nil {
		updates["expires_at"] = *input.ExpiresAt
	}

	if len(updates) > 0 {
		if err := db.Model(&referral).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	// Reload with user info
	return Get(db, id)
}

// Delete removes a referral.
func Delete(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&Referral{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrReferralNotFound
	}

	return nil
}
