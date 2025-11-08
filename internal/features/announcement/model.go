package announcement

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Announcement represents a subscription announcement.
type Announcement struct {
	types.BaseModel

	SubscriptionID uuid.UUID `gorm:"type:uuid;not null;column:subscription_id;index;index:idx_subscription_active,priority:1" json:"subscriptionId"`
	Title          string    `gorm:"type:varchar(255);not null" json:"title"`
	Content        *string   `gorm:"type:text" json:"content,omitempty"`
	ImageURL       *string   `gorm:"type:text;column:image_url" json:"imageUrl,omitempty"`
	OnClick        *string   `gorm:"type:varchar(255);column:on_click" json:"onClick,omitempty"`
	Public         bool      `gorm:"type:boolean;not null;default:true;column:is_public" json:"isPublic"`
	Active         bool      `gorm:"type:boolean;not null;default:true;column:is_active;index;index:idx_subscription_active,priority:2" json:"isActive"`
}

// TableName overrides the default table name.
func (Announcement) TableName() string { return "announcements" }

// ListFilters defines announcement query filters.
type ListFilters struct {
	SubscriptionID uuid.UUID
	ActiveOnly     bool
	PublicOnly     bool
	UserID         *uuid.UUID // For filtering by group access
}

// CreateInput carries data for creating a new announcement.
type CreateInput struct {
	SubscriptionID uuid.UUID
	Title          string
	Content        *string
	ImageURL       *string
	OnClick        *string
	Public         *bool
	Active         *bool
}

// UpdateInput captures mutable announcement fields.
type UpdateInput struct {
	Title           *string
	Content         *string
	ContentProvided bool
	ImageURL        *string
	ImageProvided   bool
	OnClick         *string
	OnClickProvided bool
	Public          *bool
	Active          *bool
}

// List retrieves paginated announcements with filters.
func List(db *gorm.DB, filters ListFilters, params pagination.Params) ([]Announcement, int64, error) {
	query := db.Model(&Announcement{}).Where("subscription_id = ?", filters.SubscriptionID)

	if filters.ActiveOnly {
		query = query.Where("is_active = ?", true)
	}

	if filters.PublicOnly {
		query = query.Where("is_public = ?", true)
	}

	// For students, filter by public announcements or group access
	if filters.UserID != nil {
		// Get announcement IDs from group access for this user
		var groupAnnouncementIDs []string
		err := db.Table("group_access").
			Where("subscription_id = ? AND ? = ANY(users)", filters.SubscriptionID, filters.UserID.String()).
			Select("UNNEST(announcements) as announcement_id").
			Pluck("announcement_id", &groupAnnouncementIDs).Error

		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, 0, err
		}

		// Show public announcements OR announcements user has access to via groups
		if len(groupAnnouncementIDs) > 0 {
			query = query.Where("is_public = ? OR id IN ?", true, groupAnnouncementIDs)
		} else {
			query = query.Where("is_public = ?", true)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var announcements []Announcement
	err := query.
		Order("created_at DESC").
		Offset(params.Skip).
		Limit(params.Limit).
		Find(&announcements).Error

	return announcements, total, err
}

// Get retrieves an announcement by ID.
func Get(db *gorm.DB, id uuid.UUID) (Announcement, error) {
	var announcement Announcement
	if err := db.First(&announcement, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return announcement, ErrAnnouncementNotFound
		}
		return announcement, err
	}
	return announcement, nil
}

// Create inserts a new announcement.
func Create(db *gorm.DB, input CreateInput) (Announcement, error) {
	if input.Title == "" {
		return Announcement{}, ErrTitleRequired
	}

	public := true
	if input.Public != nil {
		public = *input.Public
	}

	active := true
	if input.Active != nil {
		active = *input.Active
	}

	announcement := Announcement{
		SubscriptionID: input.SubscriptionID,
		Title:          input.Title,
		Content:        input.Content,
		ImageURL:       input.ImageURL,
		OnClick:        input.OnClick,
		Public:         public,
		Active:         active,
	}

	if err := db.Create(&announcement).Error; err != nil {
		return Announcement{}, err
	}

	return announcement, nil
}

// Update modifies an existing announcement.
func Update(db *gorm.DB, id uuid.UUID, input UpdateInput) (Announcement, error) {
	announcement, err := Get(db, id)
	if err != nil {
		return announcement, err
	}

	if input.Title != nil {
		if *input.Title == "" {
			return announcement, ErrTitleRequired
		}
		announcement.Title = *input.Title
	}

	if input.ContentProvided {
		announcement.Content = input.Content
	}

	if input.ImageProvided {
		announcement.ImageURL = input.ImageURL
	}

	if input.OnClickProvided {
		announcement.OnClick = input.OnClick
	}

	if input.Public != nil {
		announcement.Public = *input.Public
	}

	if input.Active != nil {
		announcement.Active = *input.Active
	}

	if err := db.Save(&announcement).Error; err != nil {
		return announcement, err
	}

	return announcement, nil
}

// Delete removes an announcement.
func Delete(db *gorm.DB, id uuid.UUID) error {
	result := db.Delete(&Announcement{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrAnnouncementNotFound
	}
	return nil
}
